//go:build gui

package main

import (
	"strings"
	"testing"
)

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := maskSecret("short"); got != "••••" {
		t.Fatalf("short = %q", got)
	}
	if got := maskSecret("1234567890abcdef"); got != "1234…cdef" {
		t.Fatalf("long = %q", got)
	}
}

func TestIsTeamProviderName(t *testing.T) {
	for _, name := range []string{"pegg-team", "pegg-team Acme"} {
		if !isTeamProviderName(name) {
			t.Fatalf("%q should be a team provider", name)
		}
	}
	for _, name := range []string{"openai", "pegg-teamx", ""} {
		if isTeamProviderName(name) {
			t.Fatalf("%q should not be a team provider", name)
		}
	}
}

func TestAppendLogKeepsTail(t *testing.T) {
	var logs []string
	for i := 0; i < 5; i++ {
		logs = appendLog(logs, string(rune('a'+i)), 3)
	}
	if strings.Join(logs, "") != "cde" {
		t.Fatalf("logs = %v", logs)
	}
}

func TestLogWriterSplitsLines(t *testing.T) {
	var lines []string
	w := newLogWriter(func(line string) { lines = append(lines, line) })
	_, _ = w.Write([]byte("one\ntwo\nthr"))
	_, _ = w.Write([]byte("ee\n"))
	want := []string{"one", "two", "three"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
}
