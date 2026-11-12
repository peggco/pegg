package middlewares

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/peggco/pegg/internal/agent"
	agentmw "github.com/peggco/pegg/internal/agent/middleware"
	"github.com/peggco/pegg/internal/llm"
)

var ErrRepetitionLoop = errors.New("repetition loop detected")

type responseRecord struct {
	content  string
	toolSigs []string
}

type loopState struct {
	recentTexts     []string
	recentReasoning []string
	recentResponses []responseRecord
	loopDetected    bool
	loopReason      string
}

type loopDetector struct {
	agentmw.BaseMiddleware

	mu        sync.Mutex
	states    map[string]*loopState
	content   repetitionConfig
	reasoning repetitionConfig
}

const (
	loopWindow      = 6
	repeatThreshold = 3
	maxCompareLen   = 300
)

func NewLoopDetector() *loopDetector {
	return &loopDetector{
		states: make(map[string]*loopState),
		reasoning: repetitionConfig{
			MinUnitChars: defaultLoopMinUnitChars,
			MaxUnitChars: defaultLoopMaxUnitChars,
			MinRepeats:   defaultLoopRepeatThreshold,
			WindowChars:  defaultLoopWindowChars,
		},
		content: repetitionConfig{
			MinUnitChars: defaultLoopMinUnitChars,
			MaxUnitChars: defaultLoopMaxUnitChars,
			MinRepeats:   defaultLoopRepeatThreshold + contentRepeatExtra,
			WindowChars:  defaultLoopWindowChars,
		},
	}
}

func loopKey(ctx context.Context) string {
	if a := agent.FromContext(ctx); a != nil && a.ID != "" {
		return a.ID
	}
	return ""
}

func (l *loopDetector) stateFor(key string) *loopState {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.states[key]
	if st == nil {
		st = &loopState{}
		l.states[key] = st
	}
	return st
}

func (l *loopDetector) forget(key string) {
	l.mu.Lock()
	delete(l.states, key)
	l.mu.Unlock()
}

func (l *loopDetector) BeforeRun(ctx context.Context, _, _ string) error {
	key := loopKey(ctx)
	l.mu.Lock()
	l.states[key] = &loopState{}
	l.mu.Unlock()
	return nil
}

func (l *loopDetector) AfterRun(ctx context.Context, _ string, _ *agentmw.Result, _ error) error {
	l.forget(loopKey(ctx))
	return nil
}

func (l *loopDetector) BeforeLLM(ctx context.Context, req *llm.Request) error {
	st := l.stateFor(loopKey(ctx))
	if !st.loopDetected {
		return nil
	}

	msg := fmt.Sprintf("An execution loop was detected: %s. Attempt an alternative reasoning path, change your approach, or use different tool arguments. Do not repeat the same pattern.", st.loopReason)
	req.Messages = append(req.Messages, llm.SystemMessage(msg))

	st.loopDetected = false
	st.loopReason = ""
	return nil
}

func (l *loopDetector) InvokeLLMStream(ctx context.Context, req *llm.Request, handler llm.StreamHandler, next agentmw.LLMStreamInvoker) error {
	content := newRepetitionDetector("content", l.content)
	reasoning := newRepetitionDetector("thinking", l.reasoning)

	wrapped := func(chunk llm.StreamChunk) error {
		if chunk.Reasoning != "" {
			if hit, ok := reasoning.Append(chunk.Reasoning); ok {
				return repetitionError(hit)
			}
		}
		if chunk.Content != "" {
			if hit, ok := content.Append(chunk.Content); ok {
				return repetitionError(hit)
			}
		}
		return handler(chunk)
	}
	return next(ctx, req, wrapped)
}

func (l *loopDetector) AfterLLM(ctx context.Context, _ *llm.Request, resp *llm.Response) error {
	st := l.stateFor(loopKey(ctx))

	content := normalizeText(resp.GetContent())
	reasoning := normalizeText(resp.GetReasoning())
	toolCalls := resp.GetToolCalls()

	if content != "" {
		st.recentTexts = append(st.recentTexts, content)
		if len(st.recentTexts) > loopWindow {
			st.recentTexts = st.recentTexts[1:]
		}
	}

	if reasoning != "" {
		st.recentReasoning = append(st.recentReasoning, reasoning)
		if len(st.recentReasoning) > loopWindow {
			st.recentReasoning = st.recentReasoning[1:]
		}
	}

	var sigs []string
	for _, tc := range toolCalls {
		sigs = append(sigs, toolSig(tc))
	}
	rec := responseRecord{content: content, toolSigs: sigs}
	st.recentResponses = append(st.recentResponses, rec)
	if len(st.recentResponses) > loopWindow {
		st.recentResponses = st.recentResponses[1:]
	}

	if hit, ok := detectRepeatedText("thinking", resp.GetReasoning(), l.reasoning); ok {
		return repetitionError(hit)
	}
	if hit, ok := detectRepeatedText("content", resp.GetContent(), l.content); ok {
		return repetitionError(hit)
	}

	if len(sigs) > 0 {
		if reason := detectResponseLoop(st.recentResponses); reason != "" {
			st.loopDetected = true
			st.loopReason = reason
			return nil
		}
	}
	if reason := detectLoop(st.recentTexts, "text"); reason != "" {
		st.loopDetected = true
		st.loopReason = reason
		return nil
	}
	if reason := detectLoop(st.recentReasoning, "thinking"); reason != "" {
		st.loopDetected = true
		st.loopReason = reason
		return nil
	}

	return nil
}

func repetitionError(hit repetitionHit) error {
	return fmt.Errorf("%w: %s", ErrRepetitionLoop, formatRepetitionHit(hit))
}

func detectLoop(window []string, label string) string {
	if len(window) < repeatThreshold {
		return ""
	}
	last := window[len(window)-1]
	count := 0
	for i := len(window) - 1; i >= 0; i-- {
		if window[i] == last {
			count++
		} else {
			break
		}
	}
	if count >= repeatThreshold {
		return fmt.Sprintf("repetitive %s output detected: same content produced %d consecutive times", label, count)
	}
	return ""
}

func detectResponseLoop(responses []responseRecord) string {
	if len(responses) < 3 {
		return ""
	}

	last := responses[len(responses)-1]
	count := 0
	for i := len(responses) - 1; i >= 0; i-- {
		if responsesEqual(responses[i], last) {
			count++
		} else {
			break
		}
	}
	if count >= repeatThreshold {
		label := "response"
		if len(last.toolSigs) > 0 {
			label = "text-tool"
		}
		return fmt.Sprintf("repetitive %s pattern detected: same content+tools produced %d consecutive times", label, count)
	}

	return ""
}

func responsesEqual(a, b responseRecord) bool {
	if a.content != b.content {
		return false
	}
	if len(a.toolSigs) != len(b.toolSigs) {
		return false
	}
	for i := range a.toolSigs {
		if a.toolSigs[i] != b.toolSigs[i] {
			return false
		}
	}
	return true
}

func toolSig(tc llm.ToolCall) string {
	args := strings.TrimSpace(tc.Function.Arguments)
	if len(args) > maxCompareLen {
		args = args[:maxCompareLen]
	}
	args = strings.Join(strings.Fields(args), " ")
	return tc.Function.Name + "(" + args + ")"
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > maxCompareLen {
		s = s[:maxCompareLen]
	}
	return s
}

func (l *loopDetector) OnError(ctx context.Context, _ error) error {
	l.forget(loopKey(ctx))
	return nil
}
