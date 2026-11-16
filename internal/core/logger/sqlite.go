package logger

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/utils/query"
	_ "modernc.org/sqlite"
)

const DriverSQLite = "sqlite"

func init() {
	RegisterDriver(DriverSQLite, func(cfg config.LoggerConfig) (Handler, error) {
		return NewSQLiteHandler(cfg)
	})
}

var createTableSQL = `
CREATE TABLE IF NOT EXISTS logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME,
	level TEXT,
	message TEXT,
	fields TEXT
);`

type SQLiteHandler struct {
	db          *sql.DB
	maxLogCount int
	insertCount int64
	mu          sync.Mutex
	builder     *query.Builder
}

func NewSQLiteHandler(cfg config.LoggerConfig) (*SQLiteHandler, error) {
	dbPath, err := config.GetConfigPath("logs.db")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("logger: create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}

	return newSQLiteHandler(db, cfg.MaxLogCount)
}

func sqliteDSN(path string) string {
	return "file:" + filepath.ToSlash(path) +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

func newSQLiteHandler(db *sql.DB, maxLogCount int) (*SQLiteHandler, error) {
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureFieldsColumn(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteHandler{
		db:          db,
		maxLogCount: maxLogCount,
		insertCount: 0,
		builder: query.NewBuilder("logs", "id", "timestamp", "level", "message", "fields").
			DefaultSort("timestamp", query.Desc),
	}, nil
}

func ensureFieldsColumn(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(logs)")
	if err != nil {
		return fmt.Errorf("logger: inspect logs table: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
			defaultV  sql.NullString
			primary   int
		)
		if err := rows.Scan(&cid, &name, &columnTyp, &notNull, &defaultV, &primary); err != nil {
			return fmt.Errorf("logger: inspect logs table: %w", err)
		}
		if name == "fields" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("logger: inspect logs table: %w", err)
	}

	if _, err := db.Exec("ALTER TABLE logs ADD COLUMN fields TEXT"); err != nil {
		return fmt.Errorf("logger: add fields column: %w", err)
	}
	return nil
}

func (s *SQLiteHandler) Write(rec Record) error {
	var fields any
	if len(rec.Fields) > 0 {
		data, err := json.Marshal(rec.Fields)
		if err != nil {
			return fmt.Errorf("logger: marshal fields: %w", err)
		}
		fields = string(data)
	}

	_, err := s.db.Exec("INSERT INTO logs (timestamp, level, message, fields) VALUES (?, ?, ?, ?)",
		rec.Timestamp, rec.Level.String(), rec.Message, fields)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.insertCount++
	shouldCleanup := s.insertCount%100 == 0
	s.mu.Unlock()

	if shouldCleanup {
		go s.cleanupOldLogs()
	}

	return nil
}

func (s *SQLiteHandler) cleanupOldLogs() {
	if s.maxLogCount <= 0 {
		return
	}

	q := `DELETE FROM logs WHERE id NOT IN (
        SELECT id FROM logs ORDER BY id DESC LIMIT ?
    )`
	_, _ = s.db.Exec(q, s.maxLogCount)
}

func (s *SQLiteHandler) Close() error {
	return s.db.Close()
}

func (s *SQLiteHandler) Query(q query.Query) ([]Record, int, error) {
	built, err := s.builder.Build(q)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := s.db.QueryRow(built.CountSQL, built.CountArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(built.SelectSQL, built.SelectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	records := make([]Record, 0, q.Page.Size)
	for rows.Next() {
		var rec Record
		var level string
		var fields sql.NullString
		if err := rows.Scan(&rec.ID, &rec.Timestamp, &level, &rec.Message, &fields); err != nil {
			return nil, 0, err
		}
		rec.Level = ParseLevel(level)
		if fields.Valid && fields.String != "" {
			_ = json.Unmarshal([]byte(fields.String), &rec.Fields)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
