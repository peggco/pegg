package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/plugin/bundle"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/vfs"
)

func TestBrowserUsePluginIntegration(t *testing.T) {
	if os.Getenv("PEGG_PLUGIN_E2E") != "1" {
		t.Skip("set PEGG_PLUGIN_E2E=1 to load browser-use from ~/.pegg/plugins")
	}
	dir, err := GetPluginDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pegg-browser-use")); err != nil {
		t.Skip("pegg-browser-use is not installed in ~/.pegg/plugins")
	}
	t.Setenv("PEGG_BROWSER_PROFILE", t.TempDir())
	workspace := t.TempDir()
	fs, err := vfs.New(workspace, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, fs, nil, nil)
	if err := mgr.LoadPlugins(); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	p, ok := mgr.GetPlugin("browser-use")
	if !ok {
		t.Fatalf("browser-use not loaded")
	}
	if len(p.Tools) != 19 {
		t.Fatalf("expected 19 tools, got %d", len(p.Tools))
	}
	if len(p.Skills) != 1 || p.Skills[0].Name != "browser-use" {
		t.Fatalf("unexpected skills: %#v", p.Skills)
	}
	if len(mgr.Commands()) == 0 {
		t.Fatal("expected plugin commands")
	}
	if !contains(bundle.Names(), "browser") {
		t.Fatalf("browser bundle not registered: %v", bundle.Names())
	}
	if s, ok := skill.Get("browser-use"); !ok || len(s.ToolBundles) != 1 || s.ToolBundles[0] != "browser" {
		t.Fatalf("browser-use skill missing or invalid: %#v", s)
	}
	if got := mgr.cfg.Permission.Rules["browser_click"]; got != "semi-judge" {
		t.Fatalf("plugin permission rule not merged: %q", got)
	}

	a := agent.New("test")
	ctx := agent.WithAgent(context.Background(), a)
	status := bundle.Activate(ctx, bundle.Deps{FS: fs}, []string{"browser"})
	if len(status) == 0 {
		t.Fatal("no activation status")
	}
	if _, ok := a.Tools.Get("browser_navigate"); !ok {
		t.Fatalf("browser tools not activated: %s", strings.Join(a.Tools.Names(), ","))
	}

	execCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := mgr.ExecuteTool(execCtx, "browser-use", "browser_status", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "browser-use") {
		t.Fatalf("unexpected status: %s", res.Output)
	}

	page := `data:text/html,<html><head><title>HostTest</title></head><body><input id="n"><button id="go" onclick="document.querySelector('h1').textContent=document.getElementById('n').value">Go</button><h1>Start</h1></body></html>`
	args, _ := json.Marshal(map[string]string{"url": page})
	if _, err := mgr.ExecuteTool(execCtx, "browser-use", "browser_navigate", string(args)); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.ExecuteTool(execCtx, "browser-use", "browser_type", `{"selector":"#n","text":"Alice"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.ExecuteTool(execCtx, "browser-use", "browser_click", `{"selector":"#go"}`); err != nil {
		t.Fatal(err)
	}
	res, err = mgr.ExecuteTool(execCtx, "browser-use", "browser_snapshot", `{"mode":"text"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "Alice") {
		t.Fatalf("page did not update: %s", res.Output)
	}
	screenshotTool, ok := a.Tools.Get("browser_screenshot")
	if !ok {
		t.Fatal("screenshot tool missing")
	}
	shot, err := screenshotTool.Execute(execCtx, `{"format":"png"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shot, "Saved:") {
		t.Fatalf("screenshot was not saved: %s", shot)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".pegg", "artifacts", "plugins", "browser-use")); err != nil {
		t.Fatalf("artifact directory missing: %v", err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
