//go:build gui

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexFilesSkipsHeavyDirs(t *testing.T) {
	root := t.TempDir()
	must := func(rel string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("main.go")
	must("src/app.tsx")
	must("node_modules/pkg/index.js")
	must(".git/config")
	must("dist/bundle.js")
	resetFileIndex()
	files := indexFiles(root)
	seen := map[string]bool{}
	for _, file := range files {
		seen[file] = true
	}
	if !seen["main.go"] || !seen["src/app.tsx"] {
		t.Fatalf("expected source files, got %v", files)
	}
	for _, skipped := range []string{"node_modules/pkg/index.js", ".git/config", "dist/bundle.js"} {
		if seen[skipped] {
			t.Fatalf("expected %s to be skipped, got %v", skipped, files)
		}
	}
}
