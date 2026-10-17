package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/lsp/diagnostic"
	"github.com/peggco/pegg/internal/vfs"
)

func TestManagerLazyStartAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(fs, nil)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})
	defer mgr.Close()

	res, err := fs.Read("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "fake error") {
		t.Fatalf("read must include diagnostics from the lazily started server, got %q", res)
	}

	diags := mgr.Diagnostics("main.go")
	if len(diags) == 0 || diags[0].Message != "fake error" {
		t.Fatalf("diags = %+v", diags)
	}

	res2, err := fs.Read("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res2, "fake error") {
		t.Fatalf("res2 must contain diagnostics 'fake error', got %q", res2)
	}
}

func waitDiagnosticsCount(t *testing.T, mgr *Manager, path string, want int) []diagnostic.Diagnostic {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		diags := mgr.Diagnostics(path)
		if len(diags) == want {
			return diags
		}
		select {
		case <-deadline:
			t.Fatalf("diags for %q = %+v, want %d", path, diags, want)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func TestWriteDiagnosticsAreVersionCorrelated(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n// fake error\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}

	tr := newScriptedTransport()
	mgr := NewManager(fs, nil)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) { return tr, nil })
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	waitDiagnosticsCount(t, mgr, "main.go", 1)

	tr.setOnChange(func(version int, text string) []map[string]any {
		if version < 2 {
			t.Errorf("didChange version = %d, want >= 2", version)
		}
		return nil
	})

	start := time.Now()
	out, err := fs.Edit("main.go", "// fake error", "// fixed", false)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > mgr.diagWait()+time.Second {
		t.Fatalf("write took %s, expected it to return after diagnostics", elapsed)
	}
	if strings.Contains(out, "fake error") {
		t.Fatalf("write returned stale diagnostics: %q", out)
	}
	waitDiagnosticsCount(t, mgr, "main.go", 0)

	uri := mgr.fileURI("main.go")
	order := tr.methodOrder()
	openIdx, changeIdx := -1, -1
	for i, entry := range order {
		if entry == MethodDidOpen+" "+uri && openIdx == -1 {
			openIdx = i
		}
		if entry == MethodDidChange+" "+uri && changeIdx == -1 {
			changeIdx = i
		}
	}
	if openIdx == -1 || changeIdx == -1 {
		t.Fatalf("missing didOpen/didChange for %s: %v", uri, order)
	}
	if openIdx > changeIdx {
		t.Fatalf("didOpen must precede didChange, order=%v", order)
	}
}

func TestWriteWithoutMatchingServerDoesNotStall(t *testing.T) {
	root := t.TempDir()
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	tr := newScriptedTransport()
	mgr := NewManager(fs, nil)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) { return tr, nil })
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}},
	})

	start := time.Now()
	if _, err := fs.Write("notes.txt", []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("write with no matching server took %s", elapsed)
	}
}

func TestManagerCloseStopsServers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, _ := vfs.New(root, vfs.Options{})

	mgr := NewManager(fs, nil)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		if len(mgr.Running()) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("server never started")
		case <-time.After(50 * time.Millisecond):
		}
	}

	if err := mgr.Close(); err != nil {
		t.Fatal(err)
	}
	if len(mgr.Running()) != 0 {
		t.Fatalf("running after close = %v", mgr.Running())
	}
}

func TestServerMatches(t *testing.T) {
	srv := NewServer("gopls", config.LanguageServerConfig{FileTypes: []string{"go"}})
	if !srv.Matches("src/main.go") {
		t.Fatal("go file must match")
	}
	if srv.Matches("main.py") {
		t.Fatal("py file must not match go server")
	}
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, _ := vfs.New(root, vfs.Options{})
	mgr := NewManager(fs, nil)

	found, err := mgr.findRoot("src/nested/a.go", []string{"go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(found) != filepath.Clean(root) {
		t.Fatalf("root = %q, want %q", found, root)
	}

	found, _ = mgr.findRoot("src/nested/a.go", []string{"missing-marker"})
	if filepath.Clean(found) != filepath.Clean(root) {
		t.Fatalf("fallback root = %q, want %q", found, root)
	}
}

func waitRunning(t *testing.T, mgr *Manager, expect int) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		if len(mgr.Running()) == expect {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("running servers = %v, want %d", mgr.Running(), expect)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestIdleServerAutoClose(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, _ := vfs.New(root, vfs.Options{})

	mgr := NewManager(fs, nil)
	mgr.SetIdleTimeout(200 * time.Millisecond)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, mgr, 1)

	deadline := time.After(10 * time.Second)
	for {
		if len(mgr.Running()) == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("idle server was never closed")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestIdleServerRestartsOnTouch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, _ := vfs.New(root, vfs.Options{})

	mgr := NewManager(fs, nil)
	mgr.SetIdleTimeout(200 * time.Millisecond)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, mgr, 1)

	deadline := time.After(10 * time.Second)
	for {
		if len(mgr.Running()) == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("idle server was never closed")
		case <-time.After(50 * time.Millisecond):
		}
	}

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, mgr, 1)

	var diags []diagnostic.Diagnostic
	deadline = time.After(5 * time.Second)
	for {
		diags = mgr.Diagnostics("main.go")
		if len(diags) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no diagnostics after restart")
		case <-time.After(50 * time.Millisecond):
		}
	}
	if diags[0].Message != "fake error" {
		t.Fatalf("diags = %+v", diags)
	}
}

func TestActivityKeepsServerAlive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs, _ := vfs.New(root, vfs.Options{})

	mgr := NewManager(fs, nil)
	mgr.SetIdleTimeout(500 * time.Millisecond)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	if _, err := fs.Read("main.go"); err != nil {
		t.Fatal(err)
	}
	waitRunning(t, mgr, 1)

	stop := time.Now().Add(time.Second)
	for time.Now().Before(stop) {
		if _, err := fs.Read("main.go"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(150 * time.Millisecond)
	}
	if len(mgr.Running()) != 1 {
		t.Fatalf("server closed despite continuous activity: %v", mgr.Running())
	}
}

func TestConcurrentTouchesSingleServer(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("f%d.go", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	fs, _ := vfs.New(root, vfs.Options{})

	mgr := NewManager(fs, nil)
	mgr.SetTransportFactory(func(opts StdioOptions) (Transport, error) {
		return newScriptedTransport(), nil
	})
	defer mgr.Close()
	mgr.Start(map[string]config.LanguageServerConfig{
		"gopls": {Command: "/bin/true", FileTypes: []string{"go"}, RootMarkers: []string{"go.mod"}},
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := fs.Read(fmt.Sprintf("f%d.go", i)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	waitRunning(t, mgr, 1)

	deadline := time.After(5 * time.Second)
	for {
		diags := mgr.Diagnostics("f0.go")
		if len(diags) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("no diagnostics after concurrent reads")
		case <-time.After(50 * time.Millisecond):
		}
	}
	if got := len(mgr.Running()); got != 1 {
		t.Fatalf("running servers = %d, want exactly 1 shared gopls", got)
	}
}
