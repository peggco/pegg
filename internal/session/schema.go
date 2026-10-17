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

	sessionSchemas = []tableSchema{
		{name: "sessions", columns: sessionColumns},
		{name: "session_messages", columns: messageColumns},
		{name: "session_snapshots", columns: snapshotColumns},
	}

	sessionIndexes = []string{
		"CREATE INDEX IF NOT EXISTS idx_session_messages_session_seq ON session_messages (session_id, seq)",
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
