package acpclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReadFrame(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("{\"a\":1}\r\n{\"b\":2}\n"))
	first, err := readFrame(r, 1024)
	if err != nil || string(first) != `{"a":1}` {
		t.Fatalf("first frame = %q, %v", first, err)
	}
	second, err := readFrame(r, 1024)
	if err != nil || string(second) != `{"b":2}` {
		t.Fatalf("second frame = %q, %v", second, err)
	}
}

func TestReadFrameTooLarge(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(strings.Repeat("x", 64) + "\n"))
	_, err := readFrame(r, 16)
	if !errors.Is(err, ErrFrameTooBig) {
		t.Fatalf("expected ErrFrameTooBig, got %v", err)
	}
}

func TestNormID(t *testing.T) {
	if got := normID(json.RawMessage(`7`)); got != "7" {
		t.Fatalf("number id = %q", got)
	}
	if got := normID(json.RawMessage(`"abc"`)); got != "abc" {
		t.Fatalf("string id = %q", got)
	}
}

func TestHandleFrameRoutesPeggUpdate(t *testing.T) {
	var gotSession string
	var gotUpdate json.RawMessage
	c := New(Options{Callbacks: Callbacks{
		OnUpdate: func(sessionID string, update json.RawMessage) {
			gotSession = sessionID
			gotUpdate = update
		},
	}})
	c.handleFrame([]byte(`{"jsonrpc":"2.0","method":"_pegg/update","params":{"sessionId":"s1","update":{"sessionUpdate":"subagent","status":"running"}}}`))
	if gotSession != "s1" {
		t.Fatalf("session = %q", gotSession)
	}
	if !strings.Contains(string(gotUpdate), `"subagent"`) {
		t.Fatalf("update = %s", gotUpdate)
	}
}

func TestHandleFrameRoutesPeggNotification(t *testing.T) {
	var gotMethod string
	c := New(Options{Callbacks: Callbacks{
		OnNotification: func(method string, params json.RawMessage) {
			gotMethod = method
		},
	}})
	c.handleFrame([]byte(`{"jsonrpc":"2.0","method":"_pegg/sessions_changed"}`))
	if gotMethod != "_pegg/sessions_changed" {
		t.Fatalf("method = %q", gotMethod)
	}
}

func TestInitializeResultPeggCapabilities(t *testing.T) {
	var res InitializeResult
	raw := `{"protocolVersion":1,"agentCapabilities":{"_meta":{"com.pegg/capabilities":{"ask":true,"terminals":true}}}}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	caps := res.Pegg()
	if caps == nil || !caps.Ask || !caps.Terminals {
		t.Fatalf("caps = %+v", caps)
	}
}
