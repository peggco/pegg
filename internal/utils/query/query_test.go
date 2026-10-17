package query

import (
	"reflect"
	"testing"
)

func TestBuild(t *testing.T) {
	b := NewBuilder("logs", "id", "level", "message")

	built, err := b.Build(Query{
		Filters: []Filter{
			{Column: "level", Operator: OpEqual, Value: "INFO"},
		},
		Search:        "timeout",
		SearchColumns: []string{"message"},
		Sort:          []Sort{{Column: "id", Dir: Desc}},
		Page:          Page{Number: 2, Size: 10},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantSelect := "SELECT * FROM logs WHERE level = ? AND (message LIKE ? ESCAPE '\\') ORDER BY id DESC LIMIT ? OFFSET ?"
	if built.SelectSQL != wantSelect {
		t.Errorf("SelectSQL:\n got %q\nwant %q", built.SelectSQL, wantSelect)
	}

	wantSelectArgs := []any{"INFO", "%timeout%", 10, 10}
	if !reflect.DeepEqual(built.SelectArgs, wantSelectArgs) {
		t.Errorf("SelectArgs: got %#v, want %#v", built.SelectArgs, wantSelectArgs)
	}

	wantCount := "SELECT COUNT(*) FROM logs WHERE level = ? AND (message LIKE ? ESCAPE '\\')"
	if built.CountSQL != wantCount {
		t.Errorf("CountSQL:\n got %q\nwant %q", built.CountSQL, wantCount)
	}

	wantCountArgs := []any{"INFO", "%timeout%"}
	if !reflect.DeepEqual(built.CountArgs, wantCountArgs) {
		t.Errorf("CountArgs: got %#v, want %#v", built.CountArgs, wantCountArgs)
	}
}

func TestBuildDefaults(t *testing.T) {
	b := NewBuilder("logs", "id")

	built, err := b.Build(Query{})
	if err != nil {
		t.Fatal(err)
	}

	if want := "SELECT * FROM logs LIMIT ? OFFSET ?"; built.SelectSQL != want {
		t.Errorf("SelectSQL: got %q, want %q", built.SelectSQL, want)
	}
	if want := "SELECT COUNT(*) FROM logs"; built.CountSQL != want {
		t.Errorf("CountSQL: got %q, want %q", built.CountSQL, want)
	}

	wantArgs := []any{DefaultPageSize, 0}
	if !reflect.DeepEqual(built.SelectArgs, wantArgs) {
		t.Errorf("SelectArgs: got %#v, want %#v", built.SelectArgs, wantArgs)
	}
}

func TestBuildDefaultSort(t *testing.T) {
	b := NewBuilder("logs", "id", "timestamp").DefaultSort("timestamp", Desc)

	t.Run("applied when query has no sort", func(t *testing.T) {
		built, err := b.Build(Query{})
		if err != nil {
			t.Fatal(err)
		}
		if want := "SELECT * FROM logs ORDER BY timestamp DESC LIMIT ? OFFSET ?"; built.SelectSQL != want {
			t.Errorf("SelectSQL: got %q, want %q", built.SelectSQL, want)
		}
	})

	t.Run("overridden by explicit sort", func(t *testing.T) {
		built, err := b.Build(Query{Sort: []Sort{{Column: "id", Dir: Asc}}})
		if err != nil {
			t.Fatal(err)
		}
		if want := "SELECT * FROM logs ORDER BY id ASC LIMIT ? OFFSET ?"; built.SelectSQL != want {
			t.Errorf("SelectSQL: got %q, want %q", built.SelectSQL, want)
		}
	})
}

func TestBuildEscapesLikeWildcards(t *testing.T) {
	b := NewBuilder("logs", "message")

	built, err := b.Build(Query{
		Search:        "100%_done",
		SearchColumns: []string{"message"},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []any{`%100\%\_done%`}
	if !reflect.DeepEqual(built.SelectArgs[:1], want) {
		t.Errorf("SelectArgs: got %#v, want %#v", built.SelectArgs[:1], want)
	}
}

func TestBuildRejectsInvalidInput(t *testing.T) {
	b := NewBuilder("logs", "id", "level")

	cases := []struct {
		name string
		q    Query
	}{
		{"unknown filter column", Query{Filters: []Filter{{Column: "message", Operator: OpEqual, Value: "x"}}}},
		{"unknown operator", Query{Filters: []Filter{{Column: "level", Operator: "~", Value: "x"}}}},
		{"unknown search column", Query{Search: "x", SearchColumns: []string{"nope"}}},
		{"search without columns", Query{Search: "x"}},
		{"unknown sort column", Query{Sort: []Sort{{Column: "nope", Dir: Asc}}}},
		{"bad sort direction", Query{Sort: []Sort{{Column: "id", Dir: "up"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := b.Build(tc.q); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestBuildSelectColumns(t *testing.T) {
	b := NewBuilder("sessions", "id", "title", "mode").
		SelectColumns("id", "title", "mode")

	built, err := b.Build(Query{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT id, title, mode FROM sessions LIMIT ? OFFSET ?"
	if built.SelectSQL != want {
		t.Errorf("SelectSQL:\n got %q\nwant %q", built.SelectSQL, want)
	}
}
