package llm

import (
	"context"
	"testing"
)

func TestSessionIDContext(t *testing.T) {
	ctx := WithSessionID(context.Background(), "sess-1")
	if got := SessionIDFrom(ctx); got != "sess-1" {
		t.Fatalf("SessionIDFrom = %q, want sess-1", got)
	}
	if got := SessionIDFrom(context.Background()); got != "" {
		t.Fatalf("SessionIDFrom(empty) = %q, want empty", got)
	}
}

func TestSessionHeaderFunc(t *testing.T) {
	ctx := WithSessionID(context.Background(), "sess-1")
	headers := SessionHeaderFunc([]string{"session_id", "x-opencode-session"})(ctx)
	if headers[SessionHeader] != "sess-1" {
		t.Fatalf("%s = %q, want sess-1", SessionHeader, headers[SessionHeader])
	}
	if headers["session_id"] != "sess-1" {
		t.Fatalf("session_id = %q, want sess-1", headers["session_id"])
	}
	if headers["x-opencode-session"] != "sess-1" {
		t.Fatalf("x-opencode-session = %q, want sess-1", headers["x-opencode-session"])
	}

	if got := SessionHeaderFunc(nil)(context.Background()); got != nil {
		t.Fatalf("headers without a session = %v, want nil", got)
	}
}
