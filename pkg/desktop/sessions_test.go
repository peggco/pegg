//go:build gui

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickProjectDir(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	for _, dir := range []string{a, b} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		items   []SessionListItem
		exclude string
		want    string
	}{
		{
			name:  "first valid project",
			items: []SessionListItem{{ProjectDir: a}},
			want:  a,
		},
		{
			name:    "skips the removed project",
			items:   []SessionListItem{{ProjectDir: a}, {ProjectDir: b}},
			exclude: a,
			want:    b,
		},
		{
			name:  "skips empty and missing directories",
			items: []SessionListItem{{ProjectDir: ""}, {ProjectDir: filepath.Join(root, "missing")}, {ProjectDir: b}},
			want:  b,
		},
		{
			name:    "nothing selectable",
			items:   []SessionListItem{{ProjectDir: a}, {ProjectDir: ""}},
			exclude: a,
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickProjectDir(tc.items, tc.exclude); got != tc.want {
				t.Fatalf("pickProjectDir = %q, want %q", got, tc.want)
			}
		})
	}
}
