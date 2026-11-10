package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/reminder"
	"github.com/peggco/pegg/internal/llm"
)

func requestText(req *llm.Request) string {
	if req == nil {
		return ""
	}
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(fmt.Sprint(m.Content))
		b.WriteByte('\n')
	}
	return b.String()
}

func TestSubAgentNotificationWakesIdleMain(t *testing.T) {
	prov := &chatEchoProvider{chunks: []llm.StreamChunk{{Content: "woke"}}}
	orch := agent.New("orch", agent.WithProvider(prov), agent.WithModel(llm.Model{ID: "m"}))
	app, _ := newChatApp(t, orch)

	rt := &sessionRuntime{agent: orch, mode: agent.ModeAuto}
	app.chatMu.Lock()
	app.runtimes = map[string]*sessionRuntime{"": rt}
	app.chatMu.Unlock()

	orch.QueueNotification(reminder.New("subagent", "background work done", "agent", "bg-1"))
	app.onSubAgentNotification(agent.SubAgentNotification{
		ParentAgentID: orch.ID,
		SubAgentName:  "bg-1",
		Output:        "done",
	})

	waitFor(t, 5*time.Second, func() bool { return prov.lastReq != nil })
	if text := requestText(prov.lastReq); !strings.Contains(text, "background work done") {
		t.Fatalf("wake request is missing the system reminder:\n%s", text)
	}
}

func TestFinishAgentRunWakesForPendingSubAgentNotification(t *testing.T) {
	prov := &chatEchoProvider{chunks: []llm.StreamChunk{{Content: "woke"}}}
	orch := agent.New("orch", agent.WithProvider(prov), agent.WithModel(llm.Model{ID: "m"}))
	app, _ := newChatApp(t, orch)

	rt := &sessionRuntime{agent: orch, mode: agent.ModeAuto}
	app.chatMu.Lock()
	app.runtimes = map[string]*sessionRuntime{"": rt}
	app.chatMu.Unlock()

	orch.QueueNotification(reminder.New("subagent", "late finish", "agent", "bg-2"))
	app.finishAgentRun(rt, nil, func() {})

	waitFor(t, 5*time.Second, func() bool { return prov.lastReq != nil })
	if text := requestText(prov.lastReq); !strings.Contains(text, "late finish") {
		t.Fatalf("finish wake request is missing the system reminder:\n%s", text)
	}
}
