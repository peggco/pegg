package session

import (
	"database/sql"
	"time"
)

const runEventPageSize = 500

func scanRun(row interface{ Scan(...any) error }) (*Run, error) {
	var (
		r          Run
		state      string
		cancel     int
		finishedAt sql.NullTime
	)
	if err := row.Scan(
		&r.ID, &r.SessionID, &state, &r.OwnerInstance, &r.OwnerPID,
		&r.AgentID, &r.AgentName, &r.Provider, &r.Model, &r.BaseSeq,
		&cancel, &r.Error, &r.StartedAt, &r.UpdatedAt, &finishedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRunNotFound
		}
		return nil, err
	}
	r.State = RunState(state)
	r.CancelRequested = cancel != 0
	if finishedAt.Valid {
		r.FinishedAt = finishedAt.Time
	}
	return &r, nil
}

const runSelectColumns = "id, session_id, state, owner_instance, owner_pid, agent_id, agent_name, provider, model, base_seq, cancel_requested, error, started_at, updated_at, finished_at"

func (s *SQLiteStore) CreateRun(r Run) error {
	_, err := s.db.Exec(
		"INSERT INTO session_runs ("+runSelectColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		r.ID, r.SessionID, string(r.State), r.OwnerInstance, r.OwnerPID,
		r.AgentID, r.AgentName, r.Provider, r.Model, r.BaseSeq,
		boolInt(r.CancelRequested), r.Error, r.StartedAt, r.UpdatedAt, nullTime(r.FinishedAt),
	)
	return err
}

func (s *SQLiteStore) UpdateRun(r Run) error {
	res, err := s.db.Exec(
		"UPDATE session_runs SET session_id = ?, state = ?, owner_instance = ?, owner_pid = ?, agent_id = ?, agent_name = ?, provider = ?, model = ?, base_seq = ?, cancel_requested = ?, error = ?, started_at = ?, updated_at = ?, finished_at = ? WHERE id = ?",
		r.SessionID, string(r.State), r.OwnerInstance, r.OwnerPID,
		r.AgentID, r.AgentName, r.Provider, r.Model, r.BaseSeq,
		boolInt(r.CancelRequested), r.Error, r.StartedAt, r.UpdatedAt, nullTime(r.FinishedAt), r.ID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRunNotFound
	}
	return nil
}

func (s *SQLiteStore) GetRun(id string) (*Run, error) {
	return scanRun(s.db.QueryRow("SELECT "+runSelectColumns+" FROM session_runs WHERE id = ?", id))
}

func (s *SQLiteStore) ActiveRun(sessionID string) (*Run, error) {
	return scanRun(s.db.QueryRow(
		"SELECT "+runSelectColumns+" FROM session_runs WHERE session_id = ? AND state = ? ORDER BY started_at DESC LIMIT 1",
		sessionID, string(RunStateRunning),
	))
}

func (s *SQLiteStore) ListRuns() ([]Run, error) {
	rows, err := s.db.Query("SELECT " + runSelectColumns + " FROM session_runs ORDER BY started_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *r)
	}
	return runs, rows.Err()
}

func (s *SQLiteStore) FinishRun(id string, state RunState, errMsg string) error {
	now := time.Now()
	res, err := s.db.Exec(
		"UPDATE session_runs SET state = ?, error = ?, updated_at = ?, finished_at = ? WHERE id = ?",
		string(state), errMsg, now, now, id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRunNotFound
	}
	return nil
}

func (s *SQLiteStore) RequestRunCancel(id string) error {
	res, err := s.db.Exec(
		"UPDATE session_runs SET cancel_requested = 1, updated_at = ? WHERE id = ? AND state = ?",
		time.Now(), id, string(RunStateRunning),
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRunNotFound
	}
	return nil
}

