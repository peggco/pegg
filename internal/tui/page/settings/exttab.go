package settings

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/ext"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

const extDefaultWidth = 76

type extTab struct {
	settings       *Settings
	page           pkgplugin.TUIPage
	doc            *extDoc
	docWidth       int
	sel            int
	scroll         int
	drawTop        int
	drawHeight     int
	settingsValues map[string]string
	hook           bool
	hookTab        string
}

func newExtTab(s *Settings, page pkgplugin.TUIPage) *extTab {
	t := &extTab{settings: s, page: page, settingsValues: map[string]string{}}
	t.resetSettingsValues()
	t.ensureDoc(extDefaultWidth)
	return t
}

func (t *extTab) Title() string {
	if t.page.Title == "" {
		return "Plugin"
	}
	return t.page.Title
}

func (t *extTab) resetSettingsValues() {
	t.settingsValues = map[string]string{}
	for _, field := range t.page.Settings {
		t.settingsValues[field.ID] = field.Value
	}
}

func (t *extTab) ensureDoc(width int) {
	if width < 20 {
		width = 20
	}
	if t.doc != nil && t.docWidth == width {
		return
	}
	t.doc = buildExtDoc(t.page, width)
	t.docWidth = width
	t.clampSel()
}

func (t *extTab) clampSel() {
	if t.doc == nil || len(t.doc.hits) == 0 {
		t.sel = 0
		return
	}
	if t.sel >= len(t.doc.hits) {
		t.sel = len(t.doc.hits) - 1
	}
	if t.sel < 0 {
		t.sel = 0
	}
}

func (t *extTab) selectedHit() *extHit {
	if t.doc == nil || t.sel < 0 || t.sel >= len(t.doc.hits) {
		return nil
	}
	return t.doc.hits[t.sel]
}

func (t *extTab) selectHit(hit *extHit) {
	if t.doc == nil {
		return
	}
	for i, h := range t.doc.hits {
		if h == hit {
			t.sel = i
			return
		}
	}
}

func (t *extTab) itemAt(hit *extHit) (pkgplugin.TUIItem, bool) {
	if hit == nil || hit.section < 0 || hit.section >= len(t.page.Sections) {
		return pkgplugin.TUIItem{}, false
	}
	items := t.page.Sections[hit.section].Items
	if hit.item < 0 || hit.item >= len(items) {
		return pkgplugin.TUIItem{}, false
	}
	return items[hit.item], true
}

func (t *extTab) itemID(hit *extHit) string {
	item, ok := t.itemAt(hit)
	if !ok {
		return ""
	}
	return item.ID
}

func (t *extTab) SetPage(page pkgplugin.TUIPage) {
	t.page = page
	t.resetSettingsValues()
	t.doc = nil
	t.ensureDoc(t.docWidth)
}

func (t *extTab) HandleKey(ev *tcell.EventKey) bool {
	t.ensureDoc(t.docWidth)
	switch ev.Key() {
	case tcell.KeyUp:
		if t.sel > 0 {
			t.sel--
			return true
		}
		return false
	case tcell.KeyDown:
		if t.doc != nil && t.sel < len(t.doc.hits)-1 {
			t.sel++
			return true
		}
		return false
	case tcell.KeyPgUp:
		t.sel -= 5
		t.clampSel()
		return true
	case tcell.KeyPgDn:
		t.sel += 5
		t.clampSel()
		return true
	case tcell.KeyHome:
		t.sel = 0
		return true
	case tcell.KeyEnd:
		if t.doc != nil && len(t.doc.hits) > 0 {
			t.sel = len(t.doc.hits) - 1
		}
		return true
	case tcell.KeyEnter:
		t.activate()
		return true
	case tcell.KeyLeft, tcell.KeyRight:
		hit := t.selectedHit()
		if hit == nil {
			return false
		}
		delta := 1
		if ev.Key() == tcell.KeyLeft {
			delta = -1
		}
		switch hit.kind {
		case hitSetting:
			if t.cycleSetting(hit.index, delta) {
				return true
			}
		case hitItem:
			if t.cycleItemOption(hit, delta) {
				return true
			}
		}
		return false
	}
	if ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'r':
			t.settings.refreshExtPage(t)
			return true
		case 'e':
			return t.editSelected()
		case 'x':
			return t.runSelected()
		case 'd':
			return t.deleteSelected()
		case 't':
			return t.testSelected()
		case 'a':
			return t.addAction()
		case 's':
			return t.openSettings()
		}
	}
	return false
}

