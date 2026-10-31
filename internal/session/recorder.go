package session

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
)

type sessionInfo struct {
	sessionID       string
	provider        llm.Provider
	model           llm.Model
	subagent        bool
	parentSessionID string
	displayName     string
}

type Recorder struct {
	mgr      *Manager
	log      *logger.Logger
	mu       sync.Mutex
	sessions map[string]*sessionInfo
	resumed  map[string]string
	pending  map[string]*pendingMessage

	runs          RunStore
	runByAgent    map[string]string
	runSession    map[string]string
	runAgent      map[string]string
	parentByAgent map[string]string

	evMu      sync.Mutex
	events    []RunEvent
	flushStop chan struct{}
	flushDone chan struct{}

	titleWg      sync.WaitGroup
	titleStopped atomic.Bool
}

const chunksPerCommit = 10

type pendingMessage struct {
	content   string
	reasoning string
	id        string
	chunks    int
}

func NewRecorder(mgr *Manager, log *logger.Logger) *Recorder {
	return &Recorder{
		mgr:           mgr,
		log:           log,
		sessions:      make(map[string]*sessionInfo),
		resumed:       make(map[string]string),
		pending:       make(map[string]*pendingMessage),
		runByAgent:    make(map[string]string),
		runSession:    make(map[string]string),
		runAgent:      make(map[string]string),
		parentByAgent: make(map[string]string),
	}
}

func (r *Recorder) Start(bus event.Bus) error {
	if r.mgr != nil {
		if store, err := r.mgr.Runs(); err == nil {
			r.runs = store
		}
	}
	if r.runs != nil {
		r.flushStop = make(chan struct{})
		r.flushDone = make(chan struct{})
		go r.flushLoop()
	}
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.handleStarted},
		{agent.TopicAgentInput, r.handleInput},
		{agent.TopicAgentToken, r.handleToken},
		{agent.TopicAgentMessage, r.handleMessage},
		{agent.TopicAgentToolCall, r.handleToolCall},
		{agent.TopicAgentToolResult, r.handleToolResult},
		{agent.TopicAgentUsage, r.handleUsage},
		{agent.TopicAgentAsk, r.handleAsk},
		{agent.TopicAgentAskAnswer, r.handleAskAnswer},
		{agent.TopicAgentFinished, r.handleFinished},
		{agent.TopicAgentError, r.handleError},
		{agent.TopicErrorMessage, r.handleErrorMessage},
		{agent.TopicErrorMessageFinished, r.handleErrorMessageFinished},
		{agent.TopicSubAgentNotification, r.handleSubAgentNotification},
		{TopicSessionResume, r.handleResume},
		{TopicRunStarted, r.handleRunStarted},
		{TopicRunFinished, r.handleRunFinished},
		{compaction.TopicCompactionFinished, r.handleCompaction},
	}
	for _, s := range subs {
		if err := bus.Subscribe(s.topic, s.fn); err != nil {
			return fmt.Errorf("session recorder: subscribe %s: %w", s.topic, err)
		}
	}
	return nil
}

func (r *Recorder) flushPendingAll() {
	type item struct {
		agentID   string
		sessionID string
	}
	r.mu.Lock()
	items := make([]item, 0, len(r.pending))
	for agentID, p := range r.pending {
		if p == nil || (p.id == "" && p.chunks == 0 && p.content == "" && p.reasoning == "") {
			continue
		}
		if info, ok := r.sessions[agentID]; ok {
			items = append(items, item{agentID: agentID, sessionID: info.sessionID})
		}
	}
	r.mu.Unlock()
	for _, it := range items {
		r.finalizePending(it.agentID, it.sessionID, nil)
	}
}

