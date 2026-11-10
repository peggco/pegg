package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/page/settings"
)

func publishTaskCall(t *testing.T, bus event.Bus, agentID, name, callID string, background bool) {
	t.Helper()
	args := fmt.Sprintf(`{"name":%q,"subagent_type":"explorer","prompt":"Map the REST API","background":%v}`, name, background)
	bus.Publish(agent.TopicAgentToolCall, agent.AgentToolCall{
		AgentID: agentID,
		Call: llm.ToolCall{
			ID:       callID,
			Function: llm.Function{Name: "task", Arguments: args},
		},
	})
}

func findTaskItem(app *App, name string) *components.ChatItem {
	app.chatMu.Lock()
	defer app.chatMu.Unlock()
	if app.main == nil {
		return nil
	}
	for _, it := range app.main.items {
		if it.Kind == components.ItemTool && it.ToolTaskName == name {
			return it
		}
	}
	return nil
}

func countSubagentItems(app *App) int {
	app.chatMu.Lock()
	defer app.chatMu.Unlock()
	if app.main == nil {
		return 0
	}
	n := 0
	for _, it := range app.main.items {
		if it.Kind == components.ItemSubagent {
			n++
		}
	}
	return n
}

func TestTaskToolCallCreatesTaskCard(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)

	it := findTaskItem(app, "explore-api")
	if it == nil {
		t.Fatal("task card not created")
	}
	if it.ToolTaskType != "explorer" || it.ToolTaskPrompt != "Map the REST API" || !it.ToolTaskBackground {
		t.Fatalf("task fields = %+v", it)
	}
	if it.ToolName != "task:explore-api" {
		t.Fatalf("tool name = %q", it.ToolName)
	}
	if it.ToolTaskStatus != "running" || it.ToolDone {
		t.Fatalf("status = %q done = %v", it.ToolTaskStatus, it.ToolDone)
	}
}

func TestTaskCardLinksSubagentStartedByDisplayName(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID:       "sub-1",
		AgentName:     "explorer",
		DisplayName:   "explore-api",
		ParentAgentID: orch.ID,
	})

	it := findTaskItem(app, "explore-api")
	if it == nil {
		t.Fatal("task card missing")
	}
	if it.ToolTaskAgentID != "sub-1" || it.ToolTaskStatus != "running" {
		t.Fatalf("link = %q status = %q", it.ToolTaskAgentID, it.ToolTaskStatus)
	}
	if n := countSubagentItems(app); n != 0 {
		t.Fatalf("linked task should not create a separate subagent card, got %d", n)
	}
}

func TestForkSubagentStillCreatesSubagentCard(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID:       "fork-1",
		AgentName:     "developer",
		DisplayName:   "forked-work",
		ParentAgentID: orch.ID,
	})

	if n := countSubagentItems(app); n != 1 {
		t.Fatalf("fork subagent should keep its own card, got %d", n)
	}
}

func TestTaskCardMirrorsSubagentActivity(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentToolCall, agent.AgentToolCall{
		AgentID: "sub-1",
		Call: llm.ToolCall{
			ID:       "read-1",
			Function: llm.Function{Name: "read", Arguments: `{"filePath":"internal/tui/chat.go"}`},
		},
	})

	it := findTaskItem(app, "explore-api")
	if !strings.Contains(it.Activity, "internal/tui/chat.go") {
		t.Fatalf("activity = %q", it.Activity)
	}
}

func TestTaskCardFinishedFromSubagent(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
		AgentID: "sub-1", AgentName: "explorer", Output: "done",
		Usage: llm.Usage{TotalTokens: 42},
	})

	it := findTaskItem(app, "explore-api")
	if !it.ToolDone || it.ToolTaskStatus != "finished" {
		t.Fatalf("done = %v status = %q", it.ToolDone, it.ToolTaskStatus)
	}
	if it.ToolOutput != "done" {
		t.Fatalf("output = %q", it.ToolOutput)
	}
	if it.ToolTaskUsage.TotalTokens != 42 {
		t.Fatalf("usage = %+v", it.ToolTaskUsage)
	}
	if it.Activity != "" {
		t.Fatalf("activity should clear on finish, got %q", it.Activity)
	}
}

