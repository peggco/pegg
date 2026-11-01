package search

import (
	"reflect"
	"strings"
	"testing"
)

func TestScore(t *testing.T) {
	tests := []struct {
		query    string
		text     string
		wantZero bool
		wantNeg  bool
	}{
		{query: "", text: "anything", wantZero: true},
		{query: "gpt", text: "gpt-4"},
		{query: "gpt", text: "chatgpt-4"},
		{query: "gpt", text: "gpt"},
		{query: "gpt", text: "GPT"},
		{query: "gp", text: "getProfile"},
		{query: "abc", text: "xyz", wantNeg: true},
		{query: "gpt", text: "claude-3", wantNeg: true},
		{query: "gpt-4", text: "gpt-4"},
		{query: "g4", text: "gpt-4"},
		{query: "g4", text: "gpt-4-turbo"},
	}

	for _, tt := range tests {
		score := Score(tt.query, tt.text)
		if tt.wantZero && score != 0 {
			t.Errorf("Score(%q, %q) = %d, want 0", tt.query, tt.text, score)
		}
		if tt.wantNeg && score >= 0 {
			t.Errorf("Score(%q, %q) = %d, want negative", tt.query, tt.text, score)
		}
		if !tt.wantZero && !tt.wantNeg && score <= 0 {
			t.Errorf("Score(%q, %q) = %d, want positive", tt.query, tt.text, score)
		}
	}
}

func TestPrefixHigherThanContains(t *testing.T) {
	query := "gpt"
	prefix := Score(query, "gpt-4")
	contains := Score(query, "chatgpt-4")

	if prefix <= contains {
		t.Errorf("Prefix score %d should be higher than contains score %d", prefix, contains)
	}
}

func TestExactHigherThanPrefix(t *testing.T) {
	query := "gpt"
	exact := Score(query, "gpt")
	prefix := Score(query, "gpt-4")

	if exact <= prefix {
		t.Errorf("Exact score %d should be higher than prefix score %d", exact, prefix)
	}
}

func TestCamelCaseMatch(t *testing.T) {
	score1 := Score("gp", "getProfile")
	score2 := Score("gp", "gearsProtocol")

	if score1 <= 0 {
		t.Errorf("CamelCase match should score positive, got %d", score1)
	}
	if score2 <= 0 {
		t.Errorf("CamelCase match should score positive, got %d", score2)
	}
}

func TestWordBoundaryMatch(t *testing.T) {
	score1 := Score("gp", "get-profile")
	score2 := Score("gp", "get_profile")
	score3 := Score("gp", "get/profile")

	for _, s := range []int{score1, score2, score3} {
		if s <= 0 {
			t.Errorf("Word boundary match should score positive, got %d", s)
		}
	}
}

func TestFind(t *testing.T) {
	query := "gpt"
	items := []string{"gpt-4", "gpt-4-turbo", "chatgpt-4", "claude-3", "gemini-pro"}

	matches := Find(query, items)

	if len(matches) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(matches))
	}

	for _, m := range matches {
		if m.Score <= 0 {
			t.Errorf("Expected positive score, got %d", m.Score)
		}
	}
}

func TestFindSortedByScore(t *testing.T) {
	query := "gpt"
	items := []string{"chatgpt-4", "gpt-4", "gpt-4-turbo"}

	matches := Find(query, items)

	if len(matches) < 2 {
		t.Fatalf("Expected at least 2 matches, got %d", len(matches))
	}

	for i := 1; i < len(matches); i++ {
		if matches[i].Score > matches[i-1].Score {
			t.Errorf("Matches not sorted: score %d at index %d > score %d at index %d",
				matches[i].Score, i, matches[i-1].Score, i-1)
		}
	}
}

func TestFindPositions(t *testing.T) {
	query := "gpt"
	text := "gpt-4"
	matches := Find(query, []string{text})

	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}

	m := matches[0]
	if len(m.Positions) != 3 {
		t.Errorf("Expected 3 positions, got %d", len(m.Positions))
	}

	expected := []int{0, 1, 2}
	if !reflect.DeepEqual(m.Positions, expected) {
		t.Errorf("Expected positions %v, got %v", expected, m.Positions)
	}
}

func TestFindEmptyQuery(t *testing.T) {
	items := []string{"a", "b", "c"}
	matches := Find("", items)

	if len(matches) != 3 {
		t.Errorf("Expected 3 matches for empty query, got %d", len(matches))
	}
}

