package settings

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type extActionRunnerFunc func(tab *extTab, action pkgplugin.TUIAction, values map[string]string)

type extConfirm struct {
	settings *Settings
	tab      *extTab
	action   pkgplugin.TUIAction
	list     *components.List
	region   layout.Region
}

func newExtConfirm(s *Settings, tab *extTab, action pkgplugin.TUIAction) *extConfirm {
	c := &extConfirm{settings: s, tab: tab, action: action, list: components.NewList(action.Label)}
	c.list.SetItems([]components.ListItem{
		{Label: "Yes"},
		{Label: "No"},
	})
	c.list.SetOnSelect(func(_ int, item components.ListItem) {
		s.back()
		if item.Label != "Yes" {
			return
		}
		if len(action.Fields) > 0 {
			s.openSub(newExtForm(s, tab, action))
			return
		}
		s.invokeExtAction(tab, action, nil)
	})
	return c
}

func (c *extConfirm) HandleKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEsc {
		c.settings.back()
		return true
	}
	return c.list.HandleKey(ev)
}

func (c *extConfirm) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	if c.list.HandleMouse(x, y, buttons) {
		return true
	}
	if buttons&tcell.ButtonPrimary != 0 && !inRegion(c.region, x, y) {
		c.settings.back()
		return true
	}
	return false
}

func (c *extConfirm) Draw(s tcell.Screen, bounds layout.Region, focused bool) {
	w := bounds.Width - 8
	if w > 48 {
		w = 48
	}
	if w < 20 {
		w = bounds.Width
	}
	h := 9
	if h > bounds.Height {
		h = bounds.Height
	}
	inner := components.DrawCenteredBox(s, bounds, w, h, c.action.Label)
	c.region = inner
	c.list.Draw(s, layout.Region{Left: inner.Left, Top: inner.Top, Width: inner.Width, Height: inner.Height - 1}, focused)
	components.DrawFooter(s, inner, "↑/↓ select  Enter confirm  Esc cancel")
}

type extFormField struct {
	spec  pkgplugin.TUIField
	value []rune
	pos   int
	box   layout.Region
	inner layout.Region
}

func (f *extFormField) display() []rune {
	if len(f.value) == 0 && f.spec.Placeholder != "" {
		return []rune(f.spec.Placeholder)
	}
	if f.spec.Secret {
		out := make([]rune, len(f.value))
		for i := range out {
			out[i] = '*'
		}
		return out
	}
	return f.value
}

func (f *extFormField) Value() string { return string(f.value) }

func (f *extFormField) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if f.pos > 0 {
			f.value = append(f.value[:f.pos-1], f.value[f.pos:]...)
			f.pos--
		}
		return true
	case tcell.KeyDelete:
		if f.pos < len(f.value) {
			f.value = append(f.value[:f.pos], f.value[f.pos+1:]...)
		}
		return true
	case tcell.KeyLeft:
		if f.pos > 0 {
			f.pos--
		}
		return true
	case tcell.KeyRight:
		if f.pos < len(f.value) {
			f.pos++
		}
		return true
	case tcell.KeyHome:
		f.pos = 0
		return true
	case tcell.KeyEnd:
		f.pos = len(f.value)
		return true
	case tcell.KeyRune:
		if r := ev.Rune(); r != 0 && r >= ' ' {
			f.value = append(f.value[:f.pos], append([]rune{r}, f.value[f.pos:]...)...)
			f.pos++
		}
		return true
	}
	return false
}

type extForm struct {
	settings *Settings
	tab      *extTab
	action   pkgplugin.TUIAction
	fields   []extFormField
	focused  int
	scroll   int
	visible  int
	region   layout.Region
	preset   map[string]string
	onSubmit func(map[string]string)
}

func newExtForm(s *Settings, tab *extTab, action pkgplugin.TUIAction, presets ...map[string]string) *extForm {
	preset := map[string]string{}
	if len(presets) > 0 && presets[0] != nil {
		preset = presets[0]
	}
	values := map[string]string{}
	for _, field := range action.Fields {
		values[field.ID] = field.Value
	}
	for key, value := range preset {
		values[key] = value
	}
	m := &extForm{settings: s, tab: tab, action: action, preset: preset}
	for _, field := range action.Fields {
		if action.PickerField != "" && field.ID == action.PickerField {
			continue
		}
		if !fieldVisible(field, values) {
			continue
		}
		value := []rune(values[field.ID])
		m.fields = append(m.fields, extFormField{spec: field, value: value, pos: len(value)})
	}
	return m
}

