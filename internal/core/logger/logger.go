package logger

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ParseLevelName(s string) (Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN":
		return LevelWarn, true
	case "ERROR":
		return LevelError, true
	default:
		return LevelInfo, false
	}
}

func ParseLevel(s string) Level {
	if level, ok := ParseLevelName(s); ok {
		return level
	}
	return LevelInfo
}

type Record struct {
	ID        int64          `json:"id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Level     Level          `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func FormatRecord(rec Record) string {
	line := fmt.Sprintf("[%s] [%-5s] %s",
		rec.Timestamp.Format("2006-01-02 15:04:05"),
		rec.Level.String(),
		rec.Message,
	)
	if len(rec.Fields) == 0 {
		return line
	}

	keys := make([]string, 0, len(rec.Fields))
	for key := range rec.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(line)
	for _, key := range keys {
		b.WriteString(" ")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(fmt.Sprint(rec.Fields[key]))
	}
	return b.String()
}

type Handler interface {
	Write(rec Record) error
	Close() error
}

const maxBufferedRecords = 1000

type Logger struct {
	mu       sync.RWMutex
	minLevel Level
	handlers []Handler
	records  []Record
}

func New(minLevel Level, handlers ...Handler) *Logger {
	return &Logger{
		minLevel: minLevel,
		handlers: handlers,
		records:  make([]Record, 0, 64),
	}
}

func (l *Logger) AddHandler(h Handler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers = append(l.handlers, h)
}

func (l *Logger) MinLevel() Level {
	return l.minLevel
}

func (l *Logger) log(level Level, msg string) {
	l.logFields(level, msg, nil)
}

func (l *Logger) logFields(level Level, msg string, fields map[string]any) {
	if level < l.minLevel {
		return
	}

	rec := Record{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
	}
	if len(fields) > 0 {
		rec.Fields = make(map[string]any, len(fields))
		for key, value := range fields {
			rec.Fields[key] = value
		}
	}

	l.mu.Lock()
	l.records = append(l.records, rec)
	if len(l.records) > maxBufferedRecords {
		l.records = append([]Record(nil), l.records[len(l.records)-maxBufferedRecords:]...)
	}
	l.mu.Unlock()

	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, h := range l.handlers {
		_ = h.Write(rec)
	}
}

func (l *Logger) Records() []Record {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]Record, len(l.records))
	copy(out, l.records)
	return out
}

func (l *Logger) Debug(msg string) { l.log(LevelDebug, msg) }
func (l *Logger) Info(msg string)  { l.log(LevelInfo, msg) }
func (l *Logger) Warn(msg string)  { l.log(LevelWarn, msg) }
func (l *Logger) Error(msg string) { l.log(LevelError, msg) }

func (l *Logger) Fdebug(msg string, args ...any) { l.log(LevelDebug, fmt.Sprintf(msg, args...)) }
func (l *Logger) Finfo(msg string, args ...any)  { l.log(LevelInfo, fmt.Sprintf(msg, args...)) }
func (l *Logger) Fwarn(msg string, args ...any)  { l.log(LevelWarn, fmt.Sprintf(msg, args...)) }
func (l *Logger) Ferror(msg string, args ...any) { l.log(LevelError, fmt.Sprintf(msg, args...)) }

func (l *Logger) DebugFields(msg string, fields map[string]any) { l.logFields(LevelDebug, msg, fields) }
func (l *Logger) InfoFields(msg string, fields map[string]any)  { l.logFields(LevelInfo, msg, fields) }
func (l *Logger) WarnFields(msg string, fields map[string]any)  { l.logFields(LevelWarn, msg, fields) }
func (l *Logger) ErrorFields(msg string, fields map[string]any) { l.logFields(LevelError, msg, fields) }

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, h := range l.handlers {
		_ = h.Close()
	}
	return nil
}
