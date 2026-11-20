package acp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/tool"
)

func TestTerminalRunnerClassifiesTimeout(t *testing.T) {
	srv := newTestServer(t)
	srv.setClientCapabilities(&ClientCapabilities{Terminal: true})

	var (
		mu     sync.Mutex
		waitID any
	)
	var client *Client
	client = NewClient(func(raw json.RawMessage) error {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
		switch req.Method {
		case "terminal/create":
			client.DeliverResponse(req.ID, json.RawMessage(`{"result":{"terminalId":"term-1"}}`))
		case "terminal/output":
			client.DeliverResponse(req.ID, json.RawMessage(`{"result":{"output":"partial output\n"}}`))
		case "terminal/wait_for_exit":
			mu.Lock()
			waitID = req.ID
			mu.Unlock()
		case "terminal/kill":
			client.DeliverResponse(req.ID, json.RawMessage(`{"result":{}}`))
			mu.Lock()
			id := waitID
			mu.Unlock()
			if id != nil {
				client.DeliverResponse(id, json.RawMessage(`{"result":{"exitCode":137}}`))
			}
		case "terminal/release":
			client.DeliverResponse(req.ID, json.RawMessage(`{"result":{}}`))
		}
		return nil
	})
	srv.client = client

	runner := &acpTerminalRunner{server: srv, sessionID: "sess-1"}
	out, handled, err := runner.RunTerminal(t.Context(), "sleep 5", 100*time.Millisecond)
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if !errors.Is(err, tool.ErrCommandTimeout) {
		t.Fatalf("err = %v, want errors.Is(tool.ErrCommandTimeout)", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want errors.Is(context.DeadlineExceeded)", err)
	}
	if !strings.Contains(out, "partial output") {
		t.Fatalf("output = %q, want partial output", out)
	}
}
