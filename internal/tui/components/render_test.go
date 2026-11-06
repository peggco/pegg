package components

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func wrappedStrings(in string, width int) []string {
	lines := WrapText(in, tcell.StyleDefault, width)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(lineText(l), " ")
	}
	return out
}

func TestWrapTextBreaksAtWordBoundary(t *testing.T) {
	got := wrappedStrings("hello tasarım", 10)
	want := []string{"hello", "tasarım"}
	if len(got) != len(want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}

func TestWrapTextNeverSplitsWordsThatFit(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	lines := wrappedStrings(text, 12)
	for _, l := range lines {
		for _, w := range strings.Fields(l) {
			if !strings.Contains(text, w) {
				t.Fatalf("line %q split a word into %q", l, w)
			}
		}
	}
	if strings.Join(lines, " ") != text {
		t.Fatalf("rejoined text = %q, want %q", strings.Join(lines, " "), text)
	}
}

func TestWrapTextHardBreaksOverlongWord(t *testing.T) {
	got := wrappedStrings("supercalifragilistic", 6)
	if len(got) < 2 {
		t.Fatalf("expected hard wrap, got %q", got)
	}
	for _, l := range got {
		if DisplayWidth(l) > 6 {
			t.Fatalf("line %q exceeds width 6", l)
		}
	}
	if strings.Join(got, "") != "supercalifragilistic" {
		t.Fatalf("hard wrap lost characters: %q", got)
	}
}

func TestWrapTextPreservesNewlines(t *testing.T) {
	got := wrappedStrings("first\nsecond", 40)
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("lines = %q, want [first second]", got)
	}
}

func TestWrapTextPreservesBlankLines(t *testing.T) {
	got := wrappedStrings("a\n\n\nb", 40)
	want := []string{"a", "", "", "b"}
	if len(got) != len(want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines = %q, want %q", got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0ms"},
		{500 * time.Microsecond, "<1ms"},
		{250 * time.Millisecond, "250ms"},
		{1500 * time.Millisecond, "1.5s"},
	}
	for _, c := range cases {
		if got := formatDuration(c.in); got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
