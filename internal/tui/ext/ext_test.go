package ext

import "testing"

func TestRegisterOrderAndReset(t *testing.T) {
	Reset()
	defer Reset()

	Register(func() []Page { return []Page{{ID: "one", Title: "One"}} })
	Register(func() []Page { return []Page{{ID: "two", Title: "Two"}} })

	pages := Pages()
	if len(pages) != 2 {
		t.Fatalf("pages = %#v", pages)
	}
	if pages[0].ID != "one" || pages[1].ID != "two" {
		t.Fatalf("unexpected order: %#v", pages)
	}

	Reset()
	if got := Pages(); len(got) != 0 {
		t.Fatalf("reset pages = %#v", got)
	}
}

func TestRegisterIgnoresNil(t *testing.T) {
	Reset()
	defer Reset()
	Register(nil)
	if got := Pages(); len(got) != 0 {
		t.Fatalf("pages = %#v", got)
	}
}
