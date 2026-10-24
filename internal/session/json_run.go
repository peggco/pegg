package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const jsonRunsFileName = "_runs.jsonl"

type jsonRunsFile struct {
	Runs             []Run          `json:"runs"`
	Events           []RunEvent     `json:"events"`
	Commands         []RunCommand   `json:"commands"`
	NextSeq          int64          `json:"next_seq"`
	NextCommand      int64          `json:"next_command"`
	SessionEvents    []SessionEvent `json:"session_events,omitempty"`
	NextSessionEvent int64          `json:"next_session_event"`
}

func (s *JSONStore) runsPath() string {
	return filepath.Join(s.dir, jsonRunsFileName)
}

func (s *JSONStore) loadRuns() (*jsonRunsFile, error) {
	data, err := os.ReadFile(s.runsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &jsonRunsFile{}, nil
		}
		return nil, err
	}
	var f jsonRunsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *JSONStore) persistRuns(f *jsonRunsFile) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(s.runsPath(), data, 0644)
}

func (s *JSONStore) CreateRun(r Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	if r.State == RunStateRunning {
		for _, existing := range f.Runs {
			if existing.SessionID == r.SessionID && existing.State == RunStateRunning {
				return ErrRunBusy
			}
		}
	}
	f.Runs = append(f.Runs, r)
	return s.persistRuns(f)
}

func (s *JSONStore) UpdateRun(r Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	for i := range f.Runs {
		if f.Runs[i].ID == r.ID {
			f.Runs[i] = r
			return s.persistRuns(f)
		}
	}
	return ErrRunNotFound
}

func (s *JSONStore) GetRun(id string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	for i := range f.Runs {
		if f.Runs[i].ID == id {
			r := f.Runs[i]
			return &r, nil
		}
	}
	return nil, ErrRunNotFound
}

func (s *JSONStore) ActiveRun(sessionID string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	var latest *Run
	for i := range f.Runs {
		r := f.Runs[i]
		if r.SessionID != sessionID || r.State != RunStateRunning {
			continue
		}
		if latest == nil || r.StartedAt.After(latest.StartedAt) {
			latest = &r
		}
	}
	if latest == nil {
		return nil, ErrRunNotFound
	}
	return latest, nil
}

func (s *JSONStore) ListRuns() ([]Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	out := append([]Run(nil), f.Runs...)
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

func (s *JSONStore) FinishRun(id string, state RunState, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	now := time.Now()
	for i := range f.Runs {
		if f.Runs[i].ID == id {
			f.Runs[i].State = state
			f.Runs[i].Error = errMsg
			f.Runs[i].UpdatedAt = now
			f.Runs[i].FinishedAt = now
			return s.persistRuns(f)
		}
	}
	return ErrRunNotFound
}

func (s *JSONStore) RequestRunCancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	for i := range f.Runs {
		if f.Runs[i].ID == id && f.Runs[i].State == RunStateRunning {
			f.Runs[i].CancelRequested = true
			f.Runs[i].UpdatedAt = time.Now()
			return s.persistRuns(f)
		}
	}
	return ErrRunNotFound
}

func (s *JSONStore) AppendRunEvents(events []RunEvent) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, e := range events {
		f.NextSeq++
		e.Seq = f.NextSeq
		if e.CreatedAt.IsZero() {
			e.CreatedAt = now
		}
		f.Events = append(f.Events, e)
	}
	for i := range f.Runs {
		if f.Runs[i].ID == events[len(events)-1].RunID && f.Runs[i].State == RunStateRunning {
			f.Runs[i].UpdatedAt = now
		}
	}
	return s.persistRuns(f)
}

func (s *JSONStore) RunEvents(runID string, afterSeq int64, limit int) ([]RunEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	out := make([]RunEvent, 0, limit)
	for _, e := range f.Events {
		if e.RunID != runID || e.Seq <= afterSeq {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *JSONStore) LastRunEventSeq(runID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return 0, err
	}
	var seq int64
	for _, e := range f.Events {
		if e.RunID == runID && e.Seq > seq {
			seq = e.Seq
		}
	}
	return seq, nil
}

func (s *JSONStore) AddRunCommand(cmd RunCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	f.NextCommand++
	cmd.ID = f.NextCommand
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = time.Now()
	}
	f.Commands = append(f.Commands, cmd)
	return s.persistRuns(f)
}

func (s *JSONStore) PendingRunCommands(runID string) ([]RunCommand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	var out []RunCommand
	for _, c := range f.Commands {
		if c.RunID == runID && c.ID > 0 {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *JSONStore) ConsumeRunCommand(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	for i := range f.Commands {
		if f.Commands[i].ID == id {
			f.Commands = append(f.Commands[:i], f.Commands[i+1:]...)
			return s.persistRuns(f)
		}
	}
	return nil
}

func (s *JSONStore) PruneRuns(before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	removed := make(map[string]bool)
	var runs []Run
	for _, r := range f.Runs {
		if r.State != RunStateRunning && !r.FinishedAt.IsZero() && r.FinishedAt.Before(before) {
			removed[r.ID] = true
			continue
		}
		runs = append(runs, r)
	}
	f.Runs = runs
	var events []RunEvent
	for _, e := range f.Events {
		if !removed[e.RunID] {
			events = append(events, e)
		}
	}
	f.Events = events
	var commands []RunCommand
	for _, c := range f.Commands {
		if !removed[c.RunID] {
			commands = append(commands, c)
		}
	}
	f.Commands = commands
	var sessionEvents []SessionEvent
	for _, e := range f.SessionEvents {
		if e.CreatedAt.Before(before) {
			continue
		}
		sessionEvents = append(sessionEvents, e)
	}
	f.SessionEvents = sessionEvents
	return s.persistRuns(f)
}

func (s *JSONStore) RunEventsAfter(afterSeq int64, limit int) ([]RunEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	out := make([]RunEvent, 0, limit)
	for _, e := range f.Events {
		if e.Seq <= afterSeq {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *JSONStore) LastGlobalRunEventSeq() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return 0, err
	}
	return f.NextSeq, nil
}
