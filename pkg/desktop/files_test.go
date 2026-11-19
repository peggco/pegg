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

func writeFileAt(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeHome(t *testing.T, home, platform string) {
	t.Helper()
	prevHome, prevOS := userHomeDir, goos
	userHomeDir = func() (string, error) { return home, nil }
	goos = platform
	t.Cleanup(func() {
		userHomeDir, goos = prevHome, prevOS
		resetFileIndex()
	})
	resetFileIndex()
}

func TestIndexFilesSkipsHomeWorkspace(t *testing.T) {
	home := t.TempDir()
	writeFileAt(t, home, "notes.txt")
	writeFileAt(t, home, "projects/app/main.go")
	fakeHome(t, home, "darwin")
	if files := indexFiles(home); len(files) != 0 {
		t.Fatalf("expected the home directory not to be indexed, got %v", files)
	}
}

func TestIndexFilesSkipsProtectedHomeDirsOnDarwin(t *testing.T) {
	users := t.TempDir()
	home := filepath.Join(users, "me")
	writeFileAt(t, home, "Documents/secret.go")
	writeFileAt(t, home, "Pictures/photo.go")
	writeFileAt(t, home, "code/app/main.go")
	fakeHome(t, home, "darwin")
	files := indexFiles(users)
	seen := map[string]bool{}
	for _, file := range files {
		seen[file] = true
	}
	if !seen["me/code/app/main.go"] {
		t.Fatalf("expected project files, got %v", files)
	}
	for _, skipped := range []string{"me/Documents/secret.go", "me/Pictures/photo.go"} {
		if seen[skipped] {
			t.Fatalf("expected %s to be skipped, got %v", skipped, files)
		}
	}
}

func TestIndexFilesKeepsProtectedNamesOffDarwin(t *testing.T) {
	users := t.TempDir()
	home := filepath.Join(users, "me")
	writeFileAt(t, home, "Documents/notes.go")
	fakeHome(t, home, "linux")
	files := indexFiles(users)
	for _, file := range files {
		if file == "me/Documents/notes.go" {
			return
		}
	}
	t.Fatalf("expected Documents to be indexed on linux, got %v", files)
}