func (t *extTab) activate() {
	hit := t.selectedHit()
	if hit == nil {
		return
	}
	switch hit.kind {
	case hitToolbar, hitAction:
		t.settings.beginExtAction(t, pkgplugin.TUIAction{
			ID:          hit.action,
			Label:       hit.label,
			Description: hit.detail,
			PickerField: hit.picker,
			Fields:      hit.fields,
		})
	case hitItem:
		if len(hit.fields) > 0 && hit.action != "" {
			t.editItem(hit)
			return
		}
		if run := hit.meta["runAction"]; run != "" {
			t.invokeItemAction(hit, run, nil)
		}
	case hitSetting:
		t.editSetting(hit.index)
	}
}

func (t *extTab) editSelected() bool {
	hit := t.selectedHit()
	if hit == nil {
		return false
	}
	switch hit.kind {
	case hitItem:
		if len(hit.fields) > 0 && hit.action != "" {
			t.editItem(hit)
			return true
		}
	case hitSetting:
		t.editSetting(hit.index)
		return true
	}
	return false
}

func (t *extTab) runSelected() bool {
	hit := t.selectedHit()
	if hit == nil || hit.kind != hitItem {
		return false
	}
	action := hit.meta["runAction"]
	if action == "" {
		return false
	}
	t.invokeItemAction(hit, action, nil)
	return true
}

func (t *extTab) deleteSelected() bool {
	hit := t.selectedHit()
	if hit == nil || hit.kind != hitItem {
		return false
	}
	action := hit.meta["deleteAction"]
	if action == "" {
		return false
	}
	t.invokeItemAction(hit, action, nil)
	return true
}

func (t *extTab) testSelected() bool {
	hit := t.selectedHit()
	if hit == nil || hit.kind != hitItem {
		return false
	}
	action := hit.meta["testAction"]
	if action == "" {
		return false
	}
	t.invokeItemAction(hit, action, nil)
	return true
}

func (t *extTab) addAction() bool {
	for i := range t.page.Actions {
		action := t.page.Actions[i]
		if action.ID == "add" || action.ID == "card.add" {
			t.settings.beginExtAction(t, action)
			return true
		}
	}
	return false
}

func (t *extTab) openSettings() bool {
	if len(t.page.Settings) == 0 {
		return false
	}
	fields := make([]pkgplugin.TUIField, len(t.page.Settings))
	for i, field := range t.page.Settings {
		fields[i] = field
		fields[i].Value = t.settingsValues[field.ID]
	}
	form := newExtForm(t.settings, t, pkgplugin.TUIAction{
		ID:     t.page.SaveAction,
		Label:  "Settings",
		Fields: fields,
	})
	form.onSubmit = func(values map[string]string) {
		for key, value := range values {
			t.settingsValues[key] = value
		}
		t.settings.back()
		t.saveSettings()
	}
	t.settings.openSub(form)
	return true
}

func (t *extTab) editItem(hit *extHit) {
	action := pkgplugin.TUIAction{
		ID:          hit.action,
		Label:       hit.label,
		Description: hit.detail,
		Fields:      hit.fields,
	}
	form := newExtForm(t.settings, t, action)
	form.onSubmit = func(values map[string]string) {
		t.settings.back()
		t.invokeItemAction(hit, action.ID, values)
	}
	t.settings.openSub(form)
}

