package changejournal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

type SessionSource interface {
	Messages(sessionID string) ([]session.Message, error)
	Snapshots(sessionID string) ([]session.Snapshot, error)
}

type Deps struct {
	Config   config.ChangeJournalConfig
	FS       *vfs.VFS
	Sessions SessionSource
	Bus      event.Bus
	Log      *logger.Logger
	Dir      string
}

type Manager struct {
	cfg      config.ChangeJournalConfig
	fs       *vfs.VFS
	sessions SessionSource
	bus      event.Bus
	log      *logger.Logger
	dir      string

	mu       sync.Mutex
	journals map[string]*journal
	started  bool
	closed   atomic.Bool
}

func New(deps Deps) (*Manager, error) {
	if deps.FS == nil {
		return nil, errors.New("changejournal: vfs is required")
	}
	cfg := deps.Config
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = config.DefaultChangeJournalMaxFileSize
	}
	if cfg.Budget <= 0 {
		cfg.Budget = config.DefaultChangeJournalBudget
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = config.DefaultChangeJournalRetention
	}
	dir := deps.Dir
	if dir == "" {
		var err error
		dir, err = DefaultDir(deps.FS.Root())
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o700); err != nil {
		return nil, fmt.Errorf("changejournal: create journal directory: %w", err)
	}
	return &Manager{
		cfg:      cfg,
		fs:       deps.FS,
		sessions: deps.Sessions,
		bus:      deps.Bus,
		log:      deps.Log,
		dir:      dir,
		journals: make(map[string]*journal),
	}, nil
}

func DefaultDir(root string) (string, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return DirIn(dir, root)
}

func DirIn(configDir, root string) (string, error) {
	if configDir == "" {
		return "", errors.New("changejournal: config dir is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	name := hex.EncodeToString(sum[:])[:16]
	return filepath.Join(configDir, "changejournal", name), nil
}

func (m *Manager) Dir() string {
	return m.dir
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.mu.Unlock()

	if !m.cfg.Active() {
		return nil
	}

	m.fs.OnAfterMutation(m.record)
	if m.bus != nil {
		_ = m.bus.Subscribe(session.TopicSessionReverted, m.onReverted)
		_ = m.bus.Subscribe(session.TopicSessionRestored, m.onRestored)
		_ = m.bus.Subscribe(session.TopicSessionDeleted, m.onDeleted)
	}

	m.mu.Lock()
	m.pruneLocked(time.Now())
	m.mu.Unlock()
	return nil
}

func (m *Manager) Close() error {
	m.closed.Store(true)
	return nil
}

func (m *Manager) record(ev vfs.FileMutation) {
	if !m.cfg.Active() || ev.Ctx == nil || isSuppressed(ev.Ctx) || m.closed.Load() {
		return
	}
	sessionID := llm.SessionIDFrom(ev.Ctx)
	if sessionID == "" || ev.Path == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		return
	}

	j := m.loadLocked(sessionID)
	j.Seq++
	entry := &Entry{
		ID:         uuid.NewString(),
		SessionID:  sessionID,
		ToolCallID: tool.CallIDFrom(ev.Ctx),
		Path:       ev.Path,
		Op:         mapOp(ev.Op),
		BeforeHash: ev.BeforeHash,
		AfterHash:  ev.AfterHash,
		Seq:        j.Seq,
		CreatedAt:  time.Now(),
	}
	m.prepareEntryLocked(j, entry, ev)
	j.Entries = append(j.Entries, entry)
	if err := m.saveLocked(j); err != nil {
		m.warn("changejournal: save %s: %v", sessionID, err)
	}
}

func (m *Manager) prepareEntryLocked(j *journal, e *Entry, ev vfs.FileMutation) {
	e.Size = ev.BeforeSize
	if ev.AfterSize > e.Size {
		e.Size = ev.AfterSize
	}

	if ev.Truncated {
		m.untracked(e, "file too large")
		return
	}
	if ev.BeforeSize > m.cfg.MaxFileSize || ev.AfterSize > m.cfg.MaxFileSize {
		m.untracked(e, "file too large")
		return
	}
	if isBinary(ev.Before) || isBinary(ev.After) {
		m.untracked(e, "binary file")
		return
	}

	switch e.Op {
	case OpAdd:
		if ev.After == nil && ev.AfterSize > 0 {
			m.untracked(e, "content unavailable")
			return
		}
		if !m.storeSideLocked(j, e, "after", ev.After) {
			return
		}
	case OpModify:
		if (ev.Before == nil && ev.BeforeSize > 0) || (ev.After == nil && ev.AfterSize > 0) {
			m.untracked(e, "content unavailable")
			return
		}
		if !m.storeSideLocked(j, e, "before", ev.Before) {
			return
		}
		if !m.storeSideLocked(j, e, "after", ev.After) {
			return
		}
	case OpDelete:
		if ev.Before == nil && ev.BeforeSize > 0 {
			m.untracked(e, "content unavailable")
			return
		}
		if !m.storeSideLocked(j, e, "before", ev.Before) {
			return
		}
	default:
		m.untracked(e, "unsupported operation")
		return
	}
	e.Tracked = true
}

func (m *Manager) storeSideLocked(j *journal, e *Entry, side string, data []byte) bool {
	rel, err := m.storeBackupLocked(j, e.ID, side, data)
	if err != nil {
		m.removeBackupLocked(j, e.BackupPath)
		m.removeBackupLocked(j, e.ForwardBackup)
		e.BackupPath = ""
		e.ForwardBackup = ""
		if errors.Is(err, errBudgetExceeded) {
			m.untracked(e, "journal budget exceeded")
		} else {
			m.untracked(e, "backup failed")
		}
		return false
	}
	if side == "before" {
		e.BackupPath = rel
	} else {
		e.ForwardBackup = rel
	}
	return true
}

func (m *Manager) untracked(e *Entry, reason string) {
	e.Tracked = false
	e.Reason = reason
	e.BackupPath = ""
	e.ForwardBackup = ""
}

func mapOp(op vfs.MutationOp) Op {
	switch op {
	case vfs.MutationAdd:
		return OpAdd
	case vfs.MutationDelete:
		return OpDelete
	default:
		return OpModify
	}
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) Entries(sessionID string) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.loadLocked(sessionID)
	out := make([]Entry, 0, len(j.Entries))
	for _, e := range j.Entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Seq < out[k].Seq })
	return out, nil
}

