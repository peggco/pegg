package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "pegg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type scriptEnv struct {
	args  string
	stdin string
	env   string
}

func newScriptEnv(dir string) scriptEnv {
	return scriptEnv{
		args:  filepath.Join(dir, "args"),
		stdin: filepath.Join(dir, "stdin"),
		env:   filepath.Join(dir, "env"),
	}
}

func (e scriptEnv) environ() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"ARGS_FILE=" + e.args,
		"STDIN_FILE=" + e.stdin,
		"ENV_FILE=" + e.env,
		"PEGG_PLUGIN=1",
		"PLUGIN_PROTOCOL_VERSIONS=2",
		"PLUGIN_MIN_PORT=10000",
		"PLUGIN_MAX_PORT=25000",
	}
}

func (e scriptEnv) read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func serveScript(response string) string {
	return `
printf '%s\n' "$@" > "$ARGS_FILE"
printf '%s' "${PEGG_PLUGIN-<unset>}:${PLUGIN_PROTOCOL_VERSIONS-<unset>}:${PLUGIN_MIN_PORT-<unset>}:${PLUGIN_MAX_PORT-<unset>}" > "$ENV_FILE"
while IFS= read -r line; do
  printf '%s\n' "$line" >> "$STDIN_FILE"
  printf '%s\n' '` + response + `'
done
`
}

func decodeRequest(t *testing.T, line string) request {
	t.Helper()
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Fatalf("decode request %q: %v", line, err)
	}
	return req
}

func TestApplyUsesPersistentServeProcess(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`{"value":"filtered","outputs":["o1"],"errors":["e1"]}`))
	client := &Client{Binary: bin, Timeout: 5 * time.Second, Env: se.environ()}
	defer client.Close()

	outcome, err := client.Apply(context.Background(), "my.hook", HookFilter, map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if string(outcome.Value) != `"filtered"` || len(outcome.Outputs) != 1 || len(outcome.Errors) != 1 {
		t.Fatalf("outcome: %#v", outcome)
	}
	args := strings.Fields(se.read(t, se.args))
	if len(args) != 2 || args[0] != "hooks" || args[1] != "serve" {
		t.Fatalf("args: %#v", args)
	}
	req := decodeRequest(t, strings.TrimSpace(se.read(t, se.stdin)))
	if req.Name != "my.hook" || req.Kind != "filter" || string(req.Value) != `{"a":"b"}` {
		t.Fatalf("request: %#v", req)
	}
	if got := se.read(t, se.env); got != "<unset>:<unset>:<unset>:<unset>" {
		t.Fatalf("plugin environment was not stripped: %q", got)
	}
}

func TestPersistentProcessIsReused(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	starts := filepath.Join(dir, "starts")
	bin := writeScript(t, dir, `
printf 'start\n' >> "`+starts+`"
while IFS= read -r line; do
  printf '%s\n' '{"value":"ok"}'
done
`)
	client := &Client{Binary: bin, Timeout: 5 * time.Second, Env: se.environ()}
	defer client.Close()

	for i := 0; i < 3; i++ {
		if _, err := client.Apply(context.Background(), "my.hook", HookFilter, i); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
	}
	lines := strings.Fields(se.read(t, starts))
	if len(lines) != 1 {
		t.Fatalf("serve process started %d times: %q", len(lines), se.read(t, starts))
	}
}

func TestFilterIntoDecodesValue(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`{"value":"changed"}`))
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	var out string
	outcome, err := client.FilterInto(context.Background(), "my.filter", "input", &out)
	if err != nil {
		t.Fatal(err)
	}
	if out != "changed" || string(outcome.Value) != `"changed"` {
		t.Fatalf("out: %q outcome: %#v", out, outcome)
	}
	req := decodeRequest(t, strings.TrimSpace(se.read(t, se.stdin)))
	if req.Kind != "filter" || req.Name != "my.filter" {
		t.Fatalf("request: %#v", req)
	}
}

func TestActionReturnsOutputs(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`{"outputs":["one","two"]}`))
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	outcome, err := client.Action(context.Background(), "my.action", map[string]bool{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Outputs) != 2 || string(outcome.Outputs[0]) != `"one"` || string(outcome.Outputs[1]) != `"two"` {
		t.Fatalf("outputs: %#v", outcome.Outputs)
	}
	req := decodeRequest(t, strings.TrimSpace(se.read(t, se.stdin)))
	if req.Kind != "action" || string(req.Value) != `{"ok":true}` {
		t.Fatalf("request: %#v", req)
	}
}

func TestApplyReturnedError(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`{"error":"no handler"}`))
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	_, err := client.Apply(context.Background(), "bad.hook", HookFilter, nil)
	if err == nil || !strings.Contains(err.Error(), "no handler") {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyMalformedOutput(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`not json`))
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	_, err := client.Apply(context.Background(), "bad.hook", HookFilter, nil)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyTimeout(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, "exec sleep 5\n")
	client := &Client{Binary: bin, Timeout: 50 * time.Millisecond, Env: se.environ()}
	defer client.Close()

	start := time.Now()
	_, err := client.Apply(context.Background(), "slow.hook", HookFilter, nil)
	if err == nil || !strings.Contains(err.Error(), "slow.hook") || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
}

func TestApplyMissingBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-pegg")
	client := &Client{Binary: missing, Env: []string{}}
	defer client.Close()

	_, err := client.Apply(context.Background(), "any.hook", HookFilter, nil)
	if err == nil || !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "any.hook") {
		t.Fatalf("error: %v", err)
	}
}

func TestApplyCommandFailureIncludesStderr(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, "echo boom >&2\nexit 3\n")
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	_, err := client.Apply(context.Background(), "fail.hook", HookAction, json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "fail.hook") {
		t.Fatalf("error: %v", err)
	}
}

func TestCloseWaitsForServeProcess(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	clean := filepath.Join(dir, "clean")
	bin := writeScript(t, dir, `
while IFS= read -r line; do
  printf '%s\n' '{"value":"ok"}'
done
printf 'clean\n' > "`+clean+`"
`)
	client := &Client{Binary: bin, Env: se.environ()}
	if _, err := client.Apply(context.Background(), "my.hook", HookFilter, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(se.read(t, clean)); got != "clean" {
		t.Fatalf("serve process was not closed gracefully: %q", got)
	}
}

func TestNestedDispatchIsRefused(t *testing.T) {
	t.Setenv("PEGG_HOOKS_SERVE", "1")
	client := &Client{Binary: filepath.Join(t.TempDir(), "missing")}
	_, err := client.Apply(context.Background(), "my.hook", HookFilter, nil)
	if !errors.Is(err, ErrNestedDispatch) {
		t.Fatalf("error = %v", err)
	}
}

func TestApplyReturnsNoHandlerSentinel(t *testing.T) {
	dir := t.TempDir()
	se := newScriptEnv(dir)
	bin := writeScript(t, dir, serveScript(`{"error":"no handler registered"}`))
	client := &Client{Binary: bin, Env: se.environ()}
	defer client.Close()

	_, err := client.Apply(context.Background(), "x", HookFilter, nil)
	if !errors.Is(err, ErrNoHandler) {
		t.Fatalf("error = %v", err)
	}
}
