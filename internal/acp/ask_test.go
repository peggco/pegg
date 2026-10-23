package acp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanFilePathPrefersNewestPlan(t *testing.T) {
	cwd := t.TempDir()
	plansDir := filepath.Join(cwd, ".pegg", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	older := filepath.Join(plansDir, "older.md")
	newer := filepath.Join(plansDir, "newer.md")
	if err := os.WriteFile(older, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, base, base); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, base.Add(time.Minute), base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	if got, want := planFilePath(cwd), newer; got != want {
		t.Fatalf("planFilePath = %q, want %q", got, want)
	}
}

func TestPlanFilePathFallsBackToPlanMD(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".pegg", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(cwd, ".pegg", "plans", "plan.md")
	if got := planFilePath(cwd); got != want {
		t.Fatalf("planFilePath = %q, want %q", got, want)
	}
}

func TestNewestPlanFileIgnoresNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := newestPlanFile(dir); got != "" {
		t.Fatalf("newestPlanFile = %q, want empty", got)
	}
}