func (m *Manager) EntriesForMessage(sessionID, messageID string) ([]Entry, error) {
	if messageID == "" {
		return nil, nil
	}
	msgs, err := m.messages(sessionID)
	if err != nil {
		return nil, err
	}
	calls := callsForMessage(msgs, messageID)
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.loadLocked(sessionID)
	out := make([]Entry, 0)
	for _, e := range j.Entries {
		if e.MessageID == messageID || (e.MessageID == "" && calls[e.ToolCallID]) {
			cp := *e
			if cp.MessageID == "" {
				cp.MessageID = messageID
			}
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Seq < out[k].Seq })
	return out, nil
}

func (m *Manager) messages(sessionID string) ([]session.Message, error) {
	if m.sessions == nil {
		return nil, nil
	}
	return m.sessions.Messages(sessionID)
}

func callsForMessage(msgs []session.Message, messageID string) map[string]bool {
	calls := make(map[string]bool)
	for _, msg := range msgs {
		if msg.ID != messageID {
			continue
		}
		for _, call := range msg.ToolCalls {
			if call.ID != "" {
				calls[call.ID] = true
			}
		}
	}
	return calls
}

func (m *Manager) onReverted(ev session.SessionReverted) {
	res, err := m.revertSnapshot(ev.SessionID, ev.SnapshotID)
	if err != nil {
		m.warn("changejournal: revert %s: %v", ev.SessionID, err)
		return
	}
	m.publish(TopicChangeJournalReverted, res)
}

func (m *Manager) onRestored(ev session.SessionRestored) {
	res, err := m.UndoRevert(ev.SessionID, ev.SnapshotID)
	if err != nil {
		m.warn("changejournal: undo %s: %v", ev.SessionID, err)
		return
	}
	m.publish(TopicChangeJournalRestored, res)
}

func (m *Manager) onDeleted(ev session.SessionDeleted) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteSessionLocked(ev.SessionID)
}

func (m *Manager) publish(topic string, res Result) {
	if m.bus == nil {
		return
	}
	m.bus.Publish(topic, res)
	if topic != TopicChangeJournal {
		m.bus.Publish(TopicChangeJournal, res)
	}
}

func (m *Manager) warn(format string, args ...any) {
	if m.log != nil {
		m.log.Fwarn(format, args...)
	}
}

type suppressKey struct{}

func withSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressKey{}, true)
}

func isSuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(suppressKey{}).(bool)
	return v
}
