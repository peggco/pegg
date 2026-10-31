package acpclient

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestListSessionsIntegration(t *testing.T) {
	bin := os.Getenv("PEGG_TEST_BINARY")
	if bin == "" {
		t.Skip("set PEGG_TEST_BINARY to run the ACP integration test")
	}
	client := New(Options{Binary: bin, Cwd: t.TempDir()})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "initialize", InitializeParams{ProtocolVersion: 1}); err != nil {
		t.Fatal(err)
	}
	raw, err := client.Request(ctx, "session/list", SessionListParams{All: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	var result SessionListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("session/list decode: %v", err)
	}
	t.Logf("sessions: %d", len(result.Sessions))
}
