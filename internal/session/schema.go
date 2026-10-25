package session

import (
	"database/sql"
	"strings"
)

type columnDef struct {
	name string
	def  string
}

type tableSchema struct {
	name    string
	columns []columnDef
}

func columnNames(cols []columnDef) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.name
	}
	return out
}

func (t tableSchema) createSQL() string {
	defs := make([]string, len(t.columns))
	for i, c := range t.columns {
		defs[i] = c.name + " " + c.def
	}
	return "CREATE TABLE IF NOT EXISTS " + t.name + " (" + strings.Join(defs, ", ") + ")"
}

var (
	sessionColumns = []columnDef{
		{"id", "TEXT PRIMARY KEY"},
		{"title", "TEXT NOT NULL DEFAULT ''"},
		{"provider", "TEXT NOT NULL DEFAULT ''"},
		{"model", "TEXT NOT NULL DEFAULT ''"},
		{"reasoning_effort", "TEXT NOT NULL DEFAULT ''"},
		{"project_dir", "TEXT NOT NULL DEFAULT ''"},
		{"parent_id", "TEXT NOT NULL DEFAULT ''"},
		{"compaction_parent_id", "TEXT NOT NULL DEFAULT ''"},
		{"is_subagent", "INTEGER NOT NULL DEFAULT 0"},
		{"created_at", "DATETIME NOT NULL"},
		{"updated_at", "DATETIME NOT NULL"},
		{"usage", "TEXT NOT NULL DEFAULT '{}'"},
		{"mode", "TEXT NOT NULL DEFAULT ''"},
	}

	messageColumns = []columnDef{
		{"id", "TEXT PRIMARY KEY"},
		{"session_id", "TEXT NOT NULL"},
		{"seq", "INTEGER NOT NULL"},
		{"role", "TEXT NOT NULL"},
		{"content", "TEXT NOT NULL DEFAULT ''"},
		{"reasoning", "TEXT NOT NULL DEFAULT ''"},
		{"name", "TEXT NOT NULL DEFAULT ''"},
		{"tool_call_id", "TEXT NOT NULL DEFAULT ''"},
		{"tool_calls", "TEXT NOT NULL DEFAULT ''"},
		{"created_at", "DATETIME NOT NULL"},
	}

	snapshotColumns = []columnDef{
		{"id", "TEXT PRIMARY KEY"},
		{"session_id", "TEXT NOT NULL"},
		{"head_message_id", "TEXT NOT NULL"},
		{"created_at", "DATETIME NOT NULL"},
		{"messages", "TEXT NOT NULL"},
	}

	runColumns = []columnDef{
		{"id", "TEXT PRIMARY KEY"},
		{"session_id", "TEXT NOT NULL"},
		{"state", "TEXT NOT NULL"},
		{"owner_instance", "TEXT NOT NULL DEFAULT ''"},
		{"owner_pid", "INTEGER NOT NULL DEFAULT 0"},
		{"agent_id", "TEXT NOT NULL DEFAULT ''"},
		{"agent_name", "TEXT NOT NULL DEFAULT ''"},
		{"provider", "TEXT NOT NULL DEFAULT ''"},
		{"model", "TEXT NOT NULL DEFAULT ''"},
		{"base_seq", "INTEGER NOT NULL DEFAULT 0"},
		{"cancel_requested", "INTEGER NOT NULL DEFAULT 0"},
		{"error", "TEXT NOT NULL DEFAULT ''"},
		{"started_at", "DATETIME NOT NULL"},
		{"updated_at", "DATETIME NOT NULL"},
		{"finished_at", "DATETIME"},
	}

	runEventColumns = []columnDef{
		{"seq", "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{"run_id", "TEXT NOT NULL"},
		{"session_id", "TEXT NOT NULL"},
		{"type", "TEXT NOT NULL"},
		{"agent_id", "TEXT NOT NULL DEFAULT ''"},
		{"parent_agent_id", "TEXT NOT NULL DEFAULT ''"},
		{"agent_name", "TEXT NOT NULL DEFAULT ''"},
		{"data", "TEXT NOT NULL DEFAULT '{}'"},
		{"created_at", "DATETIME NOT NULL"},
	}

	sessionEventColumns = []columnDef{
		{"seq", "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{"type", "TEXT NOT NULL"},
		{"session_id", "TEXT NOT NULL"},
		{"data", "TEXT NOT NULL DEFAULT '{}'"},
		{"origin", "TEXT NOT NULL DEFAULT ''"},
		{"created_at", "DATETIME NOT NULL"},
	}

	runCommandColumns = []columnDef{
		{"id", "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{"run_id", "TEXT NOT NULL"},
		{"kind", "TEXT NOT NULL"},
		{"payload", "TEXT NOT NULL DEFAULT '{}'"},
		{"created_at", "DATETIME NOT NULL"},
		{"consumed_at", "DATETIME"},
	}

	sessionSchemas = []tableSchema{
		{name: "sessions", columns: sessionColumns},
		{name: "session_messages", columns: messageColumns},
		{name: "session_snapshots", columns: snapshotColumns},
		{name: "session_runs", columns: runColumns},
		{name: "session_run_events", columns: runEventColumns},
		{name: "session_run_commands", columns: runCommandColumns},
		{name: "session_events", columns: sessionEventColumns},
	}

	sessionIndexes = []string{
		"CREATE INDEX IF NOT EXISTS idx_session_messages_session_seq ON session_messages (session_id, seq)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_session_runs_active ON session_runs (session_id) WHERE state = 'running'",
		"CREATE INDEX IF NOT EXISTS idx_session_runs_state ON session_runs (state)",
		"CREATE INDEX IF NOT EXISTS idx_session_run_events_run_seq ON session_run_events (run_id, seq)",
		"CREATE INDEX IF NOT EXISTS idx_session_run_events_session_seq ON session_run_events (session_id, seq)",
		"CREATE INDEX IF NOT EXISTS idx_session_run_commands_pending ON session_run_commands (run_id, consumed_at)",
		"CREATE INDEX IF NOT EXISTS idx_session_events_seq ON session_events (seq)",
	}
)

func migrateSchema(db *sql.DB) error {
	for _, t := range sessionSchemas {
		if _, err := db.Exec(t.createSQL()); err != nil {
			return err
		}
		if err := addMissingColumns(db, t); err != nil {
			return err
		}
	}
	for _, idx := range sessionIndexes {
		if _, err := db.Exec(idx); err != nil {
			return err
		}
	}
	return nil
}

func addMissingColumns(db *sql.DB, t tableSchema) error {
	existing, err := tableColumns(db, t.name)
	if err != nil {
		return err
	}
	for _, c := range t.columns {
		if existing[c.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE " + t.name + " ADD COLUMN " + c.name + " " + c.def); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cols, nil
}