func TestTaskCardErrorFromSubagent(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentError, agent.AgentError{
		AgentID: "sub-1", AgentName: "explorer", Err: fmt.Errorf("boom"),
	})

	it := findTaskItem(app, "explore-api")
	if !it.ToolDone || it.ToolTaskStatus != "error" || it.ToolErr != "boom" {
		t.Fatalf("done = %v status = %q err = %q", it.ToolDone, it.ToolTaskStatus, it.ToolErr)
	}
}

func TestBackgroundTaskCardStaysRunningAfterToolResult(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
		AgentID:  orch.ID,
		CallID:   "task-call-1",
		ToolName: "task",
		Output:   "Started subagent in background: explore-api\n",
	})

	it := findTaskItem(app, "explore-api")
	if it.ToolDone {
		t.Fatal("background task card must not finish on the tool result")
	}
	if it.ToolTaskStatus != "running" || it.ToolOutput != "" {
		t.Fatalf("status = %q output = %q", it.ToolTaskStatus, it.ToolOutput)
	}
}

func TestBackgroundTaskCardSurvivesMainFinish(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
		AgentID: orch.ID, CallID: "task-call-1", ToolName: "task",
		Output: "Started subagent in background: explore-api\n",
	})
	bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{AgentID: orch.ID, AgentName: "orch", Output: "main done"})

	it := findTaskItem(app, "explore-api")
	if it.ToolDone || it.ToolTaskStatus != "running" {
		t.Fatalf("main finish touched the task card: done = %v status = %q", it.ToolDone, it.ToolTaskStatus)
	}

	bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
		AgentID: "sub-1", AgentName: "explorer", Output: "sub done",
	})
	it = findTaskItem(app, "explore-api")
	if !it.ToolDone || it.ToolTaskStatus != "finished" || it.ToolOutput != "sub done" {
		t.Fatalf("subagent finish did not finalize the card: done = %v status = %q output = %q", it.ToolDone, it.ToolTaskStatus, it.ToolOutput)
	}
}

func TestForegroundTaskCardFinishesFromSubagent(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", false)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})
	bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
		AgentID: orch.ID, CallID: "task-call-1", ToolName: "task",
		Output: "Subagent \"explore-api\" finished.\nResponse:\nanswer\n",
	})
	bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
		AgentID: "sub-1", AgentName: "explorer", Output: "answer",
	})

	it := findTaskItem(app, "explore-api")
	if !it.ToolDone || it.ToolTaskStatus != "finished" {
		t.Fatalf("done = %v status = %q", it.ToolDone, it.ToolTaskStatus)
	}
	if it.ToolOutput != "answer" {
		t.Fatalf("output = %q, want the clean subagent response", it.ToolOutput)
	}
}

func TestTaskCardHistoryOpensWhileRunning(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", true)
	bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", DisplayName: "explore-api", ParentAgentID: orch.ID,
	})

	it := findTaskItem(app, "explore-api")
	app.activateItem(it)

	app.chatMu.Lock()
	viewID := app.viewID
	app.chatMu.Unlock()
	if viewID != "sub-1" {
		t.Fatalf("viewID = %q, want sub-1", viewID)
	}
}

func TestTaskCardFinishedClickExpands(t *testing.T) {
	orch := agent.New("orch", agent.WithProvider(&chatEchoProvider{}), agent.WithModel(llm.Model{ID: "m"}))
	app, bus := newChatApp(t, orch)

	publishTaskCall(t, bus, orch.ID, "explore-api", "task-call-1", false)
	bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
		AgentID: orch.ID, CallID: "task-call-1", ToolName: "task",
		Output: "Subagent \"explore-api\" finished.\nResponse:\nanswer\n",
	})

	it := findTaskItem(app, "explore-api")
	app.activateItem(it)
	if !it.Expanded {
		t.Fatal("finished task card should expand on activation")
	}
}

