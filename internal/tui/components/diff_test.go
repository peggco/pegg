package components

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/tui/styles"
)

func countKind(hunks []DiffHunk, kind byte) int {
	n := 0
	for _, h := range hunks {
		for _, l := range h.Lines {
			if l.Kind == kind {
				n++
			}
		}
	}
	return n
}

func TestComputeDiffMergesCloseHunks(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	new := "A\nb\nc\nd\nE\n"

	hunks := ComputeDiff(old, new)
	if len(hunks) != 1 {
		t.Fatalf("expected close changes in a single merged hunk, got %d", len(hunks))
	}

	seen := map[string]int{}
	for _, l := range hunks[0].Lines {
		if l.Kind == ' ' {
			seen[l.Text]++
		}
	}
	for text, n := range seen {
		if n > 1 {
			t.Errorf("context line %q repeated %d times", text, n)
		}
	}
	if countKind(hunks, '-') != 2 || countKind(hunks, '+') != 2 {
		t.Errorf("expected 2 removals and 2 additions, got -%d +%d", countKind(hunks, '-'), countKind(hunks, '+'))
	}
}

func TestComputeDiffSparseHunks(t *testing.T) {
	var oldB, newB strings.Builder
	for i := 0; i < 40; i++ {
		oldB.WriteString("line\n")
		newB.WriteString("line\n")
	}
	oldLines := strings.Split(oldB.String(), "\n")
	newLines := strings.Split(newB.String(), "\n")
	oldLines[2] = "old-head"
	newLines[2] = "new-head"
	oldLines[35] = "old-tail"
	newLines[35] = "new-tail"

	hunks := ComputeDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"))
	if len(hunks) != 2 {
		t.Fatalf("expected 2 separated hunks, got %d", len(hunks))
	}
	if hunks[0].OldStart != 1 {
		t.Errorf("first hunk OldStart = %d, want 1", hunks[0].OldStart)
	}
	if hunks[1].OldStart <= hunks[0].OldStart+hunks[0].OldCount {
		t.Errorf("second hunk should start after the first: %+v %+v", hunks[0], hunks[1])
	}
}

func TestComputeDiffLineNumbers(t *testing.T) {
	old := "1\n2\n3\n4\n5\n"
	new := "1\n2\nX\n4\n5\n"

	hunks := ComputeDiff(old, new)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	h := hunks[0]
	if h.OldStart != 1 || h.NewStart != 1 {
		t.Fatalf("hunk start = old %d new %d, want 1/1", h.OldStart, h.NewStart)
	}

	var removed, added *DiffLine
	for i := range h.Lines {
		switch h.Lines[i].Kind {
		case '-':
			removed = &h.Lines[i]
		case '+':
			added = &h.Lines[i]
		}
	}
	if removed == nil || added == nil {
		t.Fatal("expected a removal and an addition")
	}
	if removed.OldLine != 3 {
		t.Errorf("removed OldLine = %d, want 3", removed.OldLine)
	}
	if added.NewLine != 3 {
		t.Errorf("added NewLine = %d, want 3", added.NewLine)
	}

	var ctx5 *DiffLine
	for i := range h.Lines {
		if h.Lines[i].Kind == ' ' && h.Lines[i].Text == "5" {
			ctx5 = &h.Lines[i]
		}
	}
	if ctx5 == nil {
		t.Fatal("expected trailing context line \"5\"")
	}
	if ctx5.OldLine != 5 || ctx5.NewLine != 5 {
		t.Errorf("context \"5\" line numbers = old %d new %d, want 5/5", ctx5.OldLine, ctx5.NewLine)
	}
}

func TestComputeDiffLargeInputGuard(t *testing.T) {
	var oldB, newB strings.Builder
	for i := 0; i < 3000; i++ {
		oldB.WriteString("shared prefix line\n")
		newB.WriteString("shared prefix line\n")
	}
	oldB.WriteString("only-old\n")
	newB.WriteString("only-new\n")

	hunks := ComputeDiff(oldB.String(), newB.String())
	if !HasDiff(hunks) {
		t.Fatal("expected a diff for large input")
	}
	if countKind(hunks, '-') != 1 || countKind(hunks, '+') != 1 {
		t.Errorf("expected exactly one change pair, got -%d +%d", countKind(hunks, '-'), countKind(hunks, '+'))
	}
}

func TestComputeDiffExpandsTabs(t *testing.T) {
	hunks := ComputeDiff("a\tb\n", "a\tc\n")
	for _, h := range hunks {
		for _, l := range h.Lines {
			if strings.ContainsRune(l.Text, '\t') {
				t.Errorf("tab not expanded: %q", l.Text)
			}
		}
	}
}

func TestEditCardRendersUnifiedDiff(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")

	c := NewChat()
	it := &ChatItem{
		Kind:       ItemTool,
		ToolName:   "edit:main.go",
		ToolOutput: "ok",
		Diff:       ComputeDiff("a\nb\nc\nd\ne\n", "a\nB\nc\nd\nE\n"),
	}
	lines := c.editCardLines(it, 80)
	if len(lines) == 0 {
		t.Fatal("expected rendered diff card")
	}

	joined := make([]string, len(lines))
	for i, l := range lines {
		var b strings.Builder
		for _, cell := range l {
			b.WriteRune(cell.R)
		}
		joined[i] = b.String()
	}
	all := strings.Join(joined, "\n")

	if strings.Contains(all, "@@") {
		t.Error("diff should not include hunk headers")
	}
	if !strings.Contains(joined[0], "╭") || !strings.Contains(joined[len(joined)-1], "╯") {
		t.Errorf("diff should be bordered like the bash card, got %q / %q", joined[0], joined[len(joined)-1])
	}
	if !strings.Contains(joined[0], "Edit") && !strings.Contains(joined[0], "DIFF") && !strings.Contains(joined[0], "main.go") {
		t.Errorf("header should show the file path, got %q", joined[0])
	}

	adds, dels := 0, 0
	for _, s := range joined {
		if strings.Contains(s, "+") {
			adds++
		}
		if strings.Contains(s, "-") {
			dels++
		}
	}
	if adds < 1 || dels < 1 {
		t.Errorf("expected add and remove rows, got +%d -%d", adds, dels)
	}

	for li, l := range lines {
		if len(l) < 3 {
			continue
		}
		_, wantBg, _ := l[1].S.Decompose()
		for ci := 1; ci < len(l)-1; ci++ {
			if _, bg, _ := l[ci].S.Decompose(); bg != wantBg {
				t.Fatalf("line %d cell %d background mismatch", li, ci)
			}
		}
	}
}
