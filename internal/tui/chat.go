package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	runpkg "github.com/peggco/pegg/internal/run"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

type agentTranscript struct {
	id        string
	name      string
	items     []*components.ChatItem
	thinking  *components.ChatItem
	assistant *components.ChatItem
	toolByID  map[string]*components.ChatItem
	started   bool
	done      bool
	err       string
}

func newTranscript(id, name string) *agentTranscript {
	return &agentTranscript{id: id, name: name, toolByID: make(map[string]*components.ChatItem)}
}

func (a *App) transcriptFor(id, name string) *agentTranscript {
	if a.agent != nil && id == a.agent.ID {
		if a.main == nil {
			a.main = newTranscript(id, name)
		}
		return a.main
	}
	if t, ok := a.subs[id]; ok {
		return t
	}
	t := newTranscript(id, name)
	a.subs[id] = t
	return t
}

func (a *App) appendItem(t *agentTranscript, it *components.ChatItem) {
	if t == nil {
		return
	}
	if a.viewID == t.id {
		a.chat.AppendItem(it)
		t.items = a.chat.Items()
	} else {
		t.items = append(t.items, it)
	}
}

func (a *App) showTranscript(t *agentTranscript) {
	a.viewID = t.id
	a.chat.SetItems(t.items)
	t.items = a.chat.Items()
	a.chat.Invalidate()
	a.requestRedraw()
}

func (a *App) showMain() {
	a.showTranscript(a.main)
	a.chat.SetBack(false)
}

func (a *App) refreshChat() {
	a.chat.MarkLastDirty()
	a.requestRedraw()
}

func (a *App) subscribeChat(bus event.Bus) error {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentInput, a.onAgentInput},
		{agent.TopicAgentStarted, a.onAgentStarted},
		{agent.TopicAgentToken, a.onAgentToken},
		{agent.TopicAgentMessage, a.onAgentMessage},
		{agent.TopicAgentToolCall, a.onAgentToolCall},
		{agent.TopicAgentToolResult, a.onAgentToolResult},
		{agent.TopicAgentUsage, a.onAgentUsage},
		{agent.TopicAgentFinished, a.onAgentFinished},
		{agent.TopicAgentError, a.onAgentError},
		{agent.TopicErrorMessage, a.onErrorMessage},
		{agent.TopicErrorMessageFinished, a.onErrorMessageFinished},
		{session.TopicSessionAttached, a.onSessionAttached},
		{session.TopicSessionCompacted, a.onSessionCompacted},
		{session.TopicSessionUpdated, a.onSessionUpdated},
		{session.TopicSessionDeleted, a.onSessionDeleted},
		{agent.TopicAgentAsk, a.onAgentAsk},
		{agent.TopicAgentAskAnswer, a.onAgentAskAnswer},
		{agent.TopicSubAgentNotification, a.onSubAgentNotification},
		{compaction.TopicCompactionFinished, a.onCompactionFinished},
	}
	for _, s := range subs {
		if err := bus.Subscribe(s.topic, s.fn); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) unsubscribeChat(bus event.Bus) {
	subs := []struct {
		topic string
		fn    any
	}{
		{agent.TopicAgentInput, a.onAgentInput},
		{agent.TopicAgentStarted, a.onAgentStarted},
		{agent.TopicAgentToken, a.onAgentToken},
		{agent.TopicAgentMessage, a.onAgentMessage},
		{agent.TopicAgentToolCall, a.onAgentToolCall},
		{agent.TopicAgentToolResult, a.onAgentToolResult},
		{agent.TopicAgentUsage, a.onAgentUsage},
		{agent.TopicAgentFinished, a.onAgentFinished},
		{agent.TopicAgentError, a.onAgentError},
		{agent.TopicErrorMessage, a.onErrorMessage},
		{agent.TopicErrorMessageFinished, a.onErrorMessageFinished},
		{session.TopicSessionAttached, a.onSessionAttached},
		{session.TopicSessionCompacted, a.onSessionCompacted},
		{session.TopicSessionUpdated, a.onSessionUpdated},
		{session.TopicSessionDeleted, a.onSessionDeleted},
		{agent.TopicAgentAsk, a.onAgentAsk},
		{agent.TopicAgentAskAnswer, a.onAgentAskAnswer},
		{agent.TopicSubAgentNotification, a.onSubAgentNotification},
		{compaction.TopicCompactionFinished, a.onCompactionFinished},
	}
	for _, s := range subs {
		_ = bus.Unsubscribe(s.topic, s.fn)
	}
}

func (a *App) onAgentInput(e agent.AgentInput) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	it := &components.ChatItem{Kind: components.ItemUser, ID: e.AgentID, Text: e.Input, Attachments: e.Attachments}
	a.appendItem(t, it)
	if e.AgentID != a.agent.ID {
		if sub := a.subItemByID[e.AgentID]; sub != nil {
			sub.SubagentTask = e.Input
			if a.viewID == a.agent.ID {
				a.chat.InvalidateItemPtr(sub)
			}
		}
	}
	if a.viewID == t.id {
		a.requestRedraw()
	}
}

