package components

import "strings"

type DiffLine struct {
	Kind    byte
	OldLine int
	NewLine int
	Text    string
}

type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int

	Lines []DiffLine
}

const (
	diffContext  = 3
	diffMaxCells = 2_000_000
	diffTabWidth = 4
)

func ComputeDiff(oldText, newText string) []DiffHunk {
	if oldText == newText {
		return nil
	}

	oldLines := splitDiffLines(oldText)
	newLines := splitDiffLines(newText)

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}

	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	oldMid := oldLines[prefix : len(oldLines)-suffix]
	newMid := newLines[prefix : len(newLines)-suffix]

	var mid []diffOp
	if len(oldMid)*len(newMid) > diffMaxCells {
		mid = coarseDiff(oldMid, newMid)
	} else {
		mid = diffLines(oldMid, newMid)
	}

	ops := make([]diffOp, 0, len(oldLines)+len(newLines))
	for i := 0; i < prefix; i++ {
		ops = append(ops, diffOp{Kind: ' ', Text: oldLines[i]})
	}
	ops = append(ops, mid...)
	for i := len(oldLines) - suffix; i < len(oldLines); i++ {
		ops = append(ops, diffOp{Kind: ' ', Text: oldLines[i]})
	}

	return buildHunks(ops, diffContext)
}

type diffOp struct {
	Kind byte
	Text string
}

func diffLines(oldLines, newLines []string) []diffOp {
	n := len(oldLines)
	m := len(newLines)

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case oldLines[i] == newLines[j]:
			ops = append(ops, diffOp{Kind: ' ', Text: oldLines[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{Kind: '-', Text: oldLines[i]})
			i++
		default:
			ops = append(ops, diffOp{Kind: '+', Text: newLines[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{Kind: '-', Text: oldLines[i]})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{Kind: '+', Text: newLines[j]})
		j++
	}
	return ops
}

func coarseDiff(oldLines, newLines []string) []diffOp {
	ops := make([]diffOp, 0, len(oldLines)+len(newLines))
	for _, l := range oldLines {
		ops = append(ops, diffOp{Kind: '-', Text: l})
	}
	for _, l := range newLines {
		ops = append(ops, diffOp{Kind: '+', Text: l})
	}
	return ops
}

func buildHunks(ops []diffOp, context int) []DiffHunk {
	n := len(ops)
	if n == 0 {
		return nil
	}

	oldNo := make([]int, n+1)
	newNo := make([]int, n+1)
	ol, nl := 1, 1
	for i := 0; i < n; i++ {
		oldNo[i], newNo[i] = ol, nl
		switch ops[i].Kind {
		case '-':
			ol++
		case '+':
			nl++
		default:
			ol++
			nl++
		}
	}
	oldNo[n], newNo[n] = ol, nl

	type span struct{ start, end int }
	var spans []span
	for i := 0; i < n; {
		if ops[i].Kind == ' ' {
			i++
			continue
		}
		j := i
		for j < n && ops[j].Kind != ' ' {
			j++
		}
		start := i - context
		if start < 0 {
			start = 0
		}
		end := j + context
		if end > n {
			end = n
		}
		if len(spans) > 0 && start <= spans[len(spans)-1].end {
			spans[len(spans)-1].end = end
		} else {
			spans = append(spans, span{start: start, end: end})
		}
		i = j
	}

	hunks := make([]DiffHunk, 0, len(spans))
	for _, sp := range spans {
		h := DiffHunk{
			OldStart: oldNo[sp.start],
			NewStart: newNo[sp.start],
		}
		for k := sp.start; k < sp.end; k++ {
			dl := DiffLine{Kind: ops[k].Kind, Text: ops[k].Text}
			switch ops[k].Kind {
			case '-':
				dl.OldLine = oldNo[k]
				h.OldCount++
			case '+':
				dl.NewLine = newNo[k]
				h.NewCount++
			default:
				dl.OldLine = oldNo[k]
				dl.NewLine = newNo[k]
				h.OldCount++
				h.NewCount++
			}
			h.Lines = append(h.Lines, dl)
		}
		hunks = append(hunks, h)
	}
	return hunks
}

func splitDiffLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.IndexByte(l, '\t') >= 0 {
			lines[i] = expandDiffTabs(l)
		}
	}
	return lines
}

func expandDiffTabs(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := diffTabWidth - col%diffTabWidth
			for i := 0; i < n; i++ {
				b.WriteByte(' ')
			}
			col += n
			continue
		}
		b.WriteRune(r)
		col += cellWidth(r)
	}
	return b.String()
}

func HasDiff(hunks []DiffHunk) bool {
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if line.Kind != ' ' {
				return true
			}
		}
	}
	return false
}
