package session

import "time"

func (s *SQLiteStore) AppendSessionEvent(ev SessionEvent) error {
	created := ev.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	_, err := s.db.Exec(
		"INSERT INTO session_events (type, session_id, data, origin, created_at) VALUES (?, ?, ?, ?, ?)",
		ev.Type, ev.SessionID, jsonString(ev.Data), ev.Origin, created,
	)
	return err
}

func (s *SQLiteStore) SessionEvents(afterSeq int64, limit int) ([]SessionEvent, error) {
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	rows, err := s.db.Query(
		"SELECT seq, type, session_id, data, origin, created_at FROM session_events WHERE seq > ? ORDER BY seq ASC LIMIT ?",
		afterSeq, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []SessionEvent
	for rows.Next() {
		var (
			e    SessionEvent
			data string
		)
		if err := rows.Scan(&e.Seq, &e.Type, &e.SessionID, &data, &e.Origin, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Data = rawJSON(data)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) LastSessionEventSeq() (int64, error) {
	var seq int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(seq), 0) FROM session_events").Scan(&seq)
	return seq, err
}