func (a *App) onAgentStarted(e agent.AgentStarted) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, e.ParentAgentID) {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	t.started = true
	if e.AgentID == a.agent.ID {
		a.running = true
		a.refreshHomeLocked()
		a.requestRedraw()
		return
	}
	if a.main == nil {
		a.main = newTranscript(a.agent.ID, i18n.T("chat.orchestrator"))
	}
	sub := &components.ChatItem{
		Kind:           components.ItemSubagent,
		ID:             e.AgentID,
		AgentID:        e.AgentID,
		SubagentName:   e.AgentName,
		SubagentStatus: "running",
	}
	a.subItemByID[e.AgentID] = sub
	a.appendItem(a.main, sub)
	if a.viewID == a.agent.ID {
		a.requestRedraw()
	}
}

func (a *App) onAgentToken(e agent.AgentToken) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	if e.Reasoning != "" {
		if t.thinking == nil {
			t.thinking = &components.ChatItem{Kind: components.ItemThinking, ID: e.AgentID}
			a.appendItem(t, t.thinking)
		}
		t.thinking.Reasoning += e.Reasoning
	}
	if e.Content != "" {
		if t.assistant == nil {
			t.assistant = &components.ChatItem{Kind: components.ItemAssistant, ID: e.AgentID}
			a.appendItem(t, t.assistant)
		}
		t.assistant.Text += e.Content
	}
	if a.viewID == t.id {
		a.refreshChat()
	}
}

func (a *App) onAgentMessage(e agent.AgentMessage) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	msg := e.Message

	t.thinking = nil

	if t.assistant != nil {
		t.assistant = nil
		if a.viewID == t.id {
			a.chat.Invalidate()
			a.requestRedraw()
		}
		return
	}

	if text := messageContent(msg); text != "" {
		it := &components.ChatItem{Kind: components.ItemAssistant, ID: e.AgentID, Text: text}
		a.appendItem(t, it)
	}
	if a.viewID == t.id {
		a.chat.Invalidate()
		a.requestRedraw()
	}
}

func (a *App) onAgentToolCall(e agent.AgentToolCall) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	a.addToolItem(t, e.Call, e.AgentID)
	if e.AgentID != a.agent.ID {
		if sub := a.subItemByID[e.AgentID]; sub != nil {
			sub.SubagentActivity = formatActivity(e.Call)
			if a.viewID == a.agent.ID {
				a.chat.InvalidateItemPtr(sub)
			}
		}
	}
	if a.viewID == t.id {
		a.requestRedraw()
	}
}

func formatActivity(call llm.ToolCall) string {
	name := call.Function.Name
	args := call.Function.Arguments
	switch name {
	case "read":
		var p struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			return i18n.Tf("chat.activity.reading", p.FilePath)
		}
	case "write":
		var p struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			return i18n.Tf("chat.activity.writing", p.FilePath)
		}
	case "edit":
		var p struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			return i18n.Tf("chat.activity.editing", p.FilePath)
		}
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Command != "" {
			cmd := p.Command
			if len(cmd) > 50 {
				cmd = cmd[:50] + "…"
			}
			return i18n.Tf("chat.activity.running", cmd)
		}
	case "glob":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			return i18n.Tf("chat.activity.finding", p.Pattern)
		}
	case "grep":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			return i18n.Tf("chat.activity.searching", p.Pattern)
		}
	case "list":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Path != "" {
			return i18n.Tf("chat.activity.listing", p.Path)
		}
	case "webfetch":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.URL != "" {
			return i18n.Tf("chat.activity.fetching", p.URL)
		}
	}
	return name
}

func (a *App) onAgentToolResult(e agent.AgentToolResult) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	it, ok := t.toolByID[e.CallID]
	if !ok {
		it = &components.ChatItem{
			Kind:     components.ItemTool,
			ID:       e.CallID,
			ToolName: e.ToolName,
		}
		t.toolByID[e.CallID] = it
		a.appendItem(t, it)
	}
	if e.Err != nil {
		it.ToolErr = e.Err.Error()
	} else {
		it.ToolOutput = e.Output
	}
	if a.viewID == t.id {
		a.chat.InvalidateItemPtr(it)
		a.requestRedraw()
	}
}

func (a *App) onAgentFinished(e agent.AgentFinished) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	t.done = true
	t.thinking = nil
	t.assistant = nil
	if e.AgentID == a.agent.ID {
		a.running = false
		a.clearEscHint()
		a.refreshHomeLocked()
		a.appendItem(t, &components.ChatItem{Kind: components.ItemFinished, ID: e.AgentID})
		if a.viewID == t.id {
			a.chat.Invalidate()
			a.requestRedraw()
		}
		return
	}
	if sub := a.subItemByID[e.AgentID]; sub != nil {
		sub.SubagentStatus = "finished"
		sub.SubagentOutput = e.Output
		sub.SubagentActivity = ""
		sub.SubagentUsage = e.Usage
		if a.viewID == a.agent.ID {
			a.chat.InvalidateItemPtr(sub)
		}
	}
	a.appendItem(t, &components.ChatItem{Kind: components.ItemFinished, ID: e.AgentID})
	if a.viewID == t.id {
		a.chat.Invalidate()
		a.requestRedraw()
	}
}

func (a *App) onAgentError(e agent.AgentError) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	t.done = true
	t.err = e.Err.Error()
	t.thinking = nil
	t.assistant = nil
	if e.AgentID == a.agent.ID {
		a.running = false
		a.clearEscHint()
		a.refreshHomeLocked()
		a.appendItem(t, &components.ChatItem{Kind: components.ItemError, Text: e.Err.Error()})
		if a.viewID == t.id {
			a.chat.Invalidate()
			a.requestRedraw()
		}
		return
	}
	if sub := a.subItemByID[e.AgentID]; sub != nil {
		sub.SubagentStatus = "error"
		sub.SubagentActivity = ""
		sub.SubagentOutput = e.Err.Error()
		if a.viewID == a.agent.ID {
			a.chat.InvalidateItemPtr(sub)
		}
	}
	a.appendItem(t, &components.ChatItem{Kind: components.ItemError, Text: e.Err.Error()})
	if a.viewID == t.id {
		a.chat.Invalidate()
		a.requestRedraw()
	}
}