func restoredTaskItem(t *testing.T, msgs []session.Message) *components.ChatItem {
	t.Helper()
	items := messagesToItems(msgs)
	for _, it := range items {
		if it.Kind == components.ItemTool {
			return it
		}
	}
	t.Fatal("no task item restored")
	return nil
}

func taskCallMessage(background bool) session.Message {
	return session.Message{
		ID: "m1", Seq: 1, Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID: "tc-1",
			Function: llm.Function{
				Name:      "task",
				Arguments: fmt.Sprintf(`{"name":"explore-api","subagent_type":"explorer","prompt":"Map the REST API","background":%v}`, background),
			},
		}},
	}
}

func TestMessagesToItemsRestoresForegroundTaskCard(t *testing.T) {
	it := restoredTaskItem(t, []session.Message{
		taskCallMessage(false),
		{ID: "m2", Seq: 2, Role: llm.RoleTool, ToolCallID: "tc-1", Content: "Subagent \"explore-api\" finished.\nResponse:\nanswer\n"},
	})
	if it.ToolTaskStatus != "finished" || it.ToolOutput != "answer" {
		t.Fatalf("status = %q output = %q", it.ToolTaskStatus, it.ToolOutput)
	}
	if !it.ToolDone || it.ToolTaskAgentID != "" {
		t.Fatalf("done = %v agent = %q", it.ToolDone, it.ToolTaskAgentID)
	}
}

func TestMessagesToItemsRestoresBackgroundTaskCard(t *testing.T) {
	it := restoredTaskItem(t, []session.Message{
		taskCallMessage(true),
		{ID: "m2", Seq: 2, Role: llm.RoleTool, ToolCallID: "tc-1", Content: "Started subagent in background: explore-api\n"},
	})
	if it.ToolTaskStatus != "background" || it.ToolOutput != "" {
		t.Fatalf("status = %q output = %q", it.ToolTaskStatus, it.ToolOutput)
	}
	if !it.ToolDone || it.ToolTaskAgentID != "" {
		t.Fatalf("done = %v agent = %q", it.ToolDone, it.ToolTaskAgentID)
	}
}

func TestMessagesToItemsRestoresFailedTaskCard(t *testing.T) {
	it := restoredTaskItem(t, []session.Message{
		taskCallMessage(false),
		{ID: "m2", Seq: 2, Role: llm.RoleTool, ToolCallID: "tc-1", Content: "Subagent \"explore-api\" failed: boom\n"},
	})
	if it.ToolTaskStatus != "error" || it.ToolErr != "boom" {
		t.Fatalf("status = %q err = %q", it.ToolTaskStatus, it.ToolErr)
	}
}

func TestMessagesToItemsRestoresInterruptedTaskCard(t *testing.T) {
	foreground := restoredTaskItem(t, []session.Message{taskCallMessage(false)})
	if foreground.ToolTaskStatus != "unknown" {
		t.Fatalf("foreground status = %q", foreground.ToolTaskStatus)
	}

	background := restoredTaskItem(t, []session.Message{taskCallMessage(true)})
	if background.ToolTaskStatus != "background" {
		t.Fatalf("background status = %q", background.ToolTaskStatus)
	}
}

func TestMergeLiveTaskItemsKeepsRunningLink(t *testing.T) {
	live := &components.ChatItem{
		Kind: components.ItemTool, ID: "call-1", ToolName: "task:bg-task",
		ToolTaskName: "bg-task", ToolTaskAgentID: "sub-1", ToolTaskStatus: "running",
	}
	restored := &components.ChatItem{
		Kind: components.ItemTool, ID: "call-1", ToolName: "task:bg-task",
		ToolTaskName: "bg-task", ToolTaskStatus: "background", ToolDone: true,
	}
	app := &App{
		main:        &agentTranscript{id: "main", items: []*components.ChatItem{live}},
		taskByAgent: map[string]*taskLink{"sub-1": {item: live, transcriptID: "main"}},
	}
	merged := app.mergeLiveTaskItemsLocked([]*components.ChatItem{restored})
	if len(merged) != 1 || merged[0] != live {
		t.Fatalf("live task card was not preserved: %+v", merged)
	}

	live.ToolDone = true
	merged = app.mergeLiveTaskItemsLocked([]*components.ChatItem{restored})
	if len(merged) != 1 || merged[0] != restored {
		t.Fatalf("finished task card should prefer the restored item: %+v", merged)
	}
}

