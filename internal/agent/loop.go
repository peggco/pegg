package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/agent/reminder"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
)

type runState struct {
	agent              *Agent
	history            []llm.Message
	iterations         int
	usage              llm.Usage
	output             string
	finishReason       llm.FinishReason
	modelID            string
	provider           string
	stream             StreamHandler
	pendingAttachments []llm.Attachment
	continuation       string
	emptyResponses     int
}

const (
	maxEmptyResponseRetries = 3
	maxParallelToolCalls    = 8
)

func (a *Agent) run(ctx context.Context, input string, stream StreamHandler) (*RunResult, error) {
	if strings.TrimSpace(input) == "" && len(a.Attachments) == 0 {
		return nil, a.fail(ctx, ErrEmptyInput)
	}
	mi := expandInput(input)
	input = mi.Text
	messageText := a.messageWithAttachments(input)
	if err := a.resolveToolNames(); err != nil {
		return nil, a.fail(ctx, err)
	}
	if err := a.resolveMiddlewareNames(); err != nil {
		return nil, a.fail(ctx, err)
	}
	ctx = a.withSession(ctx)
	resolveModel(ctx, a, input)
	if a.Provider == nil {
		return nil, a.fail(ctx, ErrNoProvider)
	}

	ctx = WithAgent(ctx, a)
	state := &runState{agent: a, stream: stream}
	ctx = WithHistory(ctx, &state.history)
	ctx = WithStream(ctx, stream)
	a.debugf("agent %q started run", a.Name)
	if err := a.chain.BeforeRun(ctx, a.Name, input); err != nil {
		return nil, a.fail(ctx, err)
	}
	a.publish(TopicAgentStarted, AgentStarted{
		AgentID:         a.ID,
		AgentName:       a.Name,
		DisplayName:     a.DisplayName,
		ParentAgentID:   a.ParentAgentID,
		Model:           a.Model,
		Provider:        a.Provider,
		ReasoningEffort: a.ReasoningEffort,
		Agent:           a,
	})
	a.publish(TopicAgentInput, AgentInput{
		AgentID:     a.ID,
		AgentName:   a.Name,
		Model:       a.Model,
		Input:       messageText,
		Attachments: a.Attachments,
	})

	ctx = a.withSession(ctx)
	prov := a.Provider
	state.modelID = a.Model.ID
	state.provider = prov.Name()

	if a.SystemPrompt != "" {
		state.history = append(state.history, llm.SystemMessage(a.SystemPrompt))
	}
	state.history = appendSystemReminders(state.history, mi.SystemReminders)
	if strings.TrimSpace(messageText) != "" {
		state.history = append(state.history, a.userMessage(messageText))
	}
	if len(mi.Calls) > 0 {
		ctx = a.withSession(ctx)
		a.resolveInputCalls(ctx, state, mi.Calls)
	}

	return a.loop(ctx, state, prov)
}

func (a *Agent) resume(ctx context.Context, input string, history []llm.Message, stream StreamHandler) (*RunResult, error) {
	if strings.TrimSpace(input) == "" && len(a.Attachments) == 0 {
		return nil, a.fail(ctx, ErrEmptyInput)
	}
	mi := expandInput(input)
	input = mi.Text
	messageText := a.messageWithAttachments(input)
	if err := a.resolveToolNames(); err != nil {
		return nil, a.fail(ctx, err)
	}
	if err := a.resolveMiddlewareNames(); err != nil {
		return nil, a.fail(ctx, err)
	}
	ctx = a.withSession(ctx)
	resolveModel(ctx, a, input)
	if a.Provider == nil {
		return nil, a.fail(ctx, ErrNoProvider)
	}

	ctx = WithAgent(ctx, a)
	state := &runState{agent: a, stream: stream}
	ctx = WithHistory(ctx, &state.history)
	ctx = WithStream(ctx, stream)
	a.debugf("agent %q resumed run", a.Name)
	if err := a.chain.BeforeRun(ctx, a.Name, input); err != nil {
		return nil, a.fail(ctx, err)
	}
	a.publish(TopicAgentStarted, AgentStarted{
		AgentID:         a.ID,
		AgentName:       a.Name,
		DisplayName:     a.DisplayName,
		ParentAgentID:   a.ParentAgentID,
		Model:           a.Model,
		Provider:        a.Provider,
		ReasoningEffort: a.ReasoningEffort,
		Agent:           a,
	})
	a.publish(TopicAgentInput, AgentInput{
		AgentID:     a.ID,
		AgentName:   a.Name,
		Model:       a.Model,
		Input:       messageText,
		Attachments: a.Attachments,
	})

	ctx = a.withSession(ctx)
	prov := a.Provider
	state.modelID = a.Model.ID
	state.provider = prov.Name()

	state.history = append(state.history, history...)
	state.history = appendSystemReminders(state.history, mi.SystemReminders)
	if strings.TrimSpace(messageText) != "" {
		state.history = append(state.history, a.userMessage(messageText))
	}
	if len(mi.Calls) > 0 {
		ctx = a.withSession(ctx)
		a.resolveInputCalls(ctx, state, mi.Calls)
	}

	return a.loop(ctx, state, prov)
}