func (a *App) onErrorMessage(e agent.ErrorMessage) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.agent == nil || e.AgentID != a.agent.ID {
		return
	}
	a.errorMsg = e.Message
	a.requestRedraw()
}

func (a *App) onErrorMessageFinished(e agent.ErrorMessageFinished) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.agent == nil || e.AgentID != a.agent.ID {
		return
	}
	a.errorMsg = ""
	a.requestRedraw()
}

func (a *App) onCompactionFinished(e compaction.Event) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	t := a.transcriptFor(e.AgentID, e.AgentName)
	text := i18n.Tf("chat.compaction", e.Strategy, e.Messages, e.Tokens)
	it := &components.ChatItem{Kind: components.ItemCompaction, ID: e.AgentID, Text: text}
	a.appendItem(t, it)
	if a.viewID == t.id {
		a.requestRedraw()
	}
}

func (a *App) onAgentAsk(e agent.AgentAsk) {
	a.chatMu.Lock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		a.chatMu.Unlock()
		return
	}
	a.askAgentID = e.AgentID
	a.chatMu.Unlock()
	qs := make([]components.AskQuestion, len(e.Questions))
	for i, q := range e.Questions {
		qs[i] = components.AskQuestion{
			ID:       q.ID,
			Question: q.Question,
			Type:     q.Type,
			Options:  q.Options,
			Required: q.Required,
		}
	}
	a.home.SetAsk(qs)
	attachRunID := a.attachRunID
	a.home.AskPicker().SetOnSend(func(answers map[string]string) {
		if attachRunID != "" && a.deps.Runs != nil {
			_ = a.deps.Runs.Answer(attachRunID, answers)
			return
		}
		a.bus.Publish(agent.TopicAgentAskAnswer, agent.AgentAskAnswer{
			AgentID: e.AgentID,
			Answers: answers,
		})
	})
	a.home.AskPicker().SetOnDone(func() {
		a.chatMu.Lock()
		a.askAgentID = ""
		a.chatMu.Unlock()
		a.home.ClearAsk()
		a.requestRedraw()
	})
	a.requestRedraw()
}

func (a *App) onAgentAskAnswer(e agent.AgentAskAnswer) {
	a.chatMu.Lock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		a.chatMu.Unlock()
		return
	}
	match := a.askAgentID == "" || a.askAgentID == e.AgentID
	if match {
		a.askAgentID = ""
	}
	a.chatMu.Unlock()
	if match && a.home != nil && a.home.AskOpen() {
		a.home.ClearAsk()
		a.requestRedraw()
	}
}

func (a *App) onAgentUsage(e agent.AgentUsage) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.agent == nil || !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	if e.AgentID == a.agent.ID {
		a.usage.PromptTokens = e.Usage.PromptTokens
		a.usage.CompletionTokens = e.Usage.CompletionTokens
		a.usage.TotalTokens = e.Usage.TotalTokens
		a.usage.Cost = e.Usage.Cost
		a.refreshHomeLocked()
		a.requestRedraw()
		return
	}
	if sub := a.subItemByID[e.AgentID]; sub != nil {
		sub.SubagentUsage = e.Usage
		if a.viewID == a.agent.ID {
			a.chat.InvalidateItemPtr(sub)
			a.requestRedraw()
		}
	}
}

func (a *App) onSessionAttached(e session.SessionAttached) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if !a.acceptAgentEventLocked(e.AgentID, "") {
		return
	}
	if a.agent != nil && e.AgentID != a.agent.ID {
		return
	}
	if a.session == nil {
		a.session = &activeSession{info: settings.SessionInfo{ID: e.SessionID, Mode: string(a.mode)}, viewID: e.SessionID}
		a.refreshHomeLocked()
	}
}

func (a *App) onSessionCompacted(e session.SessionCompacted) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if rt := a.runtimes[e.ParentSessionID]; rt != nil && rt.agent != nil {
		a.remapRuntimeLocked(e.ParentSessionID, e.SessionID, rt.agent.ID)
	}
	if a.session == nil || a.session.info.ID != e.ParentSessionID {
		return
	}
	a.session.info.ID = e.SessionID
	a.session.info.CompactionParentID = e.ParentSessionID
	if a.session.viewID == e.ParentSessionID {
		a.session.viewID = e.SessionID
	}
	a.requestRedraw()
}

