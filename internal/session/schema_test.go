package session

import (
	"database/sql"
	"testing"
)

func TestMigrateSchemaAddsMissingColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE session_messages (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, seq INTEGER NOT NULL, role TEXT NOT NULL, created_at DATETIME NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := migrateSchema(db); err != nil {
			t.Fatalf("migrateSchema run %d: %v", i+1, err)
		}
	}

	assertHasColumns(t, db, "sessions", sessionColumns)
	assertHasColumns(t, db, "session_messages", messageColumns)
	assertHasColumns(t, db, "session_snapshots", snapshotColumns)
}

func assertHasColumns(t *testing.T, db *sql.DB, table string, want []columnDef) {
	t.Helper()
	got, err := tableColumns(db, table)
	if err != nil {
		t.Fatalf("tableColumns(%s): %v", table, err)
	}
	for _, c := range want {
		if !got[c.name] {
			t.Errorf("table %s missing column %q", table, c.name)
		}
	}
}
