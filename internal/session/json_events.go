package session

import "time"

func (s *JSONStore) AppendSessionEvent(ev SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.loadRuns()
	if err != nil {
		return err
	}
	f.NextSessionEvent++
	ev.Seq = f.NextSessionEvent
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	f.SessionEvents = append(f.SessionEvents, ev)
	return s.persistRuns(f)
}

func (s *JSONStore) SessionEvents(afterSeq int64, limit int) ([]SessionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	out := make([]SessionEvent, 0, limit)
	for _, e := range f.SessionEvents {
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

func (s *JSONStore) LastSessionEventSeq() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.loadRuns()
	if err != nil {
		return 0, err
	}
	return f.NextSessionEvent, nil
}