func appendSystemReminders(history []llm.Message, reminders []string) []llm.Message {
	for _, r := range reminders {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		history = append(history, llm.SystemMessage("<system-reminder>\n"+r+"\n</system-reminder>"))
	}
	return history
}

func (a *Agent) loop(ctx context.Context, state *runState, prov llm.Provider) (*RunResult, error) {
	finished := false
	for a.MaxIterations <= 0 || state.iterations < a.MaxIterations {
		ctx = a.withSession(ctx)
		select {
		case <-ctx.Done():
			return a.partialResult(state), a.fail(ctx, ctx.Err())
		default:
		}
		state.iterations++
		msg, calls, err := a.iterate(ctx, state, prov)
		if err != nil {
			if msg.Role == llm.RoleAssistant && !emptyAssistantMessage(msg) {
				state.history = append(state.history, msg)
			}
			return a.partialResult(state), a.fail(ctx, err)
		}
		if !emptyAssistantMessage(msg) {
			state.history = append(state.history, msg)
			a.publish(TopicAgentMessage, AgentMessage{
				AgentID:   a.ID,
				AgentName: a.Name,
				Model:     a.Model,
				Message:   msg,
			})
		}
		if len(calls) > 0 {
			ctx = a.withSession(ctx)
			a.executeTools(ctx, state, calls)
			a.attachToolAttachments(state)
			state.emptyResponses = 0
			continue
		}
		if a.HasPendingNotifications() {
			continue
		}
		if emptyAssistantMessage(msg) {
			if ctx.Err() != nil {
				return a.partialResult(state), a.fail(ctx, ctx.Err())
			}
			canRetry := a.MaxIterations <= 0 || state.iterations < a.MaxIterations
			if !canRetry {
				finished = true
				break
			}
			if state.emptyResponses >= maxEmptyResponseRetries {
				err := fmt.Errorf("%w after %d attempts", ErrEmptyResponse, state.emptyResponses)
				return a.partialResult(state), a.fail(ctx, err)
			}
			state.emptyResponses++
			state.continuation = "Your previous turn ended without producing any response content or tool calls. Continue the task and provide the response now."
			continue
		}
		state.emptyResponses = 0
		finished = true
		break
	}

	result := &RunResult{
		AgentName:    a.Name,
		Model:        a.Model,
		Provider:     state.provider,
		Output:       state.output,
		History:      state.history,
		Usage:        state.usage,
		Iterations:   state.iterations,
		FinishReason: state.finishReason,
	}

	if !finished && a.MaxIterations > 0 {
		err := fmt.Errorf("%w (%d iterations)", ErrMaxIterations, state.iterations)
		a.debugf("agent %q hit max iterations", a.Name)
		a.publish(TopicAgentError, AgentError{AgentID: a.ID, AgentName: a.Name, Model: a.Model, Err: err})
		_ = a.chain.OnError(ctx, err)
		return a.partialResult(state), err
	}

	if err := a.chain.AfterRun(ctx, a.Name, result, nil); err != nil {
		return nil, a.fail(ctx, err)
	}
	a.publish(TopicAgentFinished, AgentFinished{
		AgentID:      a.ID,
		AgentName:    a.Name,
		Model:        a.Model,
		Output:       result.Output,
		Usage:        result.Usage,
		Iterations:   result.Iterations,
		FinishReason: result.FinishReason,
	})
	a.debugf("agent %q finished run in %d iterations", a.Name, state.iterations)
	return result, nil
}

func (a *Agent) messageWithAttachments(input string) string {
	block := attach.Block(a.Attachments)
	if block == "" {
		return input
	}
	if strings.TrimSpace(input) == "" {
		return block
	}
	return input + "\n\n" + block
}

func (a *Agent) userMessage(input string) llm.Message {
	atts := attach.MediaAttachments(a.Attachments, a.Model)
	if len(atts) == 0 {
		return llm.UserMessage(input)
	}
	return llm.UserMessage(llm.ContentWithAttachments(input, atts))
}

