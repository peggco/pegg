package changejournal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

type harness struct {
	root      string
	fs        *vfs.VFS
	sessions  *session.Manager
	bus       event.Bus
	journal   *Manager
	ctx       context.Context
	sessionID string
	reverted  []Result
	restored  []Result
}

func newHarness(t *testing.T, cfg config.ChangeJournalConfig) *harness {
	t.Helper()
	root := t.TempDir()
	fs, err := vfs.New(root, vfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := session.NewJSONStoreAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := event.New()
	sessions := session.NewManager(store, bus, nil)
	sess, err := sessions.Create(session.CreateOptions{Title: "journal test"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 1 << 20
	}
	if cfg.Budget == 0 {
		cfg.Budget = 1 << 20
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 30
	}
	journal, err := New(Deps{
		Config:   cfg,
		FS:       fs,
		Sessions: sessions,
		Bus:      bus,
		Dir:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Start(); err != nil {
		t.Fatal(err)
	}

	h := &harness{
		root:      root,
		fs:        fs,
		sessions:  sessions,
		bus:       bus,
		journal:   journal,
		ctx:       llm.WithSessionID(context.Background(), sess.ID),
		sessionID: sess.ID,
	}
	_ = bus.Subscribe(TopicChangeJournalReverted, func(r Result) {
		h.reverted = append(h.reverted, r)
	})
	_ = bus.Subscribe(TopicChangeJournalRestored, func(r Result) {
		h.restored = append(h.restored, r)
	})
	t.Cleanup(func() {
		_ = journal.Close()
		_ = sessions.Close()
	})
	return h
}

func (h *harness) write(t *testing.T, callID, path string, data []byte) {
	t.Helper()
	ctx := tool.WithCallID(h.ctx, callID)
	if _, err := h.fs.WriteCtx(ctx, path, data); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func (h *harness) delete(t *testing.T, callID, path string) {
	t.Helper()
	ctx := tool.WithCallID(h.ctx, callID)
	if err := h.fs.DeleteCtx(ctx, path); err != nil {
		t.Fatalf("delete %s: %v", path, err)
	}
}

func (h *harness) message(t *testing.T, callIDs ...string) session.Message {
	t.Helper()
	calls := make([]llm.ToolCall, 0, len(callIDs))
	for _, id := range callIDs {
		calls = append(calls, llm.ToolCall{
			ID:       id,
			Type:     "function",
			Function: llm.Function{Name: "write"},
		})
	}
	msg, err := h.sessions.AppendMessage(h.sessionID, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "working",
		ToolCalls: calls,
	})
	if err != nil {
		t.Fatal(err)
	}
	return *msg
}

func (h *harness) read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.root, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func (h *harness) missing(path string) bool {
	_, err := os.Stat(filepath.Join(h.root, path))
	return os.IsNotExist(err)
}

func (h *harness) seed(t *testing.T, path, content string) {
	t.Helper()
	full := filepath.Join(h.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRevertAndUndoRestoreFiles(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	h.seed(t, "base.txt", "base")
	h.seed(t, "old.txt", "old")

	msg := h.message(t, "call-1", "call-2")
	h.write(t, "call-1", "created.txt", []byte("agent"))
	h.write(t, "call-2", "base.txt", []byte("agent-base"))
	h.delete(t, "call-2", "old.txt")

	snapshotID, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !h.missing("created.txt") {
		t.Fatal("agent-created file should be removed by revert")
	}
	if got := h.read(t, "base.txt"); got != "base" {
		t.Fatalf("base.txt = %q, want base", got)
	}
	if got := h.read(t, "old.txt"); got != "old" {
		t.Fatalf("old.txt = %q, want old", got)
	}
	if len(h.reverted) != 1 {
		t.Fatalf("revert events = %d, want 1", len(h.reverted))
	}
	if h.reverted[0].Restored != 2 || h.reverted[0].Deleted != 1 || h.reverted[0].Conflicts != 0 {
		t.Fatalf("revert result = %+v", h.reverted[0])
	}

	if err := h.sessions.UndoRevert(h.sessionID, snapshotID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "created.txt"); got != "agent" {
		t.Fatalf("created.txt = %q, want agent", got)
	}
	if got := h.read(t, "base.txt"); got != "agent-base" {
		t.Fatalf("base.txt = %q, want agent-base", got)
	}
	if !h.missing("old.txt") {
		t.Fatal("old.txt should be deleted again by undo")
	}
	if len(h.restored) != 1 {
		t.Fatalf("restore events = %d, want 1", len(h.restored))
	}
}

func TestRevertSkipsUserEdits(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	h.seed(t, "base.txt", "base")

	msg := h.message(t, "call-1")
	h.write(t, "call-1", "base.txt", []byte("agent"))
	if err := os.WriteFile(filepath.Join(h.root, "base.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshotID, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "base.txt"); got != "user" {
		t.Fatalf("base.txt = %q, user edit must survive", got)
	}
	if len(h.reverted) != 1 || h.reverted[0].Conflicts != 1 {
		t.Fatalf("revert result = %+v", h.reverted)
	}
	if err := h.sessions.UndoRevert(h.sessionID, snapshotID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "base.txt"); got != "user" {
		t.Fatalf("base.txt after undo = %q, user edit must survive", got)
	}
	if len(h.restored) != 1 || len(h.restored[0].Files) != 0 {
		t.Fatalf("undo result = %+v", h.restored)
	}
}

func TestRepeatedRevertIsIdempotent(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	msg := h.message(t, "call-1")
	h.write(t, "call-1", "created.txt", []byte("agent"))

	first, err := h.journal.RevertMessage(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Deleted != 1 {
		t.Fatalf("first revert = %+v", first)
	}
	second, err := h.journal.RevertMessage(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Deleted != 0 || second.Skipped != 1 {
		t.Fatalf("second revert = %+v", second)
	}
	if !h.missing("created.txt") {
		t.Fatal("created.txt should stay deleted")
	}
}

func TestBinaryAndLargeFilesAreNotTracked(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{MaxFileSize: 16, Budget: 1 << 20})
	msg := h.message(t, "call-1", "call-2")

	h.write(t, "call-1", "blob.bin", []byte{0x00, 0x01, 0x02})
	h.write(t, "call-2", "big.txt", []byte(strings.Repeat("x", 32)))

	if _, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "blob.bin"); got != string([]byte{0x00, 0x01, 0x02}) {
		t.Fatalf("binary file changed: %q", got)
	}
	if got := h.read(t, "big.txt"); got != strings.Repeat("x", 32) {
		t.Fatalf("large file changed")
	}
	if len(h.reverted) != 1 || h.reverted[0].Skipped != 2 || h.reverted[0].Restored != 0 {
		t.Fatalf("revert result = %+v", h.reverted)
	}
	entries, err := h.journal.EntriesForMessage(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Tracked {
			t.Fatalf("entry should not be tracked: %+v", e)
		}
	}
}

func TestMissingBackupIsConflict(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	h.seed(t, "base.txt", "base")
	msg := h.message(t, "call-1")
	h.write(t, "call-1", "base.txt", []byte("agent"))

	entries, err := h.journal.EntriesForMessage(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].BackupPath == "" {
		t.Fatalf("entries = %+v", entries)
	}
	if err := os.Remove(h.journal.backupAbs(entries[0].BackupPath)); err != nil {
		t.Fatal(err)
	}

	if _, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "base.txt"); got != "agent" {
		t.Fatalf("base.txt = %q, want agent", got)
	}
	if len(h.reverted) != 1 || h.reverted[0].Conflicts != 1 {
		t.Fatalf("revert result = %+v", h.reverted)
	}
}

func TestSessionDeletionRemovesJournal(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	msg := h.message(t, "call-1")
	h.write(t, "call-1", "created.txt", []byte("agent"))

	entries, err := h.journal.EntriesForMessage(h.sessionID, msg.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
	if _, err := os.Stat(h.journal.sessionDir(h.sessionID)); err != nil {
		t.Fatal(err)
	}

	if err := h.sessions.Delete(h.sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.journal.indexPath(h.sessionID)); !os.IsNotExist(err) {
		t.Fatalf("journal index still exists: %v", err)
	}
	if _, err := os.Stat(h.journal.sessionDir(h.sessionID)); !os.IsNotExist(err) {
		t.Fatalf("journal backups still exist: %v", err)
	}
}

func TestEmptyFileRoundTrip(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{})
	h.seed(t, "base.txt", "base")
	msg := h.message(t, "call-1", "call-2")
	h.write(t, "call-1", "base.txt", nil)
	h.write(t, "call-2", "empty.txt", nil)

	snapshotID, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "base.txt"); got != "base" {
		t.Fatalf("base.txt = %q, want base", got)
	}
	if !h.missing("empty.txt") {
		t.Fatal("empty.txt should be removed by revert")
	}
	if err := h.sessions.UndoRevert(h.sessionID, snapshotID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "base.txt"); got != "" {
		t.Fatalf("base.txt = %q, want empty", got)
	}
	if got := h.read(t, "empty.txt"); got != "" {
		t.Fatalf("empty.txt = %q, want empty", got)
	}
}

func TestBudgetExceededIsNotTracked(t *testing.T) {
	h := newHarness(t, config.ChangeJournalConfig{MaxFileSize: 1 << 10, Budget: 4})
	msg := h.message(t, "call-1")
	h.write(t, "call-1", "created.txt", []byte("agent-data"))

	if _, _, err := h.sessions.RevertFrom(h.sessionID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if got := h.read(t, "created.txt"); got != "agent-data" {
		t.Fatalf("created.txt = %q, want agent-data", got)
	}
	if len(h.reverted) != 1 || h.reverted[0].Skipped != 1 {
		t.Fatalf("revert result = %+v", h.reverted)
	}
}
