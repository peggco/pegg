package vfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	json "github.com/goccy/go-json"
)

func newPersistentVFS(t *testing.T, root, statePath string) *VFS {
	t.Helper()
	fs, err := New(root, Options{SnapshotPath: statePath})
	if err != nil {
		t.Fatal(err)
	}
	return fs
}

func TestSnapshotsPersistAcrossInstances(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := newPersistentVFS(t, root, statePath)
	if _, err := first.Read("a.txt"); err != nil {
		t.Fatal(err)
	}

	second := newPersistentVFS(t, root, statePath)
	if _, err := second.Edit("a.txt", "hello", "bye", false); err != nil {
		t.Fatalf("edit after restart should be allowed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(data) != "bye world" {
		t.Fatalf("edit not applied: %q", data)
	}
}

func TestSnapshotsPersistDetectsExternalChange(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := newPersistentVFS(t, root, statePath)
	if _, err := first.Read("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	second := newPersistentVFS(t, root, statePath)
	if _, err := second.Edit("a.txt", "original", "new", false); !errors.Is(err, ErrFileChanged) {
		t.Fatalf("want ErrFileChanged, got %v", err)
	}
}

func TestSnapshotsForgetPersists(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := newPersistentVFS(t, root, statePath)
	if _, err := first.Read("a.txt"); err != nil {
		t.Fatal(err)
	}
	first.Forget("a.txt")

	second := newPersistentVFS(t, root, statePath)
	if _, err := second.Edit("a.txt", "hello", "bye", false); !errors.Is(err, ErrReadRequired) {
		t.Fatalf("want ErrReadRequired, got %v", err)
	}
}

func TestSnapshotsDeleteRemovesPersistedEntry(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := newPersistentVFS(t, root, statePath)
	if _, err := fs.Read("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete("a.txt"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var f snapshotFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Files["a.txt"]; ok {
		t.Fatalf("deleted entry still persisted: %v", f.Files)
	}
}

func TestSnapshotStoreCorruptFileIsEmpty(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "snapshots.json")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := newPersistentVFS(t, root, statePath)
	if _, err := fs.Edit("a.txt", "hello", "bye", false); !errors.Is(err, ErrReadRequired) {
		t.Fatalf("want ErrReadRequired, got %v", err)
	}
}

func TestSnapshotPathStableAndScoped(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()

	p1, err := SnapshotPathIn(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := SnapshotPathIn(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("snapshot path not stable: %q != %q", p1, p2)
	}
	if !strings.HasPrefix(p1, filepath.Join(dir, "vfs")+string(filepath.Separator)) {
		t.Fatalf("snapshot path %q not under %q", p1, filepath.Join(dir, "vfs"))
	}
	if filepath.Ext(p1) != ".json" {
		t.Fatalf("snapshot path %q missing .json suffix", p1)
	}

	other, err := SnapshotPathIn(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if other == p1 {
		t.Fatalf("different roots must map to different files: %q", p1)
	}

	if _, err := SnapshotPathIn("", root); err == nil {
		t.Fatal("empty dir must fail")
	}
}