func fieldVisible(field pkgplugin.TUIField, values map[string]string) bool {
	if field.ShowWhen == nil {
		return true
	}
	current := values[field.ShowWhen.Field]
	for _, value := range field.ShowWhen.Values {
		if value == current {
			return true
		}
	}
	return false
}

func (m *extForm) Values() map[string]string {
	values := make(map[string]string, len(m.fields)+len(m.preset))
	for key, value := range m.preset {
		values[key] = value
	}
	for i := range m.fields {
		key := m.fields[i].spec.ID
		if key == "" {
			key = m.fields[i].spec.Label
		}
		values[key] = m.fields[i].Value()
	}
	return values
}

func (m *extForm) ensureVisible() {
	if m.visible <= 0 || len(m.fields) == 0 {
		return
	}
	if m.focused < m.scroll {
		m.scroll = m.focused
	}
	if m.focused >= m.scroll+m.visible {
		m.scroll = m.focused - m.visible + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	if max := len(m.fields) - m.visible; m.scroll > max {
		m.scroll = max
	}
}

func (m *extForm) submit() {
	values := m.Values()
	if m.onSubmit != nil {
		m.onSubmit(values)
		return
	}
	m.settings.back()
	m.settings.invokeExtAction(m.tab, m.action, values)
}

func (m *extForm) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEsc:
		m.settings.back()
		return true
	case tcell.KeyEnter, tcell.KeyCtrlS:
		m.submit()
		return true
	case tcell.KeyTab:
		if len(m.fields) > 0 {
			if ev.Modifiers()&tcell.ModShift != 0 {
				m.focused = (m.focused - 1 + len(m.fields)) % len(m.fields)
			} else {
				m.focused = (m.focused + 1) % len(m.fields)
			}
			m.ensureVisible()
		}
		return true
	case tcell.KeyBacktab:
		if len(m.fields) > 0 {
			m.focused = (m.focused - 1 + len(m.fields)) % len(m.fields)
			m.ensureVisible()
		}
		return true
	}
	if len(m.fields) == 0 {
		return false
	}
	return m.fields[m.focused].handleKey(ev)
}

func (m *extForm) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	if buttons&tcell.ButtonPrimary == 0 {
		return false
	}
	for i := range m.fields {
		if inRegion(m.fields[i].box, x, y) {
			m.focused = i
			if m.fields[i].pos > len(m.fields[i].value) {
				m.fields[i].pos = len(m.fields[i].value)
			}
			return true
		}
	}
	if !inRegion(m.region, x, y) {
		m.settings.back()
		return true
	}
	return false
}

func (m *extForm) Draw(s tcell.Screen, bounds layout.Region, focused bool) {
	th := styles.Current()
	w := bounds.Width - 8
	if w > 64 {
		w = 64
	}
	if w < 20 {
		w = bounds.Width
	}
	h := 6 + len(m.fields)*4
	if h > bounds.Height {
		h = bounds.Height
	}
	inner := components.DrawCenteredBox(s, bounds, w, h, m.action.Label)
	m.region = inner
	y := inner.Top
	if m.action.Description != "" && y < inner.Bottom()-1 {
		components.DrawText(s, inner.Left, y, components.TruncateTo(m.action.Description, inner.Width),
			th.Base().Foreground(th.Hint).Background(th.InputBg))
		y++
	}

	m.visible = (inner.Bottom() - 1 - y) / 4
	if m.visible < 1 {
		m.visible = 1
	}
	m.ensureVisible()
	end := m.scroll + m.visible
	if end > len(m.fields) {
		end = len(m.fields)
	}
	for i := m.scroll; i < end; i++ {
		m.drawField(s, &m.fields[i], i == m.focused, layout.Region{
			Left:   inner.Left,
			Top:    y,
			Width:  inner.Width,
			Height: 4,
		})
		y += 4
	}
	components.DrawFooter(s, inner, "Tab next  ·  Enter save  ·  Esc cancel")
}

func (m *extForm) drawField(s tcell.Screen, f *extFormField, focused bool, bounds layout.Region) {
	th := styles.Current()
	label := f.spec.Label
	if label == "" {
		label = f.spec.ID
	}
	if f.spec.Required {
		label += " *"
	}
	if len(f.spec.Options) > 0 {
		label += " (" + strings.Join(f.spec.Options, "/") + ")"
	}
	components.DrawText(s, bounds.Left, bounds.Top, components.TruncateTo(label, bounds.Width),
		th.Base().Foreground(th.Hint).Background(th.InputBg))

	box := layout.Region{Left: bounds.Left, Top: bounds.Top + 1, Width: bounds.Width, Height: 3}
	border := th.Base().Foreground(th.Border)
	if focused {
		border = th.Base().Foreground(th.Accent)
	}
	components.DrawBox(s, box, border)
	inner := layout.Pad(box, 1, 1)
	f.box = box
	f.inner = inner

	display := f.display()
	textStyle := th.Base().Foreground(th.InputText).Background(th.InputBg)
	if len(f.value) == 0 {
		textStyle = th.Base().Foreground(th.Placeholder).Background(th.InputBg)
	}
	components.DrawText(s, inner.Left, inner.Top, components.TruncateTo(string(display), inner.Width), textStyle)

	if focused && inner.Width > 0 && inner.Height > 0 {
		cx := inner.Left + f.pos
		cy := inner.Top
		if cx < inner.Right() {
			ch := ' '
			if f.pos < len(display) {
				ch = display[f.pos]
			}
			s.SetContent(cx, cy, ch, nil, th.Base().Foreground(th.InputBg).Background(th.Cursor))
		}
	}
}

