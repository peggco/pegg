package acp

import (
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
)

func collectUpdates(t *testing.T, transport *bufferTransport) []SessionUpdate {
	t.Helper()
	transport.mu.Lock()
	defer transport.mu.Unlock()
	out := make([]SessionUpdate, 0, len(transport.sends))
	for _, raw := range transport.sends {
		var envelope struct {
			Params struct {
				Update SessionUpdate `json:"update"`
			} `json:"params"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			t.Fatalf("decode notification: %v", err)
		}
		out = append(out, envelope.Params.Update)
	}
	return out
}

func TestSubagentLifecycleNotifications(t *testing.T) {
	srv := newTestServer(t)
	transport := newBufferTransport()
	srv.transport = transport

	orch, err := srv.newAgentFn()
	if err != nil {
		t.Fatal(err)
	}
	srv.addSession(&ACPSession{ID: "acp-1", Agent: orch, Cwd: "/tmp", PeggSessionID: "pegg-1"})

	srv.bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", ParentAgentID: orch.ID,
	})
	srv.bus.Publish(agent.TopicAgentInput, agent.AgentInput{
		AgentID: "sub-1", AgentName: "explorer", Input: "find the auth code",
	})
	srv.bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
		AgentID: "sub-1", AgentName: "explorer", Output: "found it",
	})

	updates := collectUpdates(t, transport)
	if len(updates) != 3 {
		t.Fatalf("updates = %d, want 3: %+v", len(updates), updates)
	}
	for _, update := range updates {
		if update.SessionUpdate != "subagent" {
			t.Fatalf("update type = %q", update.SessionUpdate)
		}
		if update.AgentID != "sub-1" || update.AgentName != "explorer" {
			t.Fatalf("update = %+v", update)
		}
	}
	if updates[0].Status != "running" {
		t.Fatalf("started status = %q", updates[0].Status)
	}
	if updates[1].Title != "find the auth code" || updates[1].Status != "running" {
		t.Fatalf("input update = %+v", updates[1])
	}
	if updates[2].Status != "finished" || updates[2].RawOutput != "found it" {
		t.Fatalf("finished update = %+v", updates[2])
	}
}

func TestSubagentErrorNotification(t *testing.T) {
	srv := newTestServer(t)
	transport := newBufferTransport()
	srv.transport = transport

	orch, err := srv.newAgentFn()
	if err != nil {
		t.Fatal(err)
	}
	srv.addSession(&ACPSession{ID: "acp-1", Agent: orch, Cwd: "/tmp", PeggSessionID: "pegg-1"})

	srv.bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "planner", ParentAgentID: orch.ID,
	})
	srv.bus.Publish(agent.TopicAgentError, agent.AgentError{
		AgentID: "sub-1", AgentName: "planner", Err: errTestSubagent,
	})

	updates := collectUpdates(t, transport)
	last := updates[len(updates)-1]
	if last.Status != "error" || last.Error != errTestSubagent.Error() {
		t.Fatalf("error update = %+v", last)
	}
}

func TestOrchestratorIsNotASubagent(t *testing.T) {
	srv := newTestServer(t)
	transport := newBufferTransport()
	srv.transport = transport

	orch, err := srv.newAgentFn()
	if err != nil {
		t.Fatal(err)
	}
	srv.addSession(&ACPSession{ID: "acp-1", Agent: orch, Cwd: "/tmp", PeggSessionID: "pegg-1"})

	srv.bus.Publish(agent.TopicAgentFinished, agent.AgentFinished{
		AgentID: orch.ID, AgentName: "orchestrator", Output: "done",
	})

	if updates := collectUpdates(t, transport); len(updates) != 0 {
		t.Fatalf("orchestrator must not emit subagent updates: %+v", updates)
	}
}

var errTestSubagent = &subagentTestError{}

type subagentTestError struct{}

func (e *subagentTestError) Error() string { return "subagent failed: boom" }

func TestSubagentStreamIsForwarded(t *testing.T) {
	srv := newTestServer(t)
	transport := newBufferTransport()
	srv.transport = transport

	orch, err := srv.newAgentFn()
	if err != nil {
		t.Fatal(err)
	}
	srv.addSession(&ACPSession{ID: "acp-1", Agent: orch, Cwd: "/tmp", PeggSessionID: "pegg-1"})
	srv.bus.Publish(agent.TopicAgentStarted, agent.AgentStarted{
		AgentID: "sub-1", AgentName: "explorer", ParentAgentID: orch.ID,
	})

	srv.bus.Publish(agent.TopicAgentMessage, agent.AgentMessage{
		AgentID: "sub-1", AgentName: "explorer", Message: llm.AssistantMessage("did the thing"),
	})
	srv.bus.Publish(agent.TopicAgentToolCall, agent.AgentToolCall{
		AgentID: "sub-1", AgentName: "explorer",
		Call: llm.ToolCall{ID: "c1", Type: "function", Function: llm.Function{Name: "read", Arguments: `{"filePath":"a.ts"}`}},
	})
	srv.bus.Publish(agent.TopicAgentToolResult, agent.AgentToolResult{
		AgentID: "sub-1", AgentName: "explorer", CallID: "c1", ToolName: "read", Output: "file contents",
	})

	srv.bus.Publish(agent.TopicAgentMessage, agent.AgentMessage{
		AgentID: orch.ID, AgentName: "orchestrator", Message: llm.AssistantMessage("main answer"),
	})

	var sawMessage, sawToolCall, sawToolResult, sawMain bool
	for _, update := range collectUpdates(t, transport) {
		switch update.SessionUpdate {
		case "agent_message_chunk":
			if update.AgentID == "sub-1" {
				sawMessage = true
				if update.MessageID == "" {
					t.Fatal("the subagent message needs an id")
				}
			}
			if update.AgentID == orch.ID {
				sawMain = true
			}
		case "tool_call":
			if update.AgentID == "sub-1" && update.ToolName == "read" && update.ToolCallID == "c1" {
				sawToolCall = true
			}
		case "tool_call_update":
			if update.AgentID == "sub-1" && update.RawOutput == "file contents" {
				sawToolResult = true
			}
		}
	}
	if !sawMessage {
		t.Fatal("the subagent message was not forwarded")
	}
	if !sawToolCall {
		t.Fatal("the subagent tool call was not forwarded")
	}
	if !sawToolResult {
		t.Fatal("the subagent tool result was not forwarded")
	}
	if sawMain {
		t.Fatal("the orchestrator message must not be forwarded twice")
	}
}
