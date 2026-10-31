package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitPastedPaths(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "/tmp/a.png", []string{"/tmp/a.png"}},
		{"backslash escape", `/tmp/my\ file.png`, []string{"/tmp/my file.png"}},
		{"double quotes", `"/tmp/my file.png"`, []string{"/tmp/my file.png"}},
		{"single quotes", `'/tmp/my file.png'`, []string{"/tmp/my file.png"}},
		{"multiple quoted", `"/tmp/a b.png" '/tmp/c d.png'`, []string{"/tmp/a b.png", "/tmp/c d.png"}},
		{"newlines", "/tmp/a.png\n/tmp/b.png\n", []string{"/tmp/a.png", "/tmp/b.png"}},
		{"tabs", "/tmp/a.png\t/tmp/b.png", []string{"/tmp/a.png", "/tmp/b.png"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPastedPaths(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitPastedPaths(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolvePastedPaths(t *testing.T) {
	dir := t.TempDir()
	withSpace := filepath.Join(dir, "my file.png")
	plain := filepath.Join(dir, "plain.png")
	other := filepath.Join(dir, "other file.txt")
	for _, path := range []string{withSpace, plain, other} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"unescaped space", withSpace, []string{withSpace}},
		{"escaped space", `"` + withSpace + `"`, []string{withSpace}},
		{"multiple lines", plain + "\n" + withSpace, []string{plain, withSpace}},
		{"multiple quoted", `"` + withSpace + `" "` + other + `"`, []string{withSpace, other}},
		{"file url", "file://" + withSpace, []string{withSpace}},
		{"plain text", "hello world", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolvePastedPaths(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("resolvePastedPaths(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