func (a *Agent) resolveInputCalls(ctx context.Context, state *runState, calls []llm.ToolCall) {
	msg := llm.AssistantMessage("")
	msg.ToolCalls = calls
	state.history = append(state.history, msg)
	ctx = a.withSession(ctx)
	a.executeTools(ctx, state, calls)
	a.attachToolAttachments(state)
}

func (a *Agent) attachToolAttachments(state *runState) {
	if len(state.pendingAttachments) == 0 {
		return
	}
	atts := attach.MediaAttachments(state.pendingAttachments, a.Model)
	state.pendingAttachments = nil
	if len(atts) == 0 {
		return
	}
	names := make([]string, 0, len(atts))
	for _, att := range atts {
		if att.FileName != "" {
			names = append(names, att.FileName)
		}
	}
	text := "Images loaded by the read tool"
	if len(names) > 0 {
		text += ": " + strings.Join(names, ", ")
	}
	state.history = append(state.history, llm.UserMessage(llm.ContentWithAttachments(text, atts)))
}

func emptyAssistantMessage(m llm.Message) bool {
	return m.Role == llm.RoleAssistant && len(m.ToolCalls) == 0 && strings.TrimSpace(llm.MessageText(m)) == ""
}

func appendReminder(msgs []llm.Message, text string) []llm.Message {
	if len(msgs) == 0 {
		return append(msgs, llm.SystemMessage(text))
	}
	last := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == llm.RoleUser || msgs[i].Role == llm.RoleTool {
			last = i
			break
		}
	}
	if last < 0 {
		return append(msgs, llm.SystemMessage(text))
	}
	out := append([]llm.Message(nil), msgs...)
	out[last] = llm.AppendMessageText(out[last], text)
	return out
}

func (a *Agent) partialResult(state *runState) *RunResult {
	return &RunResult{
		AgentName:  a.Name,
		Model:      a.Model,
		Provider:   state.provider,
		History:    state.history,
		Usage:      state.usage,
		Iterations: state.iterations,
	}
}

func (a *Agent) iterate(ctx context.Context, state *runState, prov llm.Provider) (llm.Message, []llm.ToolCall, error) {
	req := a.buildRequest(state)

	if r := a.StandingReminder(); r != nil {
		if formatted := reminder.FormatAll([]reminder.Reminder{*r}); formatted != "" {
			req.Messages = appendReminder(req.Messages, formatted)
		}
	}

	if notifications := a.drainNotifications(); len(notifications) > 0 {
		if formatted := reminder.FormatAll(notifications); formatted != "" {
			req.Messages = append(req.Messages, llm.SystemMessage(formatted))
		}
	}
	if state.continuation != "" {
		req.Messages = append(req.Messages, llm.SystemMessage(state.continuation))
		state.continuation = ""
	}

	if err := a.chain.BeforeLLM(ctx, req); err != nil {
		return llm.Message{}, nil, err
	}
	ctx = a.withSession(ctx)
	if state.stream != nil {
		return a.iterateStream(ctx, state, prov, req)
	}

	resp, err := a.chain.InvokeLLM(ctx, req, prov.Chat)
	if err != nil {
		return llm.Message{}, nil, err
	}
	if err := a.chain.AfterLLM(ctx, req, resp); err != nil {
		return llm.Message{}, nil, err
	}

	state.accumulateUsage(resp.Usage)
	a.publish(TopicAgentUsage, AgentUsage{
		AgentID:   a.ID,
		AgentName: a.Name,
		Model:     a.Model,
		Usage:     state.usage,
	})
	state.output = resp.GetContent()
	state.finishReason = resp.GetFinishReason()

	msg := llm.AssistantMessage(state.output)
	if reasoning := resp.GetReasoning(); reasoning != "" {
		msg.Reasoning = reasoning
	}
	calls := resp.GetToolCalls()
	if len(calls) > 0 {
		msg.ToolCalls = calls
	}
	return msg, calls, nil
}