func (s *Settings) beginExtAction(tab *extTab, action pkgplugin.TUIAction) {
	if action.Confirm {
		s.openSub(newExtConfirm(s, tab, action))
		return
	}
	if action.PickerField != "" {
		for _, field := range action.Fields {
			if field.ID == action.PickerField && len(field.Options) > 0 {
				s.openSub(newExtPicker(s, tab, action, field))
				return
			}
		}
	}
	if len(action.Fields) > 0 {
		s.openSub(newExtForm(s, tab, action))
		return
	}
	s.invokeExtAction(tab, action, nil)
}

type extPicker struct {
	settings *Settings
	tab      *extTab
	action   pkgplugin.TUIAction
	field    pkgplugin.TUIField
	list     *components.List
}

func newExtPicker(s *Settings, tab *extTab, action pkgplugin.TUIAction, field pkgplugin.TUIField) *extPicker {
	p := &extPicker{settings: s, tab: tab, action: action, field: field, list: components.NewList(field.Label)}
	items := make([]components.ListItem, 0, len(field.Options))
	for _, option := range field.Options {
		items = append(items, components.ListItem{Label: option, Data: option})
	}
	p.list.SetItems(items)
	p.list.SetOnSelect(func(_ int, item components.ListItem) {
		s.back()
		s.openSub(newExtForm(s, tab, action, map[string]string{field.ID: item.Label}))
	})
	return p
}

func (p *extPicker) HandleKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEsc {
		p.settings.back()
		return true
	}
	return p.list.HandleKey(ev)
}

func (p *extPicker) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	return p.list.HandleMouse(x, y, buttons)
}

func (p *extPicker) Draw(s tcell.Screen, bounds layout.Region, focused bool) {
	w := bounds.Width - 8
	if w > 40 {
		w = 40
	}
	if w < 20 {
		w = bounds.Width
	}
	h := len(p.field.Options) + 3
	if h > bounds.Height {
		h = bounds.Height
	}
	title := p.field.Label
	if title == "" {
		title = "Select"
	}
	inner := components.DrawCenteredBox(s, bounds, w, h, title)
	p.list.Draw(s, layout.Region{Left: inner.Left, Top: inner.Top, Width: inner.Width, Height: inner.Height - 1}, focused)
	components.DrawFooter(s, inner, "Enter · Esc")
}

func (s *Settings) invokeExtAction(tab *extTab, action pkgplugin.TUIAction, values map[string]string) {
	if s.extActionRunner != nil {
		s.extActionRunner(tab, action, values)
		return
	}
	s.runExtAction(tab, action, values)
}

func (s *Settings) runExtAction(tab *extTab, action pkgplugin.TUIAction, values map[string]string) {
	if s.deps.Plugin == nil {
		s.noticeMsg = ""
		s.errMsg = "plugin manager unavailable"
		s.requestRedrawFn()
		return
	}
	label := action.Label
	if label == "" {
		label = action.ID
	}
	s.errMsg = ""
	s.noticeMsg = "Running " + label + "..."
	s.requestRedrawFn()

	req := pkgplugin.TUIActionRequest{
		PageID:   tab.page.ID,
		ActionID: action.ID,
		Values:   values,
	}
	owner := tab.page.Owner

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := s.deps.Plugin.ExecuteTUIAction(ctx, owner, req)
		s.dispatch(func() {
			if err != nil {
				s.noticeMsg = ""
				s.errMsg = fmt.Sprintf("plugin action failed: %v", err)
				s.requestRedrawFn()
				return
			}
			if res != nil && res.Page != nil {
				tab.SetPage(*res.Page)
			}
			s.errMsg = ""
			if res != nil && res.Output != "" {
				s.noticeMsg = res.Output
			} else {
				s.noticeMsg = "Done"
			}
			s.requestRedrawFn()
		})
	}()
}