func (t *extTab) invokeItemAction(hit *extHit, actionID string, values map[string]string) {
	merged := map[string]string{}
	for key, value := range values {
		merged[key] = value
	}
	if id := t.itemID(hit); id != "" {
		merged["id"] = id
		merged["name"] = id
		merged["channel"] = id
	}
	t.settings.invokeExtAction(t, pkgplugin.TUIAction{ID: actionID, Label: hit.label}, merged)
}

func (t *extTab) editSetting(index int) {
	if index < 0 || index >= len(t.page.Settings) {
		return
	}
	field := t.page.Settings[index]
	field.Value = t.settingsValues[field.ID]
	form := newExtForm(t.settings, t, pkgplugin.TUIAction{
		ID:     t.page.SaveAction,
		Label:  field.Label,
		Fields: []pkgplugin.TUIField{field},
	})
	form.onSubmit = func(values map[string]string) {
		for key, value := range values {
			t.settingsValues[key] = value
		}
		t.settings.back()
		t.saveSettings()
	}
	t.settings.openSub(form)
}

func (t *extTab) cycleSetting(index, delta int) bool {
	if index < 0 || index >= len(t.page.Settings) {
		return false
	}
	field := t.page.Settings[index]
	if len(field.Options) == 0 {
		return false
	}
	current := t.settingsValues[field.ID]
	position := 0
	for i, option := range field.Options {
		if option == current {
			position = i
			break
		}
	}
	position = (position + delta + len(field.Options)) % len(field.Options)
	t.settingsValues[field.ID] = field.Options[position]
	t.saveSettings()
	return true
}

func (t *extTab) cycleItemOption(hit *extHit, delta int) bool {
	if len(hit.fields) != 1 || len(hit.fields[0].Options) == 0 || hit.action == "" {
		return false
	}
	field := hit.fields[0]
	current := field.Value
	position := 0
	for i, option := range field.Options {
		if option == current {
			position = i
			break
		}
	}
	position = (position + delta + len(field.Options)) % len(field.Options)
	t.invokeItemAction(hit, hit.action, map[string]string{field.ID: field.Options[position]})
	return true
}

func (t *extTab) saveSettings() {
	if t.hook {
		t.settings.saveHookValues(t)
		return
	}
	if t.page.SaveAction == "" {
		return
	}
	t.settings.runExtAction(t, pkgplugin.TUIAction{
		ID:     t.page.SaveAction,
		Label:  "Save settings",
		Fields: t.page.Settings,
	}, t.settingsValues)
}

func (t *extTab) HandleMouse(x, y int, buttons tcell.ButtonMask) bool {
	if t.doc == nil {
		return false
	}
	switch {
	case buttons&tcell.WheelUp != 0:
		if t.scroll > 0 {
			t.scroll--
		}
		return true
	case buttons&tcell.WheelDown != 0:
		t.scroll++
		return true
	}
	if buttons&tcell.ButtonPrimary == 0 {
		return false
	}
	idx := t.scroll + (y - t.drawTop)
	if idx < 0 || idx >= len(t.doc.rows) {
		return false
	}
	for _, seg := range t.doc.rows[idx].segs {
		if seg.hit != nil {
			t.selectHit(seg.hit)
			t.activate()
			return true
		}
	}
	return false
}

func (t *extTab) Draw(screen tcell.Screen, bounds layout.Region, focused bool) {
	th := styles.Current()
	components.FillRegion(screen, bounds, ' ', th.Base().Background(th.InputBg))

	t.ensureDoc(bounds.Width)

	listHeight := bounds.Height - 1
	if listHeight < 0 {
		listHeight = 0
	}
	list := layout.Region{Left: bounds.Left, Top: bounds.Top, Width: bounds.Width, Height: listHeight}
	t.drawTop = list.Top
	t.drawHeight = list.Height
	t.ensureVisible(list)

	selected := t.selectedHit()
	for i := 0; i < list.Height; i++ {
		idx := t.scroll + i
		if idx >= len(t.doc.rows) {
			break
		}
		y := list.Top + i
		x := list.Left
		for _, seg := range t.doc.rows[idx].segs {
			if x >= list.Right() {
				break
			}
			style := seg.style
			if seg.hit != nil && seg.hit == selected {
				style = th.Base().Foreground(th.InputText).Background(th.Selection)
			}
			text := seg.text
			if x+components.DisplayWidth(text) > list.Right() {
				text = components.TruncateTo(text, list.Right()-x)
			}
			components.DrawText(screen, x, y, text, style)
			x += components.DisplayWidth(text)
		}
	}

	if bounds.Height > 0 {
		components.DrawFooterRight(screen, bounds, t.footerText())
	}
}