func (a *App) addToolItem(t *agentTranscript, call llm.ToolCall, agentID string) {
	it := &components.ChatItem{
		Kind:     components.ItemTool,
		ID:       call.ID,
		ToolName: call.Function.Name,
		ToolArgs: call.Function.Arguments,
	}

	name := call.Function.Name
	args := call.Function.Arguments

	switch name {
	case "edit":
		var p struct {
			FilePath  string `json:"filePath"`
			OldString string `json:"oldString"`
			NewString string `json:"newString"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.OldString != "" {
			it.Diff = components.ComputeDiff(p.OldString, p.NewString)
			if p.FilePath != "" {
				it.ToolName = "edit:" + p.FilePath
			}
		}
	case "read":
		var p struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			it.ToolName = "read:" + p.FilePath
		}
	case "write":
		var p struct {
			FilePath string `json:"filePath"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			it.ToolName = "write:" + p.FilePath
			it.WriteContent = p.Content
		}
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Command != "" {
			cmd := p.Command
			if len(cmd) > 60 {
				cmd = cmd[:60] + "…"
			}
			it.ToolName = "bash:" + cmd
		}
	case "glob":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			it.ToolName = "glob:" + p.Pattern
		}
	case "grep":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			it.ToolName = "grep:" + p.Pattern
		}
	case "askuserquestion":
		it.ToolArgs = args
		it.ToolName = "askuserquestion"
	case "todoread":
		it.ToolName = "todoread"
	case "todowrite":
		it.ToolName = "todowrite"
		var p struct {
			Todos []struct {
				ID string `json:"id"`
			} `json:"todos"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && len(p.Todos) > 0 {
			it.ToolName = fmt.Sprintf("todowrite:%d items", len(p.Todos))
		}
	case "webfetch":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.URL != "" {
			it.ToolName = "webfetch:" + p.URL
		}
	case "list":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Path != "" {
			it.ToolName = "list:" + p.Path
		}
	}

	t.toolByID[call.ID] = it
	a.appendItem(t, it)
}

func (a *App) submitMessage(input string) {
	input = strings.TrimSpace(input)

	a.chatMu.Lock()
	attachments := make([]llm.Attachment, len(a.home.AttachmentBar().Attachments()))
	copy(attachments, a.home.AttachmentBar().Attachments())
	if input == "" && len(attachments) == 0 {
		a.chatMu.Unlock()
		return
	}
	a.home.AttachmentBar().Clear()
	a.refreshMentionItemsLocked()
	a.refreshHomeLocked()
	a.chatMu.Unlock()

	input = dispatchSubmit(input)

	a.chatMu.Lock()
	if a.running {
		a.chatMu.Unlock()
		return
	}
	a.running = true
	a.refreshHomeLocked()
	a.chatMu.Unlock()

	go a.runAgentWithAttachments(input, attachments)
}

func (a *App) runAgent(input string) {
	a.chatMu.Lock()
	attachments := make([]llm.Attachment, len(a.home.AttachmentBar().Attachments()))
	copy(attachments, a.home.AttachmentBar().Attachments())
	a.home.AttachmentBar().Clear()
	a.refreshMentionItemsLocked()
	a.refreshHomeLocked()
	a.chatMu.Unlock()
	a.runAgentWithAttachments(input, attachments)
}

func (a *App) runAgentWithAttachments(input string, attachments []llm.Attachment) {
	rt, lease, ctx, cancel, history, err := a.prepareRunCurrent(input, attachments)
	if err != nil {
		a.chatMu.Lock()
		a.running = false
		a.refreshHomeLocked()
		a.chatMu.Unlock()
		a.showError(err.Error())
		return
	}
	defer a.finishAgentRun(rt, lease, cancel)

	handler := func(agent.StreamEvent) error { return nil }
	var result *agent.RunResult
	if a.deps.Runs != nil {
		result, _ = a.deps.Runs.Stream(ctx, lease, rt.agent, input, history, handler)
	} else if len(history) > 0 {
		result, _ = rt.agent.ResumeStream(ctx, input, history, handler)
	} else {
		result, _ = rt.agent.RunStream(ctx, input, handler)
	}
	a.completeAgentRun(rt, result)
}

func (a *App) prepareAgentRun(input string, attachments []llm.Attachment) (*runpkg.Lease, context.Context, context.CancelFunc, []llm.Message, error) {
	_, lease, ctx, cancel, history, err := a.prepareRunCurrent(input, attachments)
	return lease, ctx, cancel, history, err
}

func (a *App) prepareRunCurrent(input string, attachments []llm.Attachment) (*sessionRuntime, *runpkg.Lease, context.Context, context.CancelFunc, []llm.Message, error) {
	ctx, cancel := context.WithCancel(a.ctx)

	a.chatMu.Lock()
	if a.session == nil && a.deps.Sessions != nil {
		dir, _ := os.Getwd()
		sess, err := a.deps.Sessions.Create(session.CreateOptions{
			Provider:        a.model.provider,
			Model:           a.model.model.ID,
			ReasoningEffort: a.reasoningEffort,
			Mode:            string(a.mode),
			ProjectDir:      dir,
		})
		if err != nil {
			a.chatMu.Unlock()
			cancel()
			return nil, nil, ctx, cancel, nil, fmt.Errorf("create session: %w", err)
		}
		a.session = &activeSession{
			info: settings.SessionInfo{
				ID:              sess.ID,
				Title:           sess.Title,
				Provider:        sess.Provider,
				Model:           sess.Model,
				ReasoningEffort: sess.ReasoningEffort,
				Mode:            sess.Mode,
				Usage:           sess.Usage,
			},
			viewID: sess.ID,
		}
	}
	sessionID := ""
	if a.session != nil {
		sessionID = a.session.info.ID
	}
	rt := a.runtimeForLocked(sessionID)
	if rt == nil {
		a.chatMu.Unlock()
		cancel()
		return nil, nil, ctx, cancel, nil, errors.New("create session agent")
	}
	rt.model = a.model
	rt.reasoningEffort = a.reasoningEffort
	rt.mode = a.mode
	if a.agent != rt.agent {
		a.agent = rt.agent
		a.viewID = rt.agent.ID
		a.resetTranscriptsLocked()
		a.loadSessionIntoChatLocked()
	}
	a.chatMu.Unlock()

	lease, ctx, cancel, history, err := a.prepareRuntimeRun(ctx, cancel, rt, attachments)
	return rt, lease, ctx, cancel, history, err
}

func (a *App) prepareRuntimeRun(ctx context.Context, cancel context.CancelFunc, rt *sessionRuntime, attachments []llm.Attachment) (*runpkg.Lease, context.Context, context.CancelFunc, []llm.Message, error) {
	a.chatMu.Lock()
	current := a.session != nil && a.session.info.ID == rt.id
	if rt.agent.Bus == nil {
		rt.agent.Bus = a.bus
	}
	if rt.model.model.ID != "" && a.deps.LLM != nil {
		if prov, err := a.deps.LLM.Provider(rt.model.provider); err == nil {
			rt.agent.SetModelProvider(rt.model.model, prov)
		}
	}
	rt.agent.ReasoningEffort = rt.reasoningEffort
	rt.agent.Attachments = attachments
	rt.agent.SetMode(rt.mode)

	history := rt.history
	if current {
		a.agent = rt.agent
		a.history = history
		a.syncHistorySystemPromptLocked()
		history = a.history
	}
	rt.history = history
	rt.cancel = cancel
	rt.running = true
	if current {
		a.agentCancel = cancel
		a.running = true
		a.refreshHomeLocked()
	}
	if rt.id != "" && a.bus != nil {
		a.bus.Publish(session.TopicSessionResume, session.SessionResume{
			AgentID:   rt.agent.ID,
			SessionID: rt.id,
		})
	}
	a.chatMu.Unlock()

	var lease *runpkg.Lease
	if a.deps.Runs != nil && rt.id != "" {
		var err error
		lease, err = a.deps.Runs.Claim(ctx, runpkg.ClaimRequest{
			SessionID: rt.id,
			AgentID:   rt.agent.ID,
			AgentName: rt.agent.Name,
			Provider:  rt.model.provider,
			Model:     rt.model.model.ID,
			BaseSeq:   a.deps.Runs.BaseSeq(rt.id),
		})
		if err != nil {
			a.chatMu.Lock()
			rt.running = false
			rt.cancel = nil
			if current {
				a.running = false
				a.agentCancel = nil
				a.refreshHomeLocked()
			}
			a.chatMu.Unlock()
			cancel()
			var busy *runpkg.BusyError
			if errors.As(err, &busy) {
				return nil, ctx, cancel, nil, fmt.Errorf("this session is running in another client; wait for it to finish or cancel it there")
			}
			return nil, ctx, cancel, nil, fmt.Errorf("claim session: %w", err)
		}
		a.chatMu.Lock()
		rt.lease = lease
		if current {
			a.agentLease = lease
		}
		a.chatMu.Unlock()
	}
	if current && rt.id != "" {
		a.ensureFollow(rt.id)
	}
	return lease, ctx, cancel, history, nil
}

func (a *App) syncHistorySystemPromptLocked() {
	if a.agent == nil || a.agent.SystemPrompt == "" || len(a.history) == 0 {
		return
	}
	if a.history[0].Role == llm.RoleSystem {
		a.history[0] = llm.SystemMessage(a.agent.SystemPrompt)
		return
	}
	a.history = append([]llm.Message{llm.SystemMessage(a.agent.SystemPrompt)}, a.history...)
}

func (a *App) finishAgentRun(rt *sessionRuntime, lease *runpkg.Lease, cancel context.CancelFunc) {
	a.chatMu.Lock()
	if rt != nil {
		if lease == nil {
			lease = rt.lease
		}
		rt.lease = nil
		rt.cancel = nil
		current := a.session != nil && a.session.info.ID == rt.id
		if current {
			a.agentLease = nil
			a.agentCancel = nil
		}
	} else {
		lease = a.agentLease
		a.agentLease = nil
		a.agentCancel = nil
	}
	live := rt == nil || a.runtimes[rt.id] == rt
	a.chatMu.Unlock()
	if lease != nil {
		lease.Finish(session.RunStateDone, "")
	}
	cancel()
	if live && rt != nil && rt.agent != nil && rt.agent.HasPendingNotifications() {
		go a.continueRuntime(rt)
	}
}

func (a *App) completeAgentRun(rt *sessionRuntime, result *agent.RunResult) {
	if rt == nil {
		return
	}
	a.chatMu.Lock()
	current := a.session != nil && a.session.info.ID == rt.id
	if result != nil {
		rt.history = result.History
		if current {
			a.history = result.History
		}
	}
	rt.running = false
	if current {
		a.running = false
		a.refreshHomeLocked()
	}
	a.chatMu.Unlock()
	if current {
		a.requestRedraw()
	}
}

func (a *App) continueAgent() {
	a.chatMu.Lock()
	sid := a.currentSessionIDLocked()
	rt := a.runtimes[sid]
	if rt == nil {
		a.chatMu.Unlock()
		return
	}
	a.chatMu.Unlock()
	a.continueRuntime(rt)
}

func (a *App) continueRuntime(rt *sessionRuntime) {
	a.chatMu.Lock()
	if rt.running || rt.cancel != nil {
		a.chatMu.Unlock()
		return
	}
	current := a.session != nil && a.session.info.ID == rt.id
	a.chatMu.Unlock()

	ctx, cancel := context.WithCancel(a.ctx)
	lease, ctx, cancel, history, err := a.prepareRuntimeRun(ctx, cancel, rt, nil)
	if err != nil {
		cancel()
		if current {
			a.showError(err.Error())
		}
		return
	}
	defer a.finishAgentRun(rt, lease, cancel)

	var result *agent.RunResult
	if a.deps.Runs != nil {
		result, _ = a.deps.Runs.Continue(ctx, lease, rt.agent, history)
	} else {
		result, _ = rt.agent.Continue(ctx, history)
	}
	a.completeAgentRun(rt, result)
}

func (a *App) onSubAgentNotification(e agent.SubAgentNotification) {
	a.chatMu.Lock()
	if a.agent == nil || e.ParentAgentID != a.agent.ID || !a.acceptAgentEventLocked(e.ParentAgentID, "") {
		a.chatMu.Unlock()
		return
	}
	if a.running || a.agentCancel != nil {
		a.chatMu.Unlock()
		return
	}
	a.chatMu.Unlock()
	go a.continueAgent()
}

func (a *App) activateItem(it *components.ChatItem) {
	a.chatMu.Lock()
	switch it.Kind {
	case components.ItemThinking, components.ItemTool:
		it.Expanded = !it.Expanded
		a.chat.InvalidateItemPtr(it)
		a.chatMu.Unlock()
		a.requestRedraw()
	case components.ItemSubagent:
		if it.SubagentStatus == "running" {
			if t := a.subs[it.AgentID]; t != nil {
				a.showTranscript(t)
				a.chat.SetBack(true)
			}
			a.chatMu.Unlock()
			return
		}
		it.Expanded = !it.Expanded
		a.chat.InvalidateItemPtr(it)
		a.chatMu.Unlock()
		a.requestRedraw()
	case components.ItemUser:
		mainID := ""
		if a.agent != nil {
			mainID = a.agent.ID
		}
		isMain := a.viewID == mainID
		a.chatMu.Unlock()
		if isMain {
			a.openUserMenu(it)
		}
	default:
		a.chatMu.Unlock()
	}
}

func (a *App) openUserMenu(it *components.ChatItem) {
	if a.deps.Sessions == nil {
		return
	}
	sid, mid, ok := a.resolveUserMessage(it)
	if !ok {
		return
	}
	text := it.Text
	items := []components.MenuItem{
		{Label: i18n.T("chat.menu.copy"), OnSelect: func() { a.copyText(text) }},
		{Label: i18n.T("chat.menu.revert"), OnSelect: func() { a.doRevert(sid, mid, it) }},
		{Label: i18n.T("chat.menu.fork"), OnSelect: func() { a.forkMessage(sid, mid, it) }},
	}
	a.showMenu(items)
}

func (a *App) showMenu(items []components.MenuItem) {
	menu := components.NewMenu(items, a.mouseX, a.mouseY)
	menu.SetOnClose(func() {
		a.setOverlay(nil)
		a.requestRedraw()
	})
	a.setOverlay(menu)
	a.requestRedraw()
}

func (a *App) closeMenu() {
	a.setOverlay(nil)
	a.requestRedraw()
}

func (a *App) resolveUserMessage(it *components.ChatItem) (string, string, bool) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if it.MsgID != "" && it.MsgSessionID != "" {
		return it.MsgSessionID, it.MsgID, true
	}
	if a.session == nil || a.deps.Sessions == nil || a.chat == nil {
		return "", "", false
	}
	sid := a.session.info.ID
	if sid == "" {
		return "", "", false
	}
	items := a.chat.Items()
	userIndex, total := -1, 0
	for _, candidate := range items {
		if candidate.Kind != components.ItemUser {
			continue
		}
		if candidate == it {
			userIndex = total
		}
		total++
	}
	if userIndex < 0 {
		return "", "", false
	}
	k := total - 1 - userIndex
	msgs, err := a.deps.Sessions.Messages(sid)
	if err != nil {
		return "", "", false
	}
	var users []session.Message
	for _, m := range msgs {
		if m.Role == llm.RoleUser {
			users = append(users, m)
		}
	}
	if k < 0 || k >= len(users) {
		return "", "", false
	}
	target := users[len(users)-1-k]
	return sid, target.ID, true
}

func (a *App) copyText(text string) {
	a.closeMenu()
	if text == "" {
		return
	}
	a.screen.SetClipboard([]byte(text))
}

func (a *App) doRevert(sid, mid string, it *components.ChatItem) {
	a.closeMenu()
	if a.deps.Sessions == nil {
		return
	}
	if _, _, err := a.deps.Sessions.RevertFrom(sid, mid); err != nil {
		a.showError(i18n.Tf("chat.error.revert", err.Error()))
		return
	}
	info, ok := a.sessionInfoFor(sid)
	if !ok {
		return
	}
	a.activateSession(info)
	a.restoreToInput(it)
}

func (a *App) forkMessage(sid, mid string, it *components.ChatItem) {
	a.closeMenu()
	if a.deps.Sessions == nil {
		return
	}
	fork, err := a.deps.Sessions.ForkBefore(sid, mid)
	if err != nil {
		a.showError(i18n.Tf("chat.error.fork", err.Error()))
		return
	}
	info, ok := a.sessionInfoFor(fork.ID)
	if !ok {
		return
	}
	a.activateSession(info)
	a.restoreToInput(it)
}

func (a *App) sessionInfoFor(id string) (settings.SessionInfo, bool) {
	sess, err := a.deps.Sessions.Get(id)
	if err != nil {
		a.showError(i18n.Tf("chat.error.loadSession", err.Error()))
		return settings.SessionInfo{}, false
	}
	msgs, err := a.deps.Sessions.Messages(id)
	if err != nil {
		a.showError(i18n.Tf("chat.error.loadMessages", err.Error()))
		return settings.SessionInfo{}, false
	}
	return settings.SessionInfo{
		ID:                 sess.ID,
		Title:              sess.Title,
		Provider:           sess.Provider,
		Model:              sess.Model,
		ReasoningEffort:    sess.ReasoningEffort,
		Mode:               sess.Mode,
		CompactionParentID: sess.CompactionParentID,
		Messages:           msgs,
		Usage:              sess.Usage,
	}, true
}

func (a *App) restoreToInput(it *components.ChatItem) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.home == nil {
		return
	}
	a.home.Input().SetValue(it.Text)
	for _, att := range it.Attachments {
		a.home.AttachmentBar().Add(att)
	}
	a.refreshMentionItemsLocked()
	a.refreshHomeLocked()
	a.requestRedraw()
}

func (a *App) openSubagentHistory(agentID string) {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if t := a.subs[agentID]; t != nil {
		a.showTranscript(t)
		a.chat.SetBack(true)
	}
}

func (a *App) backFromSubagent() {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.main != nil {
		a.showMain()
	}
}

func (a *App) loadMore() {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	mainID := ""
	if a.agent != nil {
		mainID = a.agent.ID
	}
	if !a.chat.HasMore() || a.session == nil || a.deps.Sessions == nil || a.viewID != mainID {
		return
	}
	viewID := a.session.viewID
	if viewID == "" {
		viewID = a.session.info.ID
	}
	msgs, err := a.deps.Sessions.Messages(viewID)
	if err != nil {
		a.chat.SetHasMore(false)
		return
	}
	var older []session.Message
	for _, m := range msgs {
		if m.Seq < a.loadedFloor {
			older = append(older, m)
		}
	}
	if len(older) == 0 {
		a.loadParentSessionLocked()
		return
	}
	sort.Slice(older, func(i, j int) bool { return older[i].Seq < older[j].Seq })
	const batch = 50
	moreBelow := len(older) > batch
	if moreBelow {
		older = older[len(older)-batch:]
	}
	a.loadedFloor = older[0].Seq
	a.chat.SetHasMore(moreBelow || a.session.info.CompactionParentID != "")
	items := messagesToItems(older)
	if len(items) > 0 {
		a.main.items = append(items, a.main.items...)
		a.chat.PrependItems(items)
		a.chat.SetBack(false)
	}
}

func (a *App) loadParentSessionLocked() {
	parentID := a.session.info.CompactionParentID
	if parentID == "" {
		a.chat.SetHasMore(false)
		return
	}
	parent, err := a.deps.Sessions.Get(parentID)
	if err != nil {
		a.chat.SetHasMore(false)
		return
	}
	msgs, err := a.deps.Sessions.Messages(parentID)
	if err != nil {
		a.chat.SetHasMore(false)
		return
	}
	const batch = 50
	moreBelow := len(msgs) > batch
	if moreBelow {
		msgs = msgs[len(msgs)-batch:]
	}
	if len(msgs) == 0 {
		a.session.info.CompactionParentID = parent.CompactionParentID
		a.session.viewID = parentID
		a.loadParentSessionLocked()
		return
	}
	a.loadedFloor = msgs[0].Seq
	divider := &components.ChatItem{Kind: components.ItemCompactionDivider, Text: i18n.T("chat.compactedDivider")}
	items := append([]*components.ChatItem{divider}, messagesToItems(msgs)...)
	a.main.items = append(items, a.main.items...)
	a.chat.PrependItems(items)
	a.chat.SetHasMore(moreBelow || parent.CompactionParentID != "")
	a.chat.SetBack(false)
	a.session.viewID = parentID
	a.session.info.CompactionParentID = parent.CompactionParentID
}

func messagesToItems(msgs []session.Message) []*components.ChatItem {
	var out []*components.ChatItem
	toolByID := make(map[string]*components.ChatItem)
	pendingToolResults := make(map[string]string)

	isErrorOutput := func(s string) bool {
		return strings.HasPrefix(s, "Error: ")
	}

	var pendingReasoning string
	var pendingText string
	var pendingToolCalls []llm.ToolCall

	flushAssistant := func() {
		if pendingReasoning != "" {
			out = append(out, &components.ChatItem{Kind: components.ItemThinking, Reasoning: pendingReasoning, Expanded: false})
		}
		for _, tc := range pendingToolCalls {
			it := &components.ChatItem{
				Kind:     components.ItemTool,
				ID:       tc.ID,
				ToolName: tc.Function.Name,
				ToolArgs: tc.Function.Arguments,
			}
			enrichToolItem(it)
			toolByID[tc.ID] = it
			if output, ok := pendingToolResults[tc.ID]; ok {
				if isErrorOutput(output) {
					it.ToolErr = output
				} else {
					it.ToolOutput = output
				}
				delete(pendingToolResults, tc.ID)
			}
			out = append(out, it)
		}
		if pendingText != "" {
			out = append(out, &components.ChatItem{Kind: components.ItemAssistant, Text: pendingText})
		}
		pendingReasoning = ""
		pendingText = ""
		pendingToolCalls = nil
	}

	for _, m := range msgs {
		switch m.Role {
		case llm.RoleUser:
			flushAssistant()
			out = append(out, &components.ChatItem{Kind: components.ItemUser, Text: messageText(m), Attachments: messageAttachments(m), MsgID: m.ID, MsgSessionID: m.SessionID})
		case llm.RoleAssistant:
			pendingReasoning += messageReasoning(m)
			pendingText += messageText(m)
			if len(m.ToolCalls) > 0 {
				pendingToolCalls = m.ToolCalls
				flushAssistant()
			}
		case llm.RoleTool:
			text := messageText(m)
			if text == "" {
				text = fmt.Sprint(m.Content)
			}
			if it, ok := toolByID[m.ToolCallID]; ok {
				if isErrorOutput(text) {
					it.ToolErr = text
				} else {
					it.ToolOutput = text
				}
			} else {
				pendingToolResults[m.ToolCallID] = text
			}
		}
	}
	flushAssistant()

	return out
}

func messageReasoning(m session.Message) string {
	switch c := m.Reasoning.(type) {
	case string:
		return c
	}
	return ""
}

func enrichToolItem(it *components.ChatItem) {
	name := it.ToolName
	args := it.ToolArgs
	switch name {
	case "edit":
		var p struct {
			FilePath  string `json:"filePath"`
			OldString string `json:"oldString"`
			NewString string `json:"newString"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.OldString != "" {
			it.Diff = components.ComputeDiff(p.OldString, p.NewString)
			if p.FilePath != "" {
				it.ToolName = "edit:" + p.FilePath
			}
		}
	case "read":
		var p struct {
			FilePath string `json:"filePath"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			it.ToolName = "read:" + p.FilePath
		}
	case "write":
		var p struct {
			FilePath string `json:"filePath"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.FilePath != "" {
			it.ToolName = "write:" + p.FilePath
			it.WriteContent = p.Content
		}
	case "bash":
		var p struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Command != "" {
			cmd := p.Command
			if len(cmd) > 60 {
				cmd = cmd[:60] + "…"
			}
			it.ToolName = "bash:" + cmd
		}
	case "glob":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			it.ToolName = "glob:" + p.Pattern
		}
	case "grep":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Pattern != "" {
			it.ToolName = "grep:" + p.Pattern
		}
	case "webfetch":
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.URL != "" {
			it.ToolName = "webfetch:" + p.URL
		}
	case "list":
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &p); err == nil && p.Path != "" {
			it.ToolName = "list:" + p.Path
		}
	}
}

