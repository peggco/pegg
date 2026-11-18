package changejournal

import (
	"context"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/peggco/pegg/internal/session"
)

func (m *Manager) snapshots(sessionID string) ([]session.Snapshot, error) {
	if m.sessions == nil {
		return nil, nil
	}
	return m.sessions.Snapshots(sessionID)
}

func (m *Manager) RevertMessage(sessionID, messageID string) (Result, error) {
	res := Result{SessionID: sessionID, Action: ActionRevert, RevertID: "manual-" + uuid.NewString()}
	if messageID == "" {
		return res, errors.New("changejournal: message id is required")
	}
	msgs, err := m.messages(sessionID)
	if err != nil {
		return res, err
	}
	calls := callsForMessage(msgs, messageID)

	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.loadLocked(sessionID)
	var targets []*Entry
	for _, e := range j.Entries {
		if e.MessageID == messageID || (e.MessageID == "" && calls[e.ToolCallID]) {
			if e.MessageID == "" {
				e.MessageID = messageID
			}
			targets = append(targets, e)
		}
	}
	sort.Slice(targets, func(i, k int) bool { return targets[i].Seq > targets[k].Seq })
	res = m.revertEntriesLocked(j, res.RevertID, targets)
	if err := m.saveLocked(j); err != nil {
		return res, err
	}
	return res, nil
}

func (m *Manager) revertSnapshot(sessionID, snapshotID string) (Result, error) {
	res := Result{SessionID: sessionID, RevertID: snapshotID}
	if snapshotID == "" {
		return res, nil
	}
	snaps, err := m.snapshots(sessionID)
	if err != nil {
		return res, err
	}
	var snap *session.Snapshot
	for i := range snaps {
		if snaps[i].ID == snapshotID {
			snap = &snaps[i]
			break
		}
	}
	if snap == nil {
		return res, nil
	}

	removed := make(map[string]bool, len(snap.Messages))
	calls := make(map[string]string)
	for _, msg := range snap.Messages {
		removed[msg.ID] = true
		for _, call := range msg.ToolCalls {
			if call.ID != "" {
				calls[call.ID] = msg.ID
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.loadLocked(sessionID)
	var targets []*Entry
	for _, e := range j.Entries {
		if e.MessageID != "" && removed[e.MessageID] {
			targets = append(targets, e)
			continue
		}
		if mid, ok := calls[e.ToolCallID]; ok {
			if e.MessageID == "" {
				e.MessageID = mid
			}
			targets = append(targets, e)
		}
	}
	sort.Slice(targets, func(i, k int) bool { return targets[i].Seq > targets[k].Seq })
	res = m.revertEntriesLocked(j, snapshotID, targets)
	if err := m.saveLocked(j); err != nil {
		return res, err
	}
	return res, nil
}

func (m *Manager) revertEntriesLocked(j *journal, revertID string, entries []*Entry) Result {
	res := Result{SessionID: j.SessionID, Action: ActionRevert, RevertID: revertID}
	for _, e := range entries {
		if !e.Tracked {
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusNotTracked, Reason: e.Reason})
			continue
		}
		if e.RevertedAt != nil {
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusAlreadyReverted})
			continue
		}
		if conflict := m.revertEntryLocked(e, revertID); conflict != "" {
			e.Conflict = conflict
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusConflict, Reason: conflict})
			continue
		}
		switch e.Op {
		case OpAdd:
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusDeleted})
		default:
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusRestored})
		}
	}
	return res
}

