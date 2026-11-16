package changejournal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type journalFile struct {
	SessionID string   `json:"session_id"`
	Seq       int64    `json:"seq"`
	Bytes     int64    `json:"bytes"`
	Entries   []*Entry `json:"entries"`
}

type journal struct {
	journalFile
	path string
}

func (m *Manager) indexPath(sessionID string) string {
	return filepath.Join(m.dir, "sessions", safeSessionID(sessionID)+".json")
}

func (m *Manager) sessionDir(sessionID string) string {
	return filepath.Join(m.dir, "sessions", safeSessionID(sessionID))
}

func (m *Manager) backupRel(sessionID, entryID, side string) string {
	return filepath.Join("sessions", safeSessionID(sessionID), entryID+"."+side)
}

func (m *Manager) backupAbs(rel string) string {
	return filepath.Join(m.dir, filepath.FromSlash(rel))
}

func (m *Manager) loadLocked(sessionID string) *journal {
	if j, ok := m.journals[sessionID]; ok {
		return j
	}
	j := &journal{journalFile: journalFile{SessionID: sessionID}, path: m.indexPath(sessionID)}
	data, err := os.ReadFile(j.path)
	if err == nil {
		_ = json.Unmarshal(data, &j.journalFile)
	}
	if j.SessionID == "" {
		j.SessionID = sessionID
	}
	m.journals[sessionID] = j
	return j
}

func (m *Manager) saveLocked(j *journal) error {
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(&j.journalFile)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(j.path), ".changejournal-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, j.path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func (m *Manager) storeBackupLocked(j *journal, entryID, side string, data []byte) (string, error) {
	if int64(len(data)) > m.cfg.MaxFileSize || j.Bytes+int64(len(data)) > m.cfg.Budget {
		return "", errBudgetExceeded
	}
	rel := m.backupRel(j.SessionID, entryID, side)
	abs := m.backupAbs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o600); err != nil {
		return "", err
	}
	j.Bytes += int64(len(data))
	return rel, nil
}

func (m *Manager) removeBackupLocked(j *journal, rel string) {
	if rel == "" {
		return
	}
	abs := m.backupAbs(rel)
	if info, err := os.Stat(abs); err == nil {
		j.Bytes -= info.Size()
		if j.Bytes < 0 {
			j.Bytes = 0
		}
	}
	_ = os.Remove(abs)
}

func (m *Manager) deleteSessionLocked(sessionID string) {
	j := m.loadLocked(sessionID)
	for _, e := range j.Entries {
		m.removeBackupLocked(j, e.BackupPath)
		m.removeBackupLocked(j, e.ForwardBackup)
	}
	_ = os.Remove(j.path)
	_ = os.RemoveAll(m.sessionDir(sessionID))
	delete(m.journals, sessionID)
}

func (m *Manager) pruneLocked(now time.Time) {
	cutoff := now.AddDate(0, 0, -m.cfg.RetentionDays)
	entries, err := os.ReadDir(filepath.Join(m.dir, "sessions"))
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		info, err := de.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		sessionID := strings.TrimSuffix(de.Name(), ".json")
		m.deleteSessionLocked(sessionID)
	}
}

func safeSessionID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "_"
	}
	return out
}

var errBudgetExceeded = fmt.Errorf("changejournal: budget exceeded")
