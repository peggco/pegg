package settings

import (
	"context"
	"strings"
	"time"

	corehook "github.com/peggco/pegg/internal/core/hook"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

const (
	hookSettingsSections = "tui.settings.sections"
	hookSettingsValues   = "tui.settings.values"
	hookSettingsSave     = "tui.settings.save"
	hookSettingsTimeout  = 150 * time.Millisecond
)

type tabSlot struct {
	kind tabKind
	ext  *extTab
}

func builtinTabKey(kind tabKind) string {
	switch kind {
	case tabGeneral:
		return "general"
	case tabSession:
		return "session"
	case tabMCP:
		return "mcp"
	case tabSkills:
		return "skills"
	case tabRules:
		return "rules"
	case tabPlugins:
		return "plugins"
	case tabPermissions:
		return "permissions"
	case tabMemory:
		return "memory"
	case tabSystem:
		return "system"
	default:
		return ""
	}
}

func tabKindFromKey(key string) (tabKind, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "general":
		return tabGeneral, true
	case "session", "sessions":
		return tabSession, true
	case "mcp":
		return tabMCP, true
	case "skills":
		return tabSkills, true
	case "rules":
		return tabRules, true
	case "plugins":
		return tabPlugins, true
	case "permissions", "permission":
		return tabPermissions, true
	case "memory":
		return tabMemory, true
	case "system":
		return tabSystem, true
	default:
		return 0, false
	}
}

func builtinTabTitle(kind tabKind) string {
	names := tabNames()
	if int(kind) < 0 || int(kind) >= len(names) {
		return "Plugins"
	}
	return names[int(kind)]
}

func (s *Settings) rebuildSlots() {
	slots := make([]tabSlot, 0, builtinTabCount+len(s.hookTabs)+len(s.extTabs))
	for kind := tabKind(0); kind < builtinTabCount; kind++ {
		slots = append(slots, tabSlot{kind: kind})
		for _, tab := range s.hookTabs {
			if tab.hookTab == builtinTabKey(kind) {
				slots = append(slots, tabSlot{kind: -1, ext: tab})
			}
		}
	}
	for _, tab := range s.hookTabs {
		if tab.hookTab == "" {
			slots = append(slots, tabSlot{kind: -1, ext: tab})
		}
	}
	for _, tab := range s.extTabs {
		slots = append(slots, tabSlot{kind: -1, ext: tab})
	}
	s.slots = slots
}

func (s *Settings) reloadHookTabs() {
	previous := s.hookTabs
	s.hookTabs = nil
	if corehook.HasFilter(hookSettingsSections) {
		callCtx, cancel := context.WithTimeout(context.Background(), hookSettingsTimeout)
		out, errs := corehook.ApplyFilter(callCtx, hookSettingsSections, pkgplugin.SettingsSectionsHook{})
		cancel()
		if len(errs) > 0 {
			s.hookTabs = previous
		} else {
			s.hookTabs = s.buildHookTabs(out.Sections)
		}
	}
	s.rebuildSlots()
	if s.tabs != nil {
		active := s.tabs.Active()
		s.rebuildTabs()
		if active >= 0 && active < s.tabs.Count() {
			s.tabs.SetActive(active)
			s.tab = tabKind(active)
		}
	}
}

func (s *Settings) buildHookTabs(sections []pkgplugin.ViewSection) []*extTab {
	grouped := make(map[tabKind][]pkgplugin.ViewSection)
	var extras []pkgplugin.ViewSection
	for _, section := range sections {
		key, ok := tabKindFromKey(section.Tab)
		if !ok {
			extras = append(extras, section)
			continue
		}
		grouped[key] = append(grouped[key], section)
	}

	tabs := make([]*extTab, 0, len(grouped)+1)
	for kind := tabKind(0); kind < builtinTabCount; kind++ {
		if len(grouped[kind]) == 0 {
			continue
		}
		tabs = append(tabs, newHookTab(s, builtinTabKey(kind), builtinTabTitle(kind), grouped[kind]))
	}
	if len(extras) > 0 {
		tabs = append(tabs, newHookTab(s, "", "Extensions", extras))
	}
	return tabs
}

func newHookTab(s *Settings, key, title string, sections []pkgplugin.ViewSection) *extTab {
	page := hookPage(key, title, sections)
	title = title + " · Plugins"
	page.Title = title
	tab := newExtTab(s, page)
	tab.hook = true
	tab.hookTab = key
	return tab
}

func hookPage(key, title string, sections []pkgplugin.ViewSection) pkgplugin.TUIPage {
	id := "hook-settings"
	if key != "" {
		id += ":" + key
	}
	page := pkgplugin.TUIPage{
		ID:         id,
		Title:      title,
		SaveAction: hookSettingsSave,
	}
	for _, section := range sections {
		page.Sections = append(page.Sections, pkgplugin.TUISection{
			Title: section.Title,
			Kind:  "plugin",
			Note:  section.Description,
		})
		page.Settings = append(page.Settings, section.Fields...)
	}
	return page
}

func (s *Settings) saveHookValues(tab *extTab) {
	values := tab.settingsValues
	if corehook.HasFilter(hookSettingsValues) {
		callCtx, cancel := context.WithTimeout(context.Background(), hookSettingsTimeout)
		out, errs := corehook.ApplyFilter(callCtx, hookSettingsValues, pkgplugin.SettingsValuesHook{
			Tab:    tab.hookTab,
			Values: values,
		})
		cancel()
		if len(errs) == 0 && out.Values != nil {
			values = out.Values
		}
	}
	if corehook.HasAction(hookSettingsSave) {
		callCtx, cancel := context.WithTimeout(context.Background(), hookSettingsTimeout)
		_, errs := corehook.ApplyAction(callCtx, hookSettingsSave, pkgplugin.SettingsSaveHook{
			Tab:    tab.hookTab,
			Values: values,
		})
		cancel()
		if len(errs) > 0 {
			s.errMsg = errs[0].Error()
		} else {
			s.noticeMsg = "Saved"
		}
	}
	tab.settingsValues = values
	s.reloadHookTabs()
	s.requestRedrawFn()
}