func (s *SQLiteStore) AppendRunEvents(events []RunEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, e := range events {
		created := e.CreatedAt
		if created.IsZero() {
			created = now
		}
		if _, err := tx.Exec(
			"INSERT INTO session_run_events (run_id, session_id, type, agent_id, parent_agent_id, agent_name, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			e.RunID, e.SessionID, e.Type, e.AgentID, e.ParentAgentID, e.AgentName, jsonString(e.Data), created,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		"UPDATE session_runs SET updated_at = ? WHERE id = ? AND state = ?",
		now, events[len(events)-1].RunID, string(RunStateRunning),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) RunEvents(runID string, afterSeq int64, limit int) ([]RunEvent, error) {
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	rows, err := s.db.Query(
		"SELECT seq, run_id, session_id, type, agent_id, parent_agent_id, agent_name, data, created_at FROM session_run_events WHERE run_id = ? AND seq > ? ORDER BY seq ASC LIMIT ?",
		runID, afterSeq, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []RunEvent
	for rows.Next() {
		var (
			e    RunEvent
			data string
		)
		if err := rows.Scan(&e.Seq, &e.RunID, &e.SessionID, &e.Type, &e.AgentID, &e.ParentAgentID, &e.AgentName, &data, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Data = rawJSON(data)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) LastRunEventSeq(runID string) (int64, error) {
	var seq int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(seq), 0) FROM session_run_events WHERE run_id = ?", runID).Scan(&seq)
	return seq, err
}

func (s *SQLiteStore) AddRunCommand(cmd RunCommand) error {
	created := cmd.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	_, err := s.db.Exec(
		"INSERT INTO session_run_commands (run_id, kind, payload, created_at) VALUES (?, ?, ?, ?)",
		cmd.RunID, cmd.Kind, jsonString(cmd.Payload), created,
	)
	return err
}

func (s *SQLiteStore) PendingRunCommands(runID string) ([]RunCommand, error) {
	rows, err := s.db.Query(
		"SELECT id, run_id, kind, payload, created_at FROM session_run_commands WHERE run_id = ? AND consumed_at IS NULL ORDER BY id ASC",
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []RunCommand
	for rows.Next() {
		var (
			c    RunCommand
			data string
		)
		if err := rows.Scan(&c.ID, &c.RunID, &c.Kind, &data, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Payload = rawJSON(data)
		cmds = append(cmds, c)
	}
	return cmds, rows.Err()
}

func (s *SQLiteStore) ConsumeRunCommand(id int64) error {
	_, err := s.db.Exec("UPDATE session_run_commands SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL", time.Now(), id)
	return err
}

func (s *SQLiteStore) PruneRuns(before time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query("SELECT id FROM session_runs WHERE state != ? AND finished_at IS NOT NULL AND finished_at < ?", string(RunStateRunning), before)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec("DELETE FROM session_run_events WHERE run_id = ?", id); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM session_run_commands WHERE run_id = ?", id); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM session_runs WHERE id = ?", id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM session_events WHERE created_at < ?", before); err != nil {
		return err
	}
	return tx.Commit()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (s *SQLiteStore) RunEventsAfter(afterSeq int64, limit int) ([]RunEvent, error) {
	if limit <= 0 || limit > runEventPageSize {
		limit = runEventPageSize
	}
	rows, err := s.db.Query(
		"SELECT seq, run_id, session_id, type, agent_id, parent_agent_id, agent_name, data, created_at FROM session_run_events WHERE seq > ? ORDER BY seq ASC LIMIT ?",
		afterSeq, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []RunEvent
	for rows.Next() {
		var (
			e    RunEvent
			data string
		)
		if err := rows.Scan(&e.Seq, &e.RunID, &e.SessionID, &e.Type, &e.AgentID, &e.ParentAgentID, &e.AgentName, &data, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Data = rawJSON(data)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) LastGlobalRunEventSeq() (int64, error) {
	var seq int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(seq), 0) FROM session_run_events").Scan(&seq)
	return seq, err
}
