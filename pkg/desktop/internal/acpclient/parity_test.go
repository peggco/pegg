package acpclient

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type parityHook struct {
	Plugin   string `json:"plugin"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Priority int    `json:"priority"`
}

type parityPluginsResult struct {
	Plugins []struct {
		Name   string `json:"name"`
		Loaded bool   `json:"loaded"`
		Error  string `json:"error"`
		Hooks  []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"hooks"`
	} `json:"plugins"`
	Hooks []parityHook `json:"hooks"`
}

func parityKey(hook parityHook) string {
	return hook.Plugin + "|" + hook.Name + "|" + hook.Kind + "|" + strconv.Itoa(hook.Priority)
}

func hookKeys(hooks []parityHook) []string {
	keys := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		keys = append(keys, parityKey(hook))
	}
	sort.Strings(keys)
	return keys
}

func TestPluginHookParityIntegration(t *testing.T) {
	bin := os.Getenv("PEGG_TEST_BINARY")
	if bin == "" {
		t.Skip("set PEGG_TEST_BINARY to run the ACP plugin parity test")
	}
	if os.Getenv("PEGG_PLUGIN_E2E") != "1" {
		t.Skip("set PEGG_PLUGIN_E2E=1 to compare installed plugins")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	inProcess, err := inProcessHooks(bin, cwd)
	if err != nil {
		t.Fatalf("in-process hooks: %v", err)
	}
	if len(inProcess) == 0 {
		t.Skip("no plugin hooks registered; install kanban and kanban-github first")
	}

	client := New(Options{Binary: bin, Cwd: cwd})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "initialize", InitializeParams{ProtocolVersion: 1}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	raw, err := client.Request(ctx, "_pegg/plugins", struct{}{})
	if err != nil {
		t.Fatalf("_pegg/plugins: %v", err)
	}
	var result parityPluginsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode _pegg/plugins: %v", err)
	}

	for _, plugin := range result.Plugins {
		if plugin.Error != "" {
			t.Errorf("plugin %s failed to load in the ACP child: %s", plugin.Name, plugin.Error)
		}
	}

	want := hookKeys(inProcess)
	got := hookKeys(result.Hooks)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Fatalf("hook parity mismatch\nin-process:\n%s\nacp child:\n%s",
			strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func inProcessHooks(bin, dir string) ([]parityHook, error) {
	cmd := exec.Command(bin, "hooks", "list", "--json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var hooks []parityHook
	if err := json.Unmarshal(out, &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}
