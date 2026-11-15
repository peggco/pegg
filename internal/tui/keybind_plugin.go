package tui

import (
	"context"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	corehook "github.com/peggco/pegg/internal/core/hook"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

var pluginNamedKeys = map[string]tcell.Key{
	"tab":       tcell.KeyTab,
	"backtab":   tcell.KeyBacktab,
	"enter":     tcell.KeyEnter,
	"return":    tcell.KeyEnter,
	"esc":       tcell.KeyEscape,
	"escape":    tcell.KeyEscape,
	"backspace": tcell.KeyBackspace2,
	"delete":    tcell.KeyDelete,
	"insert":    tcell.KeyInsert,
	"home":      tcell.KeyHome,
	"end":       tcell.KeyEnd,
	"up":        tcell.KeyUp,
	"down":      tcell.KeyDown,
	"left":      tcell.KeyLeft,
	"right":     tcell.KeyRight,
	"pgup":      tcell.KeyPgUp,
	"pageup":    tcell.KeyPgUp,
	"pgdn":      tcell.KeyPgDn,
	"pagedown":  tcell.KeyPgDn,
}

func (a *App) dispatchPluginBinding(ev *tcell.EventKey) bool {
	if a.viewHooks == nil {
		return false
	}
	for _, binding := range a.viewHooks.bindings() {
		if !pluginKeyMatch(binding.Key, ev) {
			continue
		}
		bound := binding
		go corehook.ApplyAction(context.Background(), HookCommand, pkgplugin.CommandHook{ID: bound.ID, Key: bound.Key})
		a.requestRedraw()
		return true
	}
	return false
}

func pluginKeyMatch(spec string, ev *tcell.EventKey) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	var want tcell.ModMask
	name := ""
	for _, part := range parts {
		switch part {
		case "ctrl", "control":
			want |= tcell.ModCtrl
		case "alt", "option":
			want |= tcell.ModAlt
		case "shift":
			want |= tcell.ModShift
		case "":
		default:
			name = part
		}
	}
	if name == "" {
		return false
	}
	if name == "tab" && want&tcell.ModShift != 0 {
		return ev.Key() == tcell.KeyBacktab
	}
	if key, ok := pluginNamedKeys[name]; ok {
		return ev.Key() == key && modifiersMatch(ev.Modifiers(), want)
	}
	if strings.HasPrefix(name, "f") && len(name) <= 3 {
		if number, err := strconv.Atoi(name[1:]); err == nil && number >= 1 && number <= 12 {
			return ev.Key() == tcell.KeyF1+tcell.Key(number-1) && modifiersMatch(ev.Modifiers(), want)
		}
	}
	runes := []rune(name)
	if len(runes) != 1 {
		return false
	}
	r := runes[0]
	if want&tcell.ModCtrl != 0 && r >= 'a' && r <= 'z' {
		return ev.Key() == tcell.Key(int(tcell.KeyCtrlA)+int(r-'a')) && ev.Modifiers()&tcell.ModAlt == want&tcell.ModAlt
	}
	if want == 0 {
		return false
	}
	return ev.Key() == tcell.KeyRune && ev.Rune() == r && modifiersMatch(ev.Modifiers(), want)
}

func modifiersMatch(got, want tcell.ModMask) bool {
	return got&want == want
}
