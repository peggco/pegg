package i18n

import "testing"

func TestSetAndCurrent(t *testing.T) {
	t.Cleanup(func() { Set(English) })

	if Current() != English {
		t.Fatalf("default current = %q, want %q", Current(), English)
	}
	if !Set(Turkish) {
		t.Fatalf("Set(%q) = false, want true", Turkish)
	}
	if Current() != Turkish {
		t.Fatalf("current = %q, want %q", Current(), Turkish)
	}
	if Set("xx") {
		t.Fatalf("Set(unknown) = true, want false")
	}
	if Current() != Turkish {
		t.Fatalf("current changed after unknown Set: %q", Current())
	}
}

func TestTranslateFallback(t *testing.T) {
	t.Cleanup(func() { Set(English) })

	Set(Turkish)
	if got := T("settings.title"); got != "Ayarlar" {
		t.Errorf("tr settings.title = %q, want %q", got, "Ayarlar")
	}
	Set(English)
	if got := T("settings.title"); got != "Settings" {
		t.Errorf("en settings.title = %q, want %q", got, "Settings")
	}
	if got := T("does.not.exist"); got != "does.not.exist" {
		t.Errorf("missing key = %q, want key echoed", got)
	}
}

func TestTranslateFallbackToEnglish(t *testing.T) {
	t.Cleanup(func() { Set(English) })

	en["only.en.test"] = "english only"
	defer delete(en, "only.en.test")

	Set(Turkish)
	if got := T("only.en.test"); got != "english only" {
		t.Errorf("fallback = %q, want %q", got, "english only")
	}
}

func TestTf(t *testing.T) {
	t.Cleanup(func() { Set(English) })

	Set(English)
	if got := Tf("settings.provider.apiKeyTitle", "openai"); got != "API key for openai" {
		t.Errorf("Tf = %q, want %q", got, "API key for openai")
	}
}

func TestNamesAndDisplay(t *testing.T) {
	names := Names()
	if len(names) != 2 || names[0] != English || names[1] != Turkish {
		t.Fatalf("Names() = %v, want [en tr]", names)
	}
	if DisplayName(Turkish) != "Türkçe" {
		t.Errorf("DisplayName(tr) = %q", DisplayName(Turkish))
	}
	if DisplayName("xx") != "xx" {
		t.Errorf("DisplayName(unknown) = %q, want code", DisplayName("xx"))
	}
}

func TestOnChangeFires(t *testing.T) {
	t.Cleanup(func() { Set(English) })

	var got string
	OnChange(func(code string) { got = code })
	Set(Turkish)
	if got != Turkish {
		t.Errorf("onChange got %q, want %q", got, Turkish)
	}
}
