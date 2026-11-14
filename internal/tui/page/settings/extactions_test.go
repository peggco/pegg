package settings

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func extTestPage() pkgplugin.TUIPage {
	return pkgplugin.TUIPage{ID: "p", Title: "Plugin", Owner: "plugin"}
}

func TestExtFormValuesAndSubmit(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	action := pkgplugin.TUIAction{
		ID:    "save",
		Label: "Save",
		Fields: []pkgplugin.TUIField{
			{ID: "channel", Label: "Channel", Value: "ops"},
			{ID: "token", Label: "Token", Secret: true},
		},
	}
	form := newExtForm(s, tab, action)

	form.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	if got := form.fields[0].Value(); got != "opsx" {
		t.Fatalf("channel = %q, want opsx", got)
	}

	form.HandleKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if form.focused != 1 {
		t.Fatalf("focused = %d, want 1", form.focused)
	}
	for _, r := range "sec" {
		form.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, 0))
	}
	form.HandleKey(tcell.NewEventKey(tcell.KeyBackspace, 0, 0))
	if got := form.fields[1].Value(); got != "se" {
		t.Fatalf("token = %q, want se", got)
	}
	if got := string(form.fields[1].display()); got != "**" {
		t.Fatalf("secret display = %q, want **", got)
	}

	form.HandleKey(tcell.NewEventKey(tcell.KeyBacktab, 0, 0))
	if form.focused != 0 {
		t.Fatalf("focused after backtab = %d, want 0", form.focused)
	}

	var submitted map[string]string
	form.onSubmit = func(values map[string]string) { submitted = values }
	form.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if submitted["channel"] != "opsx" || submitted["token"] != "se" {
		t.Fatalf("submitted values = %+v", submitted)
	}
}

func TestExtFormCtrlSSubmits(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	form := newExtForm(s, tab, pkgplugin.TUIAction{
		ID:     "save",
		Label:  "Save",
		Fields: []pkgplugin.TUIField{{ID: "value", Value: "one"}},
	})
	submitted := false
	form.onSubmit = func(map[string]string) { submitted = true }
	form.HandleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, 0))
	if !submitted {
		t.Fatal("Ctrl+S should submit the form")
	}
}

func TestExtFormEscDoesNotSubmit(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	form := newExtForm(s, tab, pkgplugin.TUIAction{
		ID:     "save",
		Label:  "Save",
		Fields: []pkgplugin.TUIField{{ID: "value"}},
	})
	submitted := false
	form.onSubmit = func(map[string]string) { submitted = true }
	form.HandleKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
	if submitted {
		t.Fatal("Esc should cancel the form")
	}
	if s.sub != nil {
		t.Fatal("Esc should close the form")
	}
}

func TestExtFormFieldEditing(t *testing.T) {
	f := extFormField{spec: pkgplugin.TUIField{ID: "v"}, value: []rune("abc"), pos: 3}
	f.handleKey(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	f.handleKey(tcell.NewEventKey(tcell.KeyRune, 'X', 0))
	f.handleKey(tcell.NewEventKey(tcell.KeyEnd, 0, 0))
	f.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	f.handleKey(tcell.NewEventKey(tcell.KeyDelete, 0, 0))
	f.handleKey(tcell.NewEventKey(tcell.KeyBackspace, 0, 0))
	if got := f.Value(); got != "Xa" {
		t.Fatalf("value = %q, want Xa", got)
	}
}

func TestExtActionImmediateUsesRunner(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	action := pkgplugin.TUIAction{ID: "run", Label: "Run"}
	var ran string
	s.extActionRunner = func(_ *extTab, got pkgplugin.TUIAction, values map[string]string) {
		ran = got.ID
		if values != nil {
			t.Errorf("values = %+v, want nil", values)
		}
	}

	s.beginExtAction(tab, action)
	if ran != "run" {
		t.Fatalf("runner ran %q, want run", ran)
	}
	if s.sub != nil {
		t.Fatal("immediate action should not open a modal")
	}
}

func TestExtActionConfirmRunsOnYes(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	action := pkgplugin.TUIAction{ID: "delete", Label: "Delete", Confirm: true}
	ran := false
	s.extActionRunner = func(_ *extTab, _ pkgplugin.TUIAction, _ map[string]string) { ran = true }

	s.beginExtAction(tab, action)
	confirm, ok := s.sub.(*extConfirm)
	if !ok {
		t.Fatalf("sub = %T, want *extConfirm", s.sub)
	}
	confirm.list.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if !ran {
		t.Fatal("confirming Yes should run the action")
	}
	if s.sub != nil {
		t.Fatal("confirmation should close")
	}
}

func TestExtActionFieldsOpenForm(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	action := pkgplugin.TUIAction{
		ID:     "save",
		Label:  "Save",
		Fields: []pkgplugin.TUIField{{ID: "value"}},
	}
	s.beginExtAction(tab, action)
	if _, ok := s.sub.(*extForm); !ok {
		t.Fatalf("sub = %T, want *extForm", s.sub)
	}
}

func TestExtActionFormSubmitUsesRunner(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	action := pkgplugin.TUIAction{
		ID:     "save",
		Label:  "Save",
		Fields: []pkgplugin.TUIField{{ID: "value", Value: "hello"}},
	}
	var got map[string]string
	s.extActionRunner = func(_ *extTab, _ pkgplugin.TUIAction, values map[string]string) { got = values }

	s.beginExtAction(tab, action)
	form, ok := s.sub.(*extForm)
	if !ok {
		t.Fatalf("sub = %T, want *extForm", s.sub)
	}
	form.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if got["value"] != "hello" {
		t.Fatalf("values = %+v, want value=hello", got)
	}
}

func TestRunExtActionWithoutPluginSetsError(t *testing.T) {
	s := New(Deps{})
	tab := newExtTab(s, extTestPage())
	s.runExtAction(tab, pkgplugin.TUIAction{ID: "run", Label: "Run"}, nil)
	if s.errMsg == "" {
		t.Fatal("missing plugin manager should set errMsg")
	}
	if s.noticeMsg != "" {
		t.Fatalf("noticeMsg = %q, want empty", s.noticeMsg)
	}
}