func (a *Agent) iterateStream(ctx context.Context, state *runState, prov llm.Provider, req *llm.Request) (llm.Message, []llm.ToolCall, error) {
	acc := newStreamAccumulator()
	err := a.chain.InvokeLLMStream(ctx, req, func(chunk llm.StreamChunk) error {
		acc.append(chunk)
		if chunk.Usage != nil {
			state.accumulateUsage(*chunk.Usage)
			a.publish(TopicAgentUsage, AgentUsage{
				AgentID:   a.ID,
				AgentName: a.Name,
				Model:     a.Model,
				Usage:     state.usage,
			})
		}
		if chunk.FinishReason != "" {
			state.finishReason = chunk.FinishReason
		}
		if chunk.Content != "" {
			if err := state.stream(StreamEvent{
				Type:      StreamToken,
				AgentID:   a.ID,
				AgentName: a.Name,
				Content:   chunk.Content,
			}); err != nil {
				return err
			}
			a.publish(TopicAgentToken, AgentToken{
				AgentID:   a.ID,
				AgentName: a.Name,
				Model:     a.Model,
				Content:   chunk.Content,
			})
		}
		if chunk.Reasoning != "" {
			if err := state.stream(StreamEvent{
				Type:      StreamReasoning,
				AgentID:   a.ID,
				AgentName: a.Name,
				Reasoning: chunk.Reasoning,
			}); err != nil {
				return err
			}
			a.publish(TopicAgentToken, AgentToken{
				AgentID:   a.ID,
				AgentName: a.Name,
				Model:     a.Model,
				Reasoning: chunk.Reasoning,
			})
		}
		return nil
	}, prov.ChatStream)
	if err != nil {
		if acc.content == "" && len(acc.order) == 0 && acc.reasoning == "" {
			return llm.Message{}, nil, err
		}
		msg := llm.AssistantMessage(acc.content)
		if acc.reasoning != "" {
			msg.Reasoning = acc.reasoning
		}
		calls := acc.toolCalls()
		if len(calls) > 0 {
			msg.ToolCalls = calls
		}
		return msg, calls, err
	}

	state.output = acc.content
	msg := llm.AssistantMessage(acc.content)
	if acc.reasoning != "" {
		msg.Reasoning = acc.reasoning
	}
	calls := acc.toolCalls()
	if len(calls) > 0 {
		msg.ToolCalls = calls
	}

	resp := &llm.Response{
		Model:   req.Model,
		Choices: []llm.Choice{{Index: 0, Message: &msg, FinishReason: &state.finishReason}},
		Usage:   state.usage,
	}
	if err := a.chain.AfterLLM(ctx, req, resp); err != nil {
		return llm.Message{}, nil, err
	}

	for _, call := range calls {
		callCopy := call
		if err := state.stream(StreamEvent{
			Type:      StreamToolCall,
			AgentID:   a.ID,
			AgentName: a.Name,
			ToolCall:  &callCopy,
		}); err != nil {
			return llm.Message{}, nil, err
		}
	}
	if err := state.stream(StreamEvent{
		Type:      StreamDone,
		AgentID:   a.ID,
		AgentName: a.Name,
		Model:     a.Model,
		Content:   acc.content,
		Usage:     &state.usage,
	}); err != nil {
		return llm.Message{}, nil, err
	}
	return msg, calls, nil
}

type toolExecResult struct {
	call        llm.ToolCall
	output      string
	attachments []llm.Attachment
	toolErr     error
}

func (a *Agent) runTool(ctx context.Context, call llm.ToolCall) toolExecResult {
	res := toolExecResult{call: call}

	if err := a.chain.BeforeTool(ctx, call); err != nil {
		res.output = fmt.Sprintf("Error: tool call aborted by middleware: %v", err)
		res.toolErr = err
		return res
	}

	t, ok := a.Tools.Get(call.Function.Name)
	if !ok {
		res.output = fmt.Sprintf("Error: tool %q is not registered", call.Function.Name)
		res.toolErr = fmt.Errorf("%w: %s", tool.ErrNotFound, call.Function.Name)
		_ = a.chain.AfterTool(ctx, call, res.output, res.toolErr)
		return res
	}

	sink := tool.NewAttachmentSink()
	toolCtx := tool.WithAttachmentSink(ctx, sink)
	output, toolErr := a.chain.InvokeTool(toolCtx, call, func(ctx context.Context, call llm.ToolCall) (string, error) {
		return t.Execute(ctx, call.Function.Arguments)
	})
	if toolErr != nil {
		res.output = fmt.Sprintf("Error: %v", toolErr)
		res.toolErr = fmt.Errorf("%w: %s: %v", ErrToolExecutionFailed, t.Name(), toolErr)
	} else {
		res.output = output
		res.attachments = sink.Attachments()
	}

	_ = a.chain.AfterTool(ctx, call, res.output, res.toolErr)
	return res
}

