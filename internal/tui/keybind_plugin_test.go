package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestPluginKeyMatch(t *testing.T) {
	cases := []struct {
		spec string
		key  *tcell.EventKey
		want bool
	}{
		{"ctrl+k", tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl), true},
		{"ctrl+k", tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl), false},
		{"alt+enter", tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt), true},
		{"f2", tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone), true},
		{"shift+tab", tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift), true},
		{"k", tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone), false},
		{"ctrl+", tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl), false},
		{"f13", tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone), false},
	}
	for _, tc := range cases {
		if got := pluginKeyMatch(tc.spec, tc.key); got != tc.want {
			t.Errorf("pluginKeyMatch(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}