func (m *Manager) revertEntryLocked(e *Entry, revertID string) string {
	current, exists, err := m.currentHash(e.Path)
	if err != nil {
		return err.Error()
	}

	switch e.Op {
	case OpAdd:
		if !exists {
			m.markReverted(e, revertID, "")
			return ""
		}
		if current != e.AfterHash {
			return "file changed after the agent wrote it"
		}
		if err := m.fs.DeleteCtx(withSuppressed(context.Background()), e.Path); err != nil {
			return err.Error()
		}
		m.markReverted(e, revertID, "")
		return ""
	case OpModify:
		if !exists {
			return "file missing after the agent changed it"
		}
		if current != e.AfterHash {
			return "file changed after the agent wrote it"
		}
		if e.BackupPath == "" {
			return "missing backup"
		}
		data, err := m.readBackup(e.BackupPath)
		if err != nil {
			return "missing backup"
		}
		if _, err := m.fs.WriteCtx(withSuppressed(context.Background()), e.Path, data); err != nil {
			return err.Error()
		}
		m.markReverted(e, revertID, hashBytes(data))
		return ""
	case OpDelete:
		if exists {
			return "file changed after the agent deleted it"
		}
		if e.BackupPath == "" {
			return "missing backup"
		}
		data, err := m.readBackup(e.BackupPath)
		if err != nil {
			return "missing backup"
		}
		if _, err := m.fs.WriteCtx(withSuppressed(context.Background()), e.Path, data); err != nil {
			return err.Error()
		}
		m.markReverted(e, revertID, hashBytes(data))
		return ""
	default:
		return "unsupported operation"
	}
}

func (m *Manager) markReverted(e *Entry, revertID, restoredHash string) {
	now := time.Now()
	e.RevertedAt = &now
	e.RevertID = revertID
	e.RestoredHash = restoredHash
	e.Conflict = ""
}

func (m *Manager) UndoRevert(sessionID, revertID string) (Result, error) {
	res := Result{SessionID: sessionID, Action: ActionUndo, RevertID: revertID}
	if revertID == "" {
		return res, errors.New("changejournal: revert id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.loadLocked(sessionID)
	var targets []*Entry
	for _, e := range j.Entries {
		if e.RevertID == revertID && e.RevertedAt != nil {
			targets = append(targets, e)
		}
	}
	sort.Slice(targets, func(i, k int) bool { return targets[i].Seq < targets[k].Seq })

	for _, e := range targets {
		if conflict := m.undoEntryLocked(e); conflict != "" {
			e.UndoConflict = conflict
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusConflict, Reason: conflict})
			continue
		}
		if e.Op == OpDelete {
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusDeleted})
		} else {
			res.add(Change{Path: e.Path, Op: e.Op, Status: StatusRestored})
		}
	}
	if err := m.saveLocked(j); err != nil {
		return res, err
	}
	return res, nil
}

func (m *Manager) undoEntryLocked(e *Entry) string {
	current, exists, err := m.currentHash(e.Path)
	if err != nil {
		return err.Error()
	}

	switch e.Op {
	case OpAdd:
		if exists {
			return "file changed since the revert"
		}
		if e.ForwardBackup == "" {
			return "missing backup"
		}
		data, err := m.readBackup(e.ForwardBackup)
		if err != nil {
			return "missing backup"
		}
		if _, err := m.fs.WriteCtx(withSuppressed(context.Background()), e.Path, data); err != nil {
			return err.Error()
		}
	case OpModify:
		if !exists || current != e.RestoredHash {
			return "file changed since the revert"
		}
		if e.ForwardBackup == "" {
			return "missing backup"
		}
		data, err := m.readBackup(e.ForwardBackup)
		if err != nil {
			return "missing backup"
		}
		if _, err := m.fs.WriteCtx(withSuppressed(context.Background()), e.Path, data); err != nil {
			return err.Error()
		}
	case OpDelete:
		if !exists || current != e.RestoredHash {
			return "file changed since the revert"
		}
		if err := m.fs.DeleteCtx(withSuppressed(context.Background()), e.Path); err != nil {
			return err.Error()
		}
	default:
		return "unsupported operation"
	}

	e.RevertedAt = nil
	e.RevertID = ""
	e.RestoredHash = ""
	e.Conflict = ""
	e.UndoConflict = ""
	return ""
}

func (m *Manager) currentHash(rel string) (string, bool, error) {
	phys, err := m.fs.Resolve(rel)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(phys)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return hashBytes(data), true, nil
}

func (m *Manager) readBackup(rel string) ([]byte, error) {
	if rel == "" {
		return nil, errors.New("changejournal: missing backup")
	}
	return os.ReadFile(m.backupAbs(rel))
}