func (a *Agent) executeTools(ctx context.Context, state *runState, calls []llm.ToolCall) {
	if len(calls) == 0 {
		return
	}

	for _, call := range calls {
		a.debugf("agent %q tool call %q (%s)", a.Name, call.Function.Name, call.ID)
		a.publish(TopicAgentToolCall, AgentToolCall{
			AgentID:   a.ID,
			AgentName: a.Name,
			Model:     a.Model,
			Call:      call,
		})
	}

	results := make([]toolExecResult, len(calls))
	if len(calls) == 1 {
		results[0] = a.runTool(ctx, calls[0])
	} else {
		sem := make(chan struct{}, maxParallelToolCalls)
		var wg sync.WaitGroup
		for i, call := range calls {
			i, call := i, call
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				results[i] = a.runTool(ctx, call)
			}()
		}
		wg.Wait()
	}

	for _, res := range results {
		state.history = append(state.history, llm.ToolMessage(res.output, res.call.ID))
		state.pendingAttachments = append(state.pendingAttachments, res.attachments...)
		a.emitToolResult(state, res.call, res.output, res.attachments, res.toolErr)
	}
}

func (a *Agent) emitToolResult(state *runState, call llm.ToolCall, output string, attachments []llm.Attachment, toolErr error) {
	a.publish(TopicAgentToolResult, AgentToolResult{
		AgentID:     a.ID,
		AgentName:   a.Name,
		Model:       a.Model,
		CallID:      call.ID,
		ToolName:    call.Function.Name,
		Output:      output,
		Attachments: attachments,
		Err:         toolErr,
	})
	if state.stream != nil {
		callCopy := call
		_ = state.stream(StreamEvent{
			Type:            StreamToolResult,
			AgentID:         a.ID,
			AgentName:       a.Name,
			ToolCall:        &callCopy,
			ToolOutput:      output,
			ToolErr:         toolErr,
			ToolAttachments: attachments,
		})
	}
}

func (a *Agent) buildRequest(state *runState) *llm.Request {
	req := llm.NewRequest(state.modelID, state.history)
	if a.Temperature != 0 {
		req.Temperature = a.Temperature
	}
	if a.TopP != 0 {
		req.TopP = a.TopP
	}
	if a.MaxTokens != 0 {
		req.MaxTokens = a.MaxTokens
	}
	if a.ReasoningEffort != "" {
		req.ReasoningEffort = a.ReasoningEffort
	}
	if a.structuredName != "" && a.structuredSchema != nil {
		req.WithStructuredOutput(a.structuredName, a.structuredSchema)
	}
	if tools := a.visibleTools(); len(tools) > 0 {
		req.Tools = tools
	}
	return req
}

func (a *Agent) visibleTools() []llm.Tool {
	tools := a.Tools.LLMTools()
	if len(tools) == 0 || a.planToolsEnabled() {
		return tools
	}
	out := tools[:0]
	for _, t := range tools {
		if isPlanTool(t.Function.Name) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (a *Agent) fail(ctx context.Context, err error) error {
	a.debugf("agent %q failed: %v", a.Name, err)
	a.publish(TopicAgentError, AgentError{AgentID: a.ID, AgentName: a.Name, Model: a.Model, Err: err})
	_ = a.chain.OnError(ctx, err)
	return err
}

func (s *runState) accumulateUsage(u llm.Usage) {
	s.usage.PromptTokens = u.PromptTokens
	s.usage.CompletionTokens = u.CompletionTokens
	s.usage.TotalTokens = u.TotalTokens
	s.usage.Cost += u.Cost
}

type accToolCall struct {
	id        string
	name      string
	typ       string
	arguments string
}

type streamAccumulator struct {
	content   string
	reasoning string
	calls     map[int]*accToolCall
	order     []int
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{calls: make(map[int]*accToolCall)}
}

func (a *streamAccumulator) append(chunk llm.StreamChunk) {
	a.content += chunk.Content
	a.reasoning += chunk.Reasoning
	for _, tc := range chunk.ToolCalls {
		acc, ok := a.calls[tc.Index]
		if !ok {
			acc = &accToolCall{}
			a.calls[tc.Index] = acc
			a.order = append(a.order, tc.Index)
		}
		if tc.ID != "" {
			acc.id = string([]byte(tc.ID))
		}
		if tc.Type != "" {
			acc.typ = string([]byte(tc.Type))
		}
		if tc.Function.Name != "" {
			acc.name = string([]byte(tc.Function.Name))
		}
		acc.arguments += tc.Function.Arguments
	}
}

func (a *streamAccumulator) toolCalls() []llm.ToolCall {
	calls := make([]llm.ToolCall, 0, len(a.order))
	for _, idx := range a.order {
		acc := a.calls[idx]
		calls = append(calls, llm.ToolCall{
			Index: idx,
			ID:    acc.id,
			Type:  acc.typ,
			Function: llm.Function{
				Name:      acc.name,
				Arguments: acc.arguments,
			},
		})
	}
	return calls
}