func messageContent(m llm.Message) string {
	switch c := m.Content.(type) {
	case string:
		return c
	case []any:
		for _, item := range c {
			if mm, ok := item.(map[string]any); ok {
				if t, ok := mm["type"].(string); ok && t == "text" {
					if text, ok := mm["text"].(string); ok {
						return text
					}
				}
			}
		}
	}
	return ""
}

func (a *App) onSessionUpdated(e session.SessionUpdated) {
	if a.deps.Sessions == nil {
		return
	}
	a.chatMu.Lock()
	active := a.session != nil && a.session.info.ID == e.SessionID
	rt := a.runtimes[e.SessionID]
	a.chatMu.Unlock()
	if !active && rt == nil {
		return
	}
	sess, err := a.deps.Sessions.Get(e.SessionID)
	if err != nil {
		return
	}
	a.chatMu.Lock()
	if rt != nil && sess.Mode != "" {
		rt.mode = agent.NormalizeMode(agent.Mode(sess.Mode))
		if rt.agent != nil {
			rt.agent.SetMode(rt.mode)
		}
	}
	if a.session != nil && a.session.info.ID == e.SessionID {
		a.session.info.Title = sess.Title
		a.session.info.Provider = sess.Provider
		a.session.info.Model = sess.Model
		a.session.info.Mode = sess.Mode
		a.session.info.ReasoningEffort = sess.ReasoningEffort
		a.session.info.Usage = sess.Usage
		if sess.Mode != "" {
			a.mode = agent.NormalizeMode(agent.Mode(sess.Mode))
			if a.agent != nil {
				a.agent.SetMode(a.mode)
			}
		}
		a.refreshHomeLocked()
	}
	a.chatMu.Unlock()
	a.requestRedraw()
}

func (a *App) onSessionDeleted(e session.SessionDeleted) {
	a.chatMu.Lock()
	active := a.session != nil && a.session.info.ID == e.SessionID
	rt := a.runtimes[e.SessionID]
	delete(a.runtimes, e.SessionID)
	for id, sid := range a.agentSessions {
		if sid == e.SessionID {
			delete(a.agentSessions, id)
		}
	}
	cancel := context.CancelFunc(nil)
	if rt != nil {
		cancel = rt.cancel
	}
	a.chatMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if !active {
		return
	}
	a.stopAttach()
	a.chatMu.Lock()
	delete(a.runtimes, e.SessionID)
	a.agent = nil
	a.agentCancel = nil
	a.agentLease = nil
	a.session = nil
	a.history = nil
	a.usage = llm.Usage{}
	a.running = false
	a.resetTranscriptsLocked()
	a.viewID = ""
	a.refreshHomeLocked()
	a.chatMu.Unlock()
	a.requestRedraw()
}
