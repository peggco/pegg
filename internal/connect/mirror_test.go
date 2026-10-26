package connect

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func newMirrorTestConnector(t *testing.T) (*Connector, *connState) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	c, err := New(Deps{Log: testLogger(), Version: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	cs := &connState{out: make(chan *Frame, 32), done: make(chan struct{})}
	c.cur.Store(cs)
	c.runs.register(&run{
		id: "r1", sessionID: "s1", agentID: "main", cancel: func() {},
	})
	return c, cs
}

func journalEvent(t *testing.T, typ, agentID, parentID string, data any) session.RunEvent {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return session.RunEvent{
		RunID:         "r1",
		SessionID:     "s1",
		Seq:           1,
		Type:          typ,
		AgentID:       agentID,
		ParentAgentID: parentID,
		Data:          payload,
		CreatedAt:     time.Now(),
	}
}

func nextFrame(t *testing.T, cs *connState) *Frame {
	t.Helper()
	select {
	case frame := <-cs.out:
		return frame
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for frame")
	}
	return nil
}

func expectNoFrame(t *testing.T, cs *connState) {
	t.Helper()
	select {
	case frame := <-cs.out:
		t.Fatalf("unexpected frame: %s", frame.Event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestForwardLiveSubagentEvents(t *testing.T) {
	c, cs := newMirrorTestConnector(t)
	run := session.Run{ID: "r1", SessionID: "s1", AgentID: "main"}

	started := journalEvent(t, session.AgentEventStarted, "sub1", "main", map[string]any{
		"agent_name":      "explorer",
		"display_name":    "auth flow",
		"parent_agent_id": "main",
	})
	started.Seq = 2
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: started, Local: true})
	frame := nextFrame(t, cs)
	if frame.Event != eventRunSubagentStarted || frame.RunID != "r1" || frame.SessionID != "s1" {
		t.Fatalf("started frame = %+v", frame)
	}
	var startedPayload runSubagentStartedPayload
	if err := json.Unmarshal(frame.Data, &startedPayload); err != nil {
		t.Fatal(err)
	}
	if startedPayload.AgentID != "sub1" || startedPayload.DisplayName != "auth flow" {
		t.Fatalf("started payload = %+v", startedPayload)
	}

	input := journalEvent(t, session.AgentEventInput, "sub1", "main", map[string]any{"input": "inspect auth"})
	input.Seq = 3
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: input, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentMessage {
		t.Fatalf("input frame event = %s", frame.Event)
	}
	var messagePayload runSubagentMessagePayload
	if err := json.Unmarshal(frame.Data, &messagePayload); err != nil {
		t.Fatal(err)
	}
	if messagePayload.AgentID != "sub1" || messagePayload.Message.Role != string(llm.RoleUser) {
		t.Fatalf("input payload = %+v", messagePayload)
	}

	message := journalEvent(t, session.AgentEventMessage, "sub1", "main", map[string]any{
		"message": llm.AssistantMessage("found it"),
	})
	message.Seq = 4
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: message, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentMessage {
		t.Fatalf("message frame event = %s", frame.Event)
	}

	result := journalEvent(t, session.AgentEventToolResult, "sub1", "main", map[string]any{
		"call_id": "c1", "tool_name": "read", "output": "ok",
	})
	result.Seq = 5
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: result, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentMessage {
		t.Fatalf("result frame event = %s", frame.Event)
	}
	if err := json.Unmarshal(frame.Data, &messagePayload); err != nil {
		t.Fatal(err)
	}
	if messagePayload.Message.Role != string(llm.RoleTool) || messagePayload.Message.ToolCallID != "c1" {
		t.Fatalf("tool payload = %+v", messagePayload)
	}

	usage := journalEvent(t, session.AgentEventUsage, "sub1", "main", map[string]any{
		"usage": llm.Usage{TotalTokens: 12},
	})
	usage.Seq = 6
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: usage, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentUsage {
		t.Fatalf("usage frame event = %s", frame.Event)
	}

	done := journalEvent(t, session.AgentEventFinished, "sub1", "main", map[string]any{
		"output": "done", "usage": llm.Usage{TotalTokens: 14},
	})
	done.Seq = 7
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: done, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentDone {
		t.Fatalf("done frame event = %s", frame.Event)
	}

	failed := journalEvent(t, session.AgentEventError, "sub1", "main", map[string]any{"message": "boom"})
	failed.Seq = 8
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: failed, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentError {
		t.Fatalf("error frame event = %s", frame.Event)
	}
}

func TestForwardLiveSkipsLocalMainButForwardsRemoteMain(t *testing.T) {
	c, cs := newMirrorTestConnector(t)
	run := session.Run{ID: "r1", SessionID: "s1", AgentID: "main"}

	token := journalEvent(t, session.AgentEventToken, "main", "", map[string]any{"content": "hi"})
	token.Seq = 2
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: token, Local: true})
	expectNoFrame(t, cs)

	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: token, Local: false})
	frame := nextFrame(t, cs)
	if frame.Event != eventRunToken {
		t.Fatalf("remote token frame event = %s", frame.Event)
	}

	started := journalEvent(t, session.AgentEventStarted, "sub1", "main", map[string]any{"agent_name": "explorer"})
	started.Seq = 3
	c.forwardLive(live.Event{Type: live.EventRunEvent, Run: run, Journal: started, Local: true})
	frame = nextFrame(t, cs)
	if frame.Event != eventRunSubagentStarted {
		t.Fatalf("local subagent frame event = %s", frame.Event)
	}
}

func TestForwardLiveSkipsLocalRunLifecycle(t *testing.T) {
	c, cs := newMirrorTestConnector(t)
	run := session.Run{ID: "r1", SessionID: "s1", AgentID: "main"}

	c.forwardLive(live.Event{Type: live.EventRunStarted, Run: run, Local: true})
	expectNoFrame(t, cs)

	c.forwardLive(live.Event{Type: live.EventRunFinished, Run: run, Local: true})
	expectNoFrame(t, cs)

	c.forwardLive(live.Event{Type: live.EventRunStarted, Run: run, Local: false})
	if frame := nextFrame(t, cs); frame.Event != eventRunStarted {
		t.Fatalf("remote start frame event = %s", frame.Event)
	}

	c.forwardLive(live.Event{Type: live.EventRunFinished, Run: run, Local: false, State: session.RunStateDone})
	if frame := nextFrame(t, cs); frame.Event != eventRunDone {
		t.Fatalf("remote finish frame event = %s", frame.Event)
	}
}