func (r *Recorder) Stop(bus event.Bus) error {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentStarted, r.handleStarted},
		{agent.TopicAgentInput, r.handleInput},
		{agent.TopicAgentToken, r.handleToken},
		{agent.TopicAgentMessage, r.handleMessage},
		{agent.TopicAgentToolCall, r.handleToolCall},
		{agent.TopicAgentToolResult, r.handleToolResult},
		{agent.TopicAgentUsage, r.handleUsage},
		{agent.TopicAgentAsk, r.handleAsk},
		{agent.TopicAgentAskAnswer, r.handleAskAnswer},
		{agent.TopicAgentFinished, r.handleFinished},
		{agent.TopicAgentError, r.handleError},
		{agent.TopicErrorMessage, r.handleErrorMessage},
		{agent.TopicErrorMessageFinished, r.handleErrorMessageFinished},
		{agent.TopicSubAgentNotification, r.handleSubAgentNotification},
		{TopicSessionResume, r.handleResume},
		{TopicRunStarted, r.handleRunStarted},
		{TopicRunFinished, r.handleRunFinished},
		{compaction.TopicCompactionFinished, r.handleCompaction},
	}
	var firstErr error
	for _, s := range subs {
		if err := bus.Unsubscribe(s.topic, s.fn); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("session recorder: unsubscribe %s: %w", s.topic, err)
		}
	}
	r.flushPendingAll()
	r.titleStopped.Store(true)
	titleDone := make(chan struct{})
	go func() {
		r.titleWg.Wait()
		close(titleDone)
	}()
	select {
	case <-titleDone:
	case <-time.After(2 * time.Second):
	}
	if r.flushStop != nil {
		close(r.flushStop)
		<-r.flushDone
		r.flushStop = nil
	}
	r.flushEvents()
	return firstErr
}

func (r *Recorder) sessionID(agentID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.sessions[agentID]
	if !ok {
		return "", false
	}
	return info.sessionID, true
}

func (r *Recorder) getSessionInfo(agentID string) (*sessionInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info, ok := r.sessions[agentID]
	return info, ok
}

func (r *Recorder) handleResume(e SessionResume) {
	if e.AgentID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.SessionID == "" {
		delete(r.sessions, e.AgentID)
		delete(r.resumed, e.AgentID)
		return
	}
	r.resumed[e.AgentID] = e.SessionID
	if info, ok := r.sessions[e.AgentID]; ok && info.sessionID != e.SessionID {
		info.sessionID = e.SessionID
	}
}

func (r *Recorder) handleStarted(e agent.AgentStarted) {
	r.mapAgentRun(e.AgentID, e.ParentAgentID)
	r.recordRunEvent(e.AgentID, AgentEventStarted, map[string]any{
		"agent_id":         e.AgentID,
		"agent_name":       e.AgentName,
		"display_name":     e.DisplayName,
		"parent_agent_id":  e.ParentAgentID,
		"provider":         providerName(e.Provider),
		"model":            e.Model.ID,
		"reasoning_effort": e.ReasoningEffort,
	})
	r.mu.Lock()
	if info, ok := r.sessions[e.AgentID]; ok {
		info.provider = e.Provider
		info.model = e.Model
		sessID := info.sessionID
		r.mu.Unlock()
		r.persistModel(sessID, e.Provider, e.Model)
		return
	}
	sessID := r.resumed[e.AgentID]
	r.mu.Unlock()

	subagent := e.ParentAgentID != ""
	displayName := e.DisplayName
	if displayName == "" {
		displayName = e.AgentName
	}
	var parentSessionID string
	if subagent {
		parentSessionID = r.sessionIDForAgent(e.ParentAgentID)
	}

	created := false
	if sessID == "" {
		opts := CreateOptions{}
		if e.Provider != nil {
			opts.Provider = e.Provider.Name()
		}
		opts.Model = e.Model.ID
		opts.ReasoningEffort = e.ReasoningEffort
		if e.Agent != nil {
			opts.Mode = string(e.Agent.CurrentMode())
		}
		if dir, err := os.Getwd(); err == nil {
			opts.ProjectDir = dir
		}
		opts.IsSubagent = subagent
		if subagent {
			parentTitle := r.sessionTitle(parentSessionID)
			if isPlaceholderTitle(parentTitle) {
				parentTitle = ""
			}
			opts.Title = joinTitle(parentTitle, displayName)
		}

		s, err := r.mgr.Create(opts)
		if err != nil {
			r.log.Fdebug("session recorder: create session for agent %q: %v", e.AgentName, err)
			return
		}
		sessID = s.ID
		created = true
	}

	info := &sessionInfo{
		sessionID:       sessID,
		provider:        e.Provider,
		model:           e.Model,
		subagent:        subagent,
		parentSessionID: parentSessionID,
		displayName:     displayName,
	}
	r.mu.Lock()
	r.sessions[e.AgentID] = info
	r.mu.Unlock()

	if !created {
		r.persistModel(sessID, e.Provider, e.Model)
	}

	r.mgr.publish(TopicSessionAttached, SessionAttached{
		AgentID:   e.AgentID,
		SessionID: sessID,
	})
}