func TestResetTranscriptsClearsTaskLinks(t *testing.T) {
	app := &App{
		chat:        components.NewChat(),
		subs:        map[string]*agentTranscript{},
		subItemByID: map[string]*components.ChatItem{},
		taskByAgent: map[string]*taskLink{"sub-1": {}},
	}
	app.resetTranscriptsLocked()
	if len(app.taskByAgent) != 0 {
		t.Fatalf("taskByAgent was not cleared: %v", app.taskByAgent)
	}
}

func TestLoadSessionKeepsLiveTaskLink(t *testing.T) {
	app, mgr := newSessionApp(t)
	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	args := `{"name":"bg-task","subagent_type":"explorer","prompt":"do it","background":true}`
	if _, err := mgr.AppendMessage(sess.ID, llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID:       "call-1",
			Function: llm.Function{Name: "task", Arguments: args},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.AppendMessage(sess.ID, llm.ToolMessage("Started subagent in background: bg-task\n", "call-1")); err != nil {
		t.Fatal(err)
	}
	msgs, err := mgr.Messages(sess.ID)
	if err != nil {
		t.Fatal(err)
	}

	orch := agent.New("orchestrator")
	live := &components.ChatItem{
		Kind: components.ItemTool, ID: "call-1", ToolName: "task:bg-task",
		ToolTaskName: "bg-task", ToolTaskBackground: true,
		ToolTaskAgentID: "sub-1", ToolTaskStatus: "running",
	}
	app.chatMu.Lock()
	app.agent = orch
	app.session = &activeSession{info: settings.SessionInfo{ID: sess.ID, Messages: msgs}, viewID: sess.ID}
	app.main = &agentTranscript{
		id: orch.ID, name: "orchestrator",
		items:    []*components.ChatItem{live},
		toolByID: map[string]*components.ChatItem{"call-1": live},
	}
	app.viewID = orch.ID
	app.taskByAgent = map[string]*taskLink{"sub-1": {item: live, transcriptID: orch.ID}}
	app.subs = map[string]*agentTranscript{"sub-1": {id: "sub-1", name: "explorer", toolByID: map[string]*components.ChatItem{}}}
	app.loadSessionIntoChatLocked()
	app.chatMu.Unlock()

	if app.main.toolByID["call-1"] != live {
		t.Fatal("task call was not re-registered in toolByID")
	}
	found := false
	for _, it := range app.main.items {
		if it == live {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("live task card was replaced by the restored card")
	}
	link := app.taskByAgent["sub-1"]
	if link == nil || link.item != live {
		t.Fatalf("live task link = %+v", link)
	}
}

func TestMessagesToItemsRestoresBackgroundTaskWithResult(t *testing.T) {
	it := restoredTaskItem(t, []session.Message{
		taskCallMessage(true),
		{ID: "m2", Seq: 2, Role: llm.RoleTool, ToolCallID: "tc-1", Content: "final answer"},
	})
	if it.ToolTaskStatus != "finished" || it.ToolOutput != "final answer" {
		t.Fatalf("status = %q output = %q", it.ToolTaskStatus, it.ToolOutput)
	}
	if !it.ToolTaskBackground {
		t.Fatal("background badge should be preserved")
	}
}

func TestMessagesToItemsRestoresTruncatedBackgroundStart(t *testing.T) {
	it := restoredTaskItem(t, []session.Message{
		taskCallMessage(true),
		{ID: "m2", Seq: 2, Role: llm.RoleTool, ToolCallID: "tc-1", Content: "Started subagent in backgro…"},
	})
	if it.ToolTaskStatus != "background" || it.ToolOutput != "" {
		t.Fatalf("status = %q output = %q", it.ToolTaskStatus, it.ToolOutput)
	}
}
