package logger

import (
	"strings"
	"testing"
	"time"

	"github.com/peggco/pegg/internal/core/config"
)

func TestParseLevelCaseInsensitive(t *testing.T) {
	cases := map[string]Level{
		"debug":  LevelDebug,
		"DEBUG":  LevelDebug,
		" Info ": LevelInfo,
		"warn":   LevelWarn,
		"ERROR":  LevelError,
	}
	for input, want := range cases {
		if got := ParseLevel(input); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
	if got := ParseLevel("bogus"); got != LevelInfo {
		t.Errorf("ParseLevel(bogus) = %v, want INFO", got)
	}
}

func TestNewFromConfigDefaultLevel(t *testing.T) {
	t.Setenv("PEGG_DEBUG", "")
	t.Setenv("PEGG_LOG_LEVEL", "")

	l, err := NewFromConfig(config.LoggerConfig{Driver: DriverConsole})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if got := l.MinLevel(); got != LevelInfo {
		t.Fatalf("min level = %v, want INFO", got)
	}
}

func TestNewFromConfigLevelFromConfig(t *testing.T) {
	t.Setenv("PEGG_DEBUG", "")
	t.Setenv("PEGG_LOG_LEVEL", "")

	l, err := NewFromConfig(config.LoggerConfig{Driver: DriverConsole, Level: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if got := l.MinLevel(); got != LevelWarn {
		t.Fatalf("min level = %v, want WARN", got)
	}
}

func TestNewFromConfigDebugEnvOverride(t *testing.T) {
	t.Setenv("PEGG_DEBUG", "1")
	t.Setenv("PEGG_LOG_LEVEL", "")

	l, err := NewFromConfig(config.LoggerConfig{Driver: DriverConsole, Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if got := l.MinLevel(); got != LevelDebug {
		t.Fatalf("min level = %v, want DEBUG", got)
	}
}

func TestNewFromConfigLogLevelEnv(t *testing.T) {
	t.Setenv("PEGG_DEBUG", "")
	t.Setenv("PEGG_LOG_LEVEL", "debug")

	l, err := NewFromConfig(config.LoggerConfig{Driver: DriverConsole, Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if got := l.MinLevel(); got != LevelDebug {
		t.Fatalf("min level = %v, want DEBUG", got)
	}
}

func TestLevelFiltering(t *testing.T) {
	h := &captureHandler{}
	l := New(LevelWarn, h)

	l.Debug("debug")
	l.Info("info")
	l.Warn("warn")
	l.Error("error")

	if len(h.lines) != 2 || h.lines[0] != "warn" || h.lines[1] != "error" {
		t.Fatalf("lines = %v, want [warn error]", h.lines)
	}
}

func TestFormatRecordFields(t *testing.T) {
	rec := Record{
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "llm: model prices cached",
		Fields: map[string]any{
			"provider": "openai",
			"entries":  12,
		},
	}

	got := FormatRecord(rec)
	want := "[2024-01-02 03:04:05] [INFO ] llm: model prices cached entries=12 provider=openai"
	if got != want {
		t.Fatalf("format = %q, want %q", got, want)
	}
	if strings.Contains(FormatRecord(Record{Level: LevelInfo, Message: "plain"}), "=") {
		t.Fatal("record without fields should not render fields")
	}
}

func TestFieldsAreCopied(t *testing.T) {
	l := New(LevelDebug)
	fields := map[string]any{"provider": "openai"}

	l.InfoFields("llm: provider loaded", fields)
	fields["provider"] = "mutated"

	recs := l.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	if got := recs[0].Fields["provider"]; got != "openai" {
		t.Fatalf("field = %v, want openai", got)
	}
}