func (r *Recorder) persistModel(sessID string, provider llm.Provider, model llm.Model) {
	if sessID == "" {
		return
	}
	provName := ""
	if provider != nil {
		provName = provider.Name()
	}
	if err := r.mgr.SetModel(sessID, provName, model.ID); err != nil {
		r.log.Fdebug("session recorder: set model: %v", err)
	}
}

func (r *Recorder) sessionIDForAgent(agentID string) string {
	if agentID == "" {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if info, ok := r.sessions[agentID]; ok {
		return info.sessionID
	}
	return ""
}

func (r *Recorder) sessionTitle(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	s, err := r.mgr.Get(sessionID)
	if err != nil {
		return ""
	}
	return s.Title
}

func joinTitle(parentTitle, name string) string {
	parentTitle = strings.TrimSpace(parentTitle)
	name = strings.TrimSpace(name)
	switch {
	case parentTitle == "":
		return name
	case name == "":
		return parentTitle
	default:
		return parentTitle + " - " + name
	}
}

func (r *Recorder) handleInput(e agent.AgentInput) {
	r.recordRunEvent(e.AgentID, AgentEventInput, map[string]any{
		"agent_id":    e.AgentID,
		"agent_name":  e.AgentName,
		"input":       e.Input,
		"attachments": e.Attachments,
	})
	id, ok := r.sessionID(e.AgentID)
	if !ok {
		return
	}
	first := r.isFirstMessage(id)
	input := llm.UserMessage(e.Input)
	if len(e.Attachments) > 0 {
		input = llm.UserMessage(llm.ContentWithAttachments(e.Input, e.Attachments))
	}
	if _, err := r.mgr.AppendMessage(id, input); err != nil {
		r.log.Fdebug("session recorder: append input: %v", err)
		return
	}

	if first {
		if info, ok := r.getSessionInfo(e.AgentID); ok && info.provider != nil && !info.subagent {
			r.titleWg.Add(1)
			go func() {
				defer r.titleWg.Done()
				r.generateTitle(id, info.provider, info.model, attach.StripBlock(e.Input))
			}()
		}
	}
}

func (r *Recorder) isFirstMessage(sessionID string) bool {
	msgs, err := r.mgr.Messages(sessionID)
	if err != nil {
		return false
	}
	return len(msgs) == 0
}

func (r *Recorder) generateTitle(sessionID string, provider llm.Provider, model llm.Model, input string) {
	title, err := GenerateSessionTitle(provider, model, input)
	if err != nil {
		r.log.Fdebug("session recorder: generate title: %v", err)
		return
	}
	if title == "" || r.titleStopped.Load() {
		return
	}
	if err := r.mgr.SetTitle(sessionID, title); err != nil {
		r.log.Fdebug("session recorder: set title: %v", err)
		return
	}
	r.updateChildTitles(sessionID, title)
}

func (r *Recorder) updateChildTitles(parentSessionID, parentTitle string) {
	type child struct {
		sessionID string
		name      string
	}
	var children []child
	r.mu.Lock()
	for _, info := range r.sessions {
		if info.subagent && info.parentSessionID == parentSessionID {
			children = append(children, child{sessionID: info.sessionID, name: info.displayName})
		}
	}
	r.mu.Unlock()
	for _, c := range children {
		if err := r.mgr.SetTitle(c.sessionID, joinTitle(parentTitle, c.name)); err != nil {
			r.log.Fdebug("session recorder: set subagent title: %v", err)
		}
	}
}

func (r *Recorder) handleToken(e agent.AgentToken) {
	r.recordRunEvent(e.AgentID, AgentEventToken, map[string]any{
		"content":   e.Content,
		"reasoning": e.Reasoning,
	})
	r.mu.Lock()
	p := r.pending[e.AgentID]
	if p == nil {
		p = &pendingMessage{}
		r.pending[e.AgentID] = p
	}
	p.content += e.Content
	p.reasoning += e.Reasoning
	p.chunks++
	flush := p.chunks >= chunksPerCommit
	r.mu.Unlock()

	if flush {
		if id, ok := r.sessionID(e.AgentID); ok {
			r.flushPending(e.AgentID, id)
		}
	}
}

func (r *Recorder) handleMessage(e agent.AgentMessage) {
	r.recordRunEvent(e.AgentID, AgentEventMessage, map[string]any{"message": e.Message})
	id, ok := r.sessionID(e.AgentID)
	if !ok {
		return
	}
	r.mu.Lock()
	p := r.pending[e.AgentID]
	streamed := p != nil && (p.id != "" || p.chunks > 0 || p.content != "" || p.reasoning != "")
	r.mu.Unlock()
	if streamed {
		r.finalizePending(e.AgentID, id, &e.Message)
		return
	}
	if _, err := r.mgr.AppendMessage(id, e.Message); err != nil {
		r.log.Fdebug("session recorder: append message: %v", err)
	}
}

func (r *Recorder) handleError(e agent.AgentError) {
	msg := ""
	if e.Err != nil {
		msg = e.Err.Error()
	}
	r.recordRunEvent(e.AgentID, AgentEventError, map[string]any{
		"agent_id":   e.AgentID,
		"agent_name": e.AgentName,
		"message":    msg,
	})
	if id, ok := r.sessionID(e.AgentID); ok {
		r.finalizePending(e.AgentID, id, nil)
	}
}

func (r *Recorder) flushPending(agentID, sessID string) {
	r.mu.Lock()
	p := r.pending[agentID]
	if p == nil || p.chunks == 0 {
		r.mu.Unlock()
		return
	}
	content := p.content
	reasoning := p.reasoning
	id := p.id
	p.chunks = 0
	r.mu.Unlock()

	msg := assistantMessage(content, reasoning, nil)
	if id != "" {
		if err := r.mgr.UpdateMessage(Message{
			ID: id, SessionID: sessID, Content: msg.Content, Reasoning: msg.Reasoning, ToolCalls: msg.ToolCalls,
		}); err != nil {
			r.log.Fdebug("session recorder: update streaming message: %v", err)
		}
		return
	}
	rec, err := r.mgr.AppendMessage(sessID, msg)
	if err != nil {
		r.log.Fdebug("session recorder: append streaming message: %v", err)
		return
	}
	r.mu.Lock()
	if p := r.pending[agentID]; p != nil {
		p.id = rec.ID
	}
	r.mu.Unlock()
}

func (r *Recorder) finalizePending(agentID, sessID string, final *llm.Message) {
	r.mu.Lock()
	p := r.pending[agentID]
	if p == nil {
		r.mu.Unlock()
		return
	}
	content := p.content
	reasoning := p.reasoning
	id := p.id
	var calls []llm.ToolCall
	if final != nil {
		if text := llm.MessageText(*final); text != "" {
			content = text
		}
		if text, ok := final.Reasoning.(string); ok && text != "" {
			reasoning = text
		}
		calls = final.ToolCalls
	}
	delete(r.pending, agentID)
	r.mu.Unlock()

	if content == "" && reasoning == "" && len(calls) == 0 {
		return
	}
	msg := assistantMessage(content, reasoning, calls)
	if id != "" {
		if err := r.mgr.UpdateMessage(Message{
			ID: id, SessionID: sessID, Content: msg.Content, Reasoning: msg.Reasoning, ToolCalls: msg.ToolCalls,
		}); err != nil {
			r.log.Fdebug("session recorder: finalize streaming message: %v", err)
		}
		return
	}
	if _, err := r.mgr.AppendMessage(sessID, msg); err != nil {
		r.log.Fdebug("session recorder: append streamed message: %v", err)
	}
}

func assistantMessage(content, reasoning string, calls []llm.ToolCall) llm.Message {
	msg := llm.AssistantMessage(content)
	if reasoning != "" {
		msg.Reasoning = reasoning
	}
	if len(calls) > 0 {
		msg.ToolCalls = calls
	}
	return msg
}

func (r *Recorder) handleToolResult(e agent.AgentToolResult) {
	errMsg := ""
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	r.recordRunEvent(e.AgentID, AgentEventToolResult, map[string]any{
		"call_id":   e.CallID,
		"tool_name": e.ToolName,
		"output":    e.Output,
		"error":     errMsg,
	})
	if id, ok := r.sessionID(e.AgentID); ok {
		msg := llm.ToolMessage(e.Output, e.CallID)
		if len(e.Attachments) > 0 {
			msg.Content = llm.ContentWithAttachments(e.Output, e.Attachments)
		}
		if _, err := r.mgr.AppendMessage(id, msg); err != nil {
			r.log.Fdebug("session recorder: append tool result: %v", err)
		}
	}
}

func (r *Recorder) handleFinished(e agent.AgentFinished) {
	r.recordRunEvent(e.AgentID, AgentEventFinished, map[string]any{
		"output":        e.Output,
		"usage":         e.Usage,
		"iterations":    e.Iterations,
		"finish_reason": e.FinishReason,
	})
	if id, ok := r.sessionID(e.AgentID); ok {
		r.finalizePending(e.AgentID, id, nil)
		if err := r.mgr.AccumulateUsage(id, e.Usage); err != nil {
			r.log.Fdebug("session recorder: accumulate usage: %v", err)
		}
	}
}

func (r *Recorder) handleCompaction(e compaction.Event) {
	r.recordRunEvent(e.AgentID, AgentEventCompaction, map[string]any{
		"strategy": e.Strategy,
		"messages": e.Messages,
		"tokens":   e.Tokens,
	})
	r.mu.Lock()
	info, ok := r.sessions[e.AgentID]
	r.mu.Unlock()
	if !ok {
		return
	}
	oldID := info.sessionID

	newSess, err := r.mgr.CompactSession(oldID, e.CompactedMessages, e.Strategy)
	if err != nil {
		r.log.Fdebug("session recorder: compact session: %v", err)
		return
	}

	r.mu.Lock()
	info.sessionID = newSess.ID
	r.mu.Unlock()

	r.log.Fdebug("session recorder: compacted session %s -> %s (%s)", oldID, newSess.ID, e.Strategy)
}

func providerName(p llm.Provider) string {
	if p == nil {
		return ""
	}
	return p.Name()
}

func (r *Recorder) mapAgentRun(agentID, parentAgentID string) {
	if agentID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if parentAgentID != "" {
		r.parentByAgent[agentID] = parentAgentID
		if runID := r.runByAgent[parentAgentID]; runID != "" {
			r.runByAgent[agentID] = runID
		}
	}
}

func (r *Recorder) handleRunStarted(e RunStarted) {
	if e.RunID == "" || e.AgentID == "" {
		return
	}
	r.mu.Lock()
	r.runByAgent[e.AgentID] = e.RunID
	r.runSession[e.RunID] = e.SessionID
	r.runAgent[e.RunID] = e.AgentID
	r.mu.Unlock()
}

func (r *Recorder) handleRunFinished(e RunFinished) {
	r.flushEvents()
	r.mu.Lock()
	for agentID, runID := range r.runByAgent {
		if runID == e.RunID {
			delete(r.runByAgent, agentID)
			delete(r.parentByAgent, agentID)
		}
	}
	delete(r.runSession, e.RunID)
	delete(r.runAgent, e.RunID)
	r.mu.Unlock()
}

func (r *Recorder) handleToolCall(e agent.AgentToolCall) {
	r.recordRunEvent(e.AgentID, AgentEventToolCall, map[string]any{
		"call_id":   e.Call.ID,
		"tool_name": e.Call.Function.Name,
		"args":      e.Call.Function.Arguments,
		"call":      e.Call,
	})
}

func (r *Recorder) handleUsage(e agent.AgentUsage) {
	r.recordRunEvent(e.AgentID, AgentEventUsage, map[string]any{"usage": e.Usage})
}

func (r *Recorder) handleAskAnswer(e agent.AgentAskAnswer) {
	if e.AgentID == "" {
		return
	}
	r.mu.Lock()
	runID := r.runByAgent[e.AgentID]
	sessionID := r.runSession[runID]
	r.mu.Unlock()
	if runID == "" || sessionID == "" {
		return
	}
	r.appendRunEventForRun(runID, sessionID, e.AgentID, "", RunEventAskResolved, map[string]any{
		"answers": e.Answers,
	})
}

func (r *Recorder) handleAsk(e agent.AgentAsk) {
	r.recordRunEvent(e.AgentID, AgentEventAsk, map[string]any{
		"agent_id":   e.AgentID,
		"agent_name": e.AgentName,
		"questions":  e.Questions,
	})
}

func (r *Recorder) handleErrorMessage(e agent.ErrorMessage) {
	r.recordRunEvent(e.AgentID, AgentEventErrorMessage, map[string]any{"message": e.Message})
}

func (r *Recorder) handleErrorMessageFinished(e agent.ErrorMessageFinished) {
	r.recordRunEvent(e.AgentID, AgentEventErrorMessageFinish, map[string]any{})
}

func (r *Recorder) handleSubAgentNotification(e agent.SubAgentNotification) {
	r.recordRunEvent(e.ParentAgentID, AgentEventSubAgentNotice, map[string]any{
		"parent_agent_id": e.ParentAgentID,
		"subagent_name":   e.SubAgentName,
		"task_ids":        e.TaskIDs,
		"output":          e.Output,
		"error":           e.Err,
	})
}

func (r *Recorder) recordRunEvent(agentID, eventType string, data map[string]any) {
	if r.runs == nil {
		return
	}
	r.mu.Lock()
	runID := r.runByAgent[agentID]
	sessionID := r.runSession[runID]
	parentID := r.parentByAgent[agentID]
	r.mu.Unlock()
	if runID == "" || sessionID == "" {
		return
	}
	r.appendRunEventForRun(runID, sessionID, agentID, parentID, eventType, data)
}

func (r *Recorder) appendRunEventForRun(runID, sessionID, agentID, parentID, eventType string, data any) {
	if r.runs == nil || runID == "" {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	r.evMu.Lock()
	r.events = append(r.events, RunEvent{
		RunID:         runID,
		SessionID:     sessionID,
		Type:          eventType,
		AgentID:       agentID,
		ParentAgentID: parentID,
		Data:          payload,
		CreatedAt:     time.Now(),
	})
	r.evMu.Unlock()
}

func (r *Recorder) flushLoop() {
	defer close(r.flushDone)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.flushStop:
			return
		case <-ticker.C:
			r.flushEvents()
		}
	}
}

func (r *Recorder) flushEvents() {
	if r.runs == nil {
		return
	}
	r.evMu.Lock()
	if len(r.events) == 0 {
		r.evMu.Unlock()
		return
	}
	events := r.events
	r.events = nil
	r.evMu.Unlock()
	if err := r.runs.AppendRunEvents(events); err != nil && r.log != nil {
		r.log.Fdebug("session recorder: append run events: %v", err)
	}
}