func (t *extTab) ensureVisible(list layout.Region) {
	hit := t.selectedHit()
	if hit == nil || t.doc == nil {
		return
	}
	start, end, ok := t.doc.spanFor(hit)
	if !ok {
		return
	}
	visible := list.Height
	if visible <= 0 {
		return
	}
	if start < t.scroll {
		t.scroll = start
	}
	if end >= t.scroll+visible {
		t.scroll = end - visible + 1
	}
	if t.scroll < 0 {
		t.scroll = 0
	}
	maxScroll := len(t.doc.rows) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scroll > maxScroll {
		t.scroll = maxScroll
	}
}

func (t *extTab) footerText() string {
	parts := []string{}
	hit := t.selectedHit()
	if hit != nil {
		switch hit.kind {
		case hitItem:
			if len(hit.fields) > 0 && hit.action != "" {
				parts = append(parts, "e edit")
				if len(hit.fields) == 1 && len(hit.fields[0].Options) > 0 {
					parts = append(parts, "←/→ change")
				}
			}
			if hit.meta["runAction"] != "" {
				parts = append(parts, "x run")
			}
			if hit.meta["testAction"] != "" {
				parts = append(parts, "t test")
			}
			if hit.meta["deleteAction"] != "" {
				parts = append(parts, "d delete")
			}
		case hitSetting:
			parts = append(parts, "enter edit")
		}
	}
	parts = append(parts, "r refresh", "esc close")
	return strings.Join(parts, "  ·  ")
}

func (s *Settings) reloadExtTabs() {
	pages := ext.Pages()
	s.extTabs = s.extTabs[:0]
	for _, page := range pages {
		s.extTabs = append(s.extTabs, newExtTab(s, page))
	}
	s.rebuildSlots()
}

func (s *Settings) tabLabels() []string {
	labels := make([]string, 0, len(s.slots))
	for _, slot := range s.slots {
		if slot.ext != nil {
			labels = append(labels, slot.ext.Title())
			continue
		}
		labels = append(labels, builtinTabTitle(slot.kind))
	}
	return labels
}

func (s *Settings) extTabAt(index int) *extTab {
	if index < 0 || index >= len(s.slots) {
		return nil
	}
	return s.slots[index].ext
}

func (s *Settings) kindAt(index int) (tabKind, bool) {
	if index < 0 || index >= len(s.slots) || s.slots[index].ext != nil {
		return 0, false
	}
	return s.slots[index].kind, true
}

func (s *Settings) refreshExtPage(tab *extTab) {
	s.errMsg = ""
	s.noticeMsg = "Refreshing..."
	s.requestRedrawFn()
	if s.deps.Plugin == nil {
		s.noticeMsg = ""
		s.errMsg = "plugin manager unavailable"
		s.requestRedrawFn()
		return
	}

	owner := tab.page.Owner
	pageID := tab.page.ID
	go func() {
		pages, err := s.deps.Plugin.RefreshTUIPages(context.Background(), owner)
		s.dispatch(func() {
			if err != nil {
				s.noticeMsg = ""
				s.errMsg = err.Error()
				s.requestRedrawFn()
				return
			}
			for _, page := range pages {
				if page.ID == pageID {
					tab.SetPage(page)
					break
				}
			}
			s.errMsg = ""
			s.noticeMsg = "Refreshed"
			s.requestRedrawFn()
		})
	}()
}

func (s *Settings) requestRedrawFn() {
	if s.requestRedraw != nil {
		s.requestRedraw()
	}
}
