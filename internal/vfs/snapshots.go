package vfs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

type snapshotFile struct {
	Root  string            `json:"root,omitempty"`
	Files map[string]string `json:"files"`
}

type snapshotStore struct {
	mu   sync.RWMutex
	m    map[string]string
	root string
	path string
	log  *logger.Logger
}

func newSnapshotStore(path, root string, log *logger.Logger) *snapshotStore {
	s := &snapshotStore{
		m:    make(map[string]string),
		root: root,
		path: path,
		log:  log,
	}
	if path != "" {
		s.load()
	}
	return s
}

func (s *snapshotStore) get(rel string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hash, ok := s.m[rel]
	return hash, ok
}

func (s *snapshotStore) set(rel, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[rel] = hash
	s.saveLocked()
}

func (s *snapshotStore) delete(rel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[rel]; !ok {
		return
	}
	delete(s.m, rel)
	s.saveLocked()
}

func (s *snapshotStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			s.warn("vfs: read snapshot store %s: %v", s.path, err)
		}
		return
	}
	if len(data) == 0 {
		return
	}
	var f snapshotFile
	if err := json.Unmarshal(data, &f); err != nil {
		s.warn("vfs: parse snapshot store %s: %v", s.path, err)
		return
	}
	if f.Files != nil {
		s.m = f.Files
	}
}

func (s *snapshotStore) saveLocked() {
	if s.path == "" {
		return
	}
	data, err := json.MarshalIndent(snapshotFile{Root: s.root, Files: s.m}, "", "  ")
	if err != nil {
		s.warn("vfs: marshal snapshot store: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		s.warn("vfs: create snapshot dir: %v", err)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		s.warn("vfs: write snapshot store: %v", err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		s.warn("vfs: replace snapshot store: %v", err)
	}
}

func (s *snapshotStore) warn(format string, args ...any) {
	if s.log != nil {
		s.log.Fwarn(format, args...)
	}
}

func DefaultSnapshotPath(root string) (string, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return SnapshotPathIn(dir, root)
}

func SnapshotPathIn(dir, root string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("vfs: snapshot dir is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	name := hex.EncodeToString(sum[:])[:16] + ".json"
	return filepath.Join(dir, "vfs", name), nil
}
