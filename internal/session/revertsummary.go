package session

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/peggco/pegg/internal/llm"
)

const (
	RevertStatusReverted = "reverted"
	RevertStatusConflict = "conflict"
	RevertStatusSkipped  = "skipped"
)

type RevertFileChange struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

type RevertSummary struct {
	Files          []RevertFileChange `json:"files"`
	TotalAdditions int                `json:"totalAdditions"`
	TotalDeletions int                `json:"totalDeletions"`
}

func SummarizeRevert(messages []Message) RevertSummary {
	byPath := make(map[string]*RevertFileChange)
	for _, msg := range messages {
		if msg.Role != llm.RoleAssistant {
			continue
		}
		for _, call := range msg.ToolCalls {
			path, additions, deletions, ok := revertToolChange(call)
			if !ok {
				continue
			}
			change, exists := byPath[path]
			if !exists {
				change = &RevertFileChange{Path: path, Status: RevertStatusReverted}
				byPath[path] = change
			}
			change.Additions += additions
			change.Deletions += deletions
		}
	}

	files := make([]RevertFileChange, 0, len(byPath))
	totalAdditions := 0
	totalDeletions := 0
	for _, change := range byPath {
		files = append(files, *change)
		totalAdditions += change.Additions
		totalDeletions += change.Deletions
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return RevertSummary{
		Files:          files,
		TotalAdditions: totalAdditions,
		TotalDeletions: totalDeletions,
	}
}

func revertToolChange(call llm.ToolCall) (string, int, int, bool) {
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", 0, 0, false
	}
	path := firstStringArg(args, "filePath", "file_path", "path", "file", "target")
	if path == "" {
		return "", 0, 0, false
	}
	switch call.Function.Name {
	case "write":
		newText := firstStringArg(args, "content", "text")
		if newText == "" {
			return "", 0, 0, false
		}
		additions, _ := lineChangeCounts("", newText)
		return path, additions, 0, true
	case "edit":
		oldText := firstStringArg(args, "oldString", "old_string")
		newText := firstStringArg(args, "newString", "new_string")
		if oldText == "" && newText == "" {
			return "", 0, 0, false
		}
		additions, deletions := lineChangeCounts(oldText, newText)
		return path, additions, deletions, true
	default:
		return "", 0, 0, false
	}
}

func firstStringArg(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := args[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func lineChangeCounts(oldText, newText string) (int, int) {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	if len(oldLines) == 0 {
		return len(newLines), 0
	}
	if len(newLines) == 0 {
		return 0, len(oldLines)
	}
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
	if len(oldMid) == 0 {
		return len(newMid), 0
	}
	if len(newMid) == 0 {
		return 0, len(oldMid)
	}
	if len(oldMid)*len(newMid) > 1<<20 {
		return len(newMid), len(oldMid)
	}
	common := lineLCS(oldMid, newMid)
	return len(newMid) - common, len(oldMid) - common
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func lineLCS(a, b []string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
		for j := range curr {
			curr[j] = 0
		}
	}
	return prev[len(b)]
}
