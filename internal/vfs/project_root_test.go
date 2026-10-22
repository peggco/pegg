package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newProjectVFS(t *testing.T, root, projectRoot string) *VFS {
	t.Helper()
	fs, err := New(root, Options{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestProjectRootRedirectsPeggPaths(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	fs := newProjectVFS(t, root, project)

	phys, err := fs.Resolve(".pegg/plans/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(project, "plans", "plan.md")
	if phys != want {
		t.Fatalf("Resolve = %q, want %q", phys, want)
	}
	if rel := fs.Virtual(phys); rel != ".pegg/plans/plan.md" {
		t.Fatalf("Virtual = %q, want %q", rel, ".pegg/plans/plan.md")
	}
}

func TestProjectRootWriteScopeCreatesRelocatedDirectory(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	fs := newProjectVFS(t, root, project)

	scoped, err := fs.WriteScope(PlansDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(project, "plans")); err != nil {
		t.Fatalf("plans scope not created in project root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pegg")); !os.IsNotExist(err) {
		t.Fatal("plans scope must not be created inside the workspace")
	}

	if _, err := scoped.Write(".pegg/plans/plan.md", []byte("plan")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(project, "plans", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "plan" {
		t.Fatalf("plan content = %q", data)
	}
}

func TestProjectRootKeepsPeggWritePolicy(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	fs := newProjectVFS(t, root, project)

	if _, err := fs.Write(".pegg/memory/notes.md", []byte("x")); !errors.Is(err, ErrIgnored) {
		t.Fatalf("writing to .pegg/memory must stay blocked, got %v", err)
	}
	if _, err := fs.Write(".pegg/rules/rule.md", []byte("x")); err != nil {
		t.Fatalf("writing rules must be allowed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "rules", "rule.md")); err != nil {
		t.Fatalf("rule not written to project root: %v", err)
	}
}

func TestProjectRootReadReturnsRelocatedContent(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	fs := newProjectVFS(t, root, project)

	if err := os.MkdirAll(filepath.Join(project, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "plans", "plan.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, err := fs.Read(".pegg/plans/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "hello") {
		t.Fatalf("content = %q, want it to contain %q", content, "hello")
	}
}

func TestWithoutProjectRootPeggPathsStayInWorkspace(t *testing.T) {
	root := t.TempDir()
	fs := newTestVFS(t, root)

	phys, err := fs.Resolve(".pegg/plans/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".pegg", "plans", "plan.md"); phys != want {
		t.Fatalf("Resolve = %q, want %q", phys, want)
	}
}
