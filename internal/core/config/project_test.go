package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectRootUsesLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	ResetProjectRoot()
	t.Cleanup(ResetProjectRoot)

	root, err := ProjectRoot()
	if err != nil {
		t.Fatalf("ProjectRoot: %v", err)
	}
	want := filepath.Join(dir, ProjectConfigDirName)
	if root != want {
		t.Fatalf("project root = %q, want %q", root, want)
	}
	if ProjectRootFallback() {
		t.Fatal("project root must not fall back in a writable directory")
	}
	if notice := ProjectRootNotice(); notice != "" {
		t.Fatalf("unexpected notice: %q", notice)
	}
}

func TestProjectRootFallsBackWhenDirectoryIsReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	t.Chdir(dir)
	ResetProjectRoot()
	t.Cleanup(ResetProjectRoot)

	root, err := ProjectRoot()
	if err != nil {
		t.Fatalf("ProjectRoot: %v", err)
	}
	if !ProjectRootFallback() {
		t.Fatalf("expected fallback project root, got %q", root)
	}
	want := filepath.Join(home, GlobalConfigDirName, "projects", WorkspaceKey(dir))
	if root != want {
		t.Fatalf("project root = %q, want %q", root, want)
	}
	if notice := ProjectRootNotice(); !strings.Contains(notice, "unusable") {
		t.Fatalf("notice = %q, want an explanation", notice)
	}
}

func TestProjectReadPathsIncludeLocalDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	ResetProjectRoot()
	t.Cleanup(ResetProjectRoot)

	skills := filepath.Join(dir, ProjectConfigDirName, "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "demo.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := ProjectReadPaths("skills")
	if len(paths) == 0 {
		t.Fatal("expected at least one read path")
	}
	if paths[0] != skills {
		t.Fatalf("read path = %q, want %q", paths[0], skills)
	}
}

func TestWorkspaceKeyIsStable(t *testing.T) {
	dir := t.TempDir()
	first := WorkspaceKey(dir)
	second := WorkspaceKey(dir)
	if first != second {
		t.Fatalf("workspace key is not stable: %q != %q", first, second)
	}
	if len(first) != 16 {
		t.Fatalf("workspace key length = %d, want 16", len(first))
	}
	if other := WorkspaceKey(t.TempDir()); other == first {
		t.Fatal("different directories must produce different keys")
	}
}
