package install

import "testing"

func TestParseSource(t *testing.T) {
	source, err := ParseSource("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if source.Slug() != "owner/repo" || source.DefaultName() != "repo" {
		t.Fatalf("unexpected source: %#v", source)
	}
	if source.DefaultName() != "repo" {
		t.Fatal("default name")
	}
	if got := (Source{Repo: "my.plugin"}).DefaultName(); got != "my" {
		t.Fatalf("default name with extension: %q", got)
	}
	for _, value := range []string{"", "owner", "a/b/c", "../repo", "owner/..", "owner/re po", "owner/repo\\x"} {
		if _, err := ParseSource(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"plugin", "pegg-browser-use", "a_b"} {
		if err := ValidateName(name); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
	}
	for _, name := range []string{"", "../x", "a/b", ".hidden", "plugin.exe", "plugin.sh"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}