func TestFindNoMatches(t *testing.T) {
	query := "xyz"
	items := []string{"abc", "def", "ghi"}

	matches := Find(query, items)

	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

type testItem struct {
	label  string
	detail string
}

func TestFilter(t *testing.T) {
	items := []testItem{
		{label: "gpt-4", detail: "openai"},
		{label: "gpt-4-turbo", detail: "openai"},
		{label: "claude-3", detail: "anthropic"},
		{label: "gemini-pro", detail: "google"},
	}

	result := Filter("gpt", items, func(i testItem) []string {
		return []string{i.label, i.detail}
	})

	if len(result) != 2 {
		t.Errorf("Expected 2 filtered items, got %d", len(result))
	}

	for _, item := range result {
		if item.detail != "openai" {
			t.Errorf("Expected openai provider, got %s", item.detail)
		}
	}
}

func TestFilterEmptyQuery(t *testing.T) {
	items := []testItem{
		{label: "a", detail: "1"},
		{label: "b", detail: "2"},
	}

	result := Filter("", items, func(i testItem) []string {
		return []string{i.label, i.detail}
	})

	if len(result) != 2 {
		t.Errorf("Expected all items for empty query, got %d", len(result))
	}
}

func TestFilterDetailMatch(t *testing.T) {
	items := []testItem{
		{label: "model-1", detail: "openai"},
		{label: "model-2", detail: "anthropic"},
	}

	result := Filter("openai", items, func(i testItem) []string {
		return []string{i.label, i.detail}
	})

	if len(result) != 1 {
		t.Errorf("Expected 1 match on detail, got %d", len(result))
	}
	if result[0].label != "model-1" {
		t.Errorf("Expected model-1, got %s", result[0].label)
	}
}

func TestCaseInsensitive(t *testing.T) {
	score1 := Score("GPT", "gpt-4")
	score2 := Score("gpt", "GPT-4")

	if score1 <= 0 || score2 <= 0 {
		t.Errorf("Case insensitive matching failed: scores %d, %d", score1, score2)
	}
}

func TestConsecutiveBonus(t *testing.T) {
	consecutive := Score("abc", "abcdef")
	gapped := Score("abc", "a-b-c")

	if consecutive <= gapped {
		t.Errorf("Consecutive match (%d) should score higher than gapped (%d)",
			consecutive, gapped)
	}
}

func TestScoreOrdering(t *testing.T) {
	query := "gpt"
	exact := Score(query, "gpt")
	prefix := Score(query, "gpt-4")
	contains := Score(query, "chatgpt-4")

	if exact <= prefix {
		t.Errorf("Exact (%d) should score higher than prefix (%d)", exact, prefix)
	}
	if prefix <= contains {
		t.Errorf("Prefix (%d) should score higher than contains (%d)", prefix, contains)
	}
}

func TestRealWorldModelSearch(t *testing.T) {
	models := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-2024-04-09",
		"gpt-4o",
		"gpt-4o-mini",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
	}

	tests := []struct {
		query      string
		minMatches int
	}{
		{"gpt", 5},
		{"claude", 3},
		{"opus", 1},
		{"turbo", 2},
		{"mini", 1},
		{"flash", 1},
	}

	for _, tt := range tests {
		matches := Find(tt.query, models)
		if len(matches) < tt.minMatches {
			t.Errorf("Query %q: expected at least %d matches, got %d",
				tt.query, tt.minMatches, len(matches))
		}
		if len(matches) > 0 && matches[0].Score <= 0 {
			t.Errorf("Query %q: top match should have positive score", tt.query)
		}
	}
}

func BenchmarkScore(b *testing.B) {
	query := "gpt"
	text := "gpt-4-turbo-2024-04-09"
	for i := 0; i < b.N; i++ {
		Score(query, text)
	}
}

func BenchmarkFind(b *testing.B) {
	query := "gpt"
	items := make([]string, 1000)
	for i := range items {
		items[i] = "model-" + string(rune('a'+i%26)) + "-gpt-4-turbo"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Find(query, items)
	}
}

func BenchmarkFilter(b *testing.B) {
	query := "gpt"
	type item struct {
		label  string
		detail string
	}
	items := make([]item, 1000)
	for i := range items {
		items[i] = item{
			label:  "model-" + string(rune('a'+i%26)) + "-gpt-4",
			detail: "provider-" + string(rune('a'+i%10)),
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Filter(query, items, func(it item) []string {
			return []string{it.label, it.detail}
		})
	}
}

func TestFilterRanksLabelMatchesFirst(t *testing.T) {
	items := []testItem{
		{label: "other", detail: "this description mentions rule"},
		{label: "rule", detail: "create a rule"},
		{label: "readme", detail: "docs"},
	}
	got := Filter("rule", items, func(i testItem) []string { return []string{i.label, i.detail} })
	if len(got) == 0 || got[0].label != "rule" {
		t.Fatalf("expected label 'rule' first, got %+v", got)
	}
}

func TestFilterExactPathFirst(t *testing.T) {
	paths := []testItem{
		{label: "internal/tui/components/chat_test.go"},
		{label: "internal/tui/components/chat.go"},
		{label: "internal/tui/app.go"},
		{label: "internal/vfs/vfs.go"},
	}
	got := Filter("chat.go", paths, func(i testItem) []string { return []string{i.label} })
	if len(got) == 0 || got[0].label != "internal/tui/components/chat.go" {
		t.Fatalf("expected exact path first, got %+v", got)
	}
}

func TestFilterSkillNames(t *testing.T) {
	items := []testItem{
		{label: "batch", detail: "orchestrate a large parallelizable change"},
		{label: "init", detail: "analyze a codebase and create AGENTS.md"},
		{label: "review", detail: "review a pull request"},
		{label: "rule", detail: "create or update a rule file"},
		{label: "skillify", detail: "turn the current session into a reusable skill"},
	}
	for _, name := range []string{"init", "rule", "review", "batch", "skillify"} {
		got := Filter(name, items, func(i testItem) []string { return []string{i.label, i.detail} })
		if len(got) == 0 || got[0].label != name {
			t.Errorf("query %q: expected %q first, got %+v", name, name, got)
		}
	}
}

func TestScoreLargeTextIsFast(t *testing.T) {
	text := strings.Repeat("a", 5000) + "z"
	if Score("z", text) <= 0 {
		t.Fatal("expected a match in a large text")
	}
	if Score("aaaa", text) <= 0 {
		t.Fatal("expected a subsequence match in a large text")
	}
}
