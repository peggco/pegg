//go:build gui

package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/plugin"
	"github.com/peggco/pegg/internal/tui/ext"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type PluginSettingField struct {
	ID          string   `json:"id"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       string   `json:"value,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type PluginSettingAction struct {
	ID          string               `json:"id"`
	Label       string               `json:"label"`
	Description string               `json:"description,omitempty"`
	Confirm     bool                 `json:"confirm,omitempty"`
	Fields      []PluginSettingField `json:"fields,omitempty"`
}

type PluginSettingMetric struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	Badge  string `json:"badge,omitempty"`
}

type PluginSettingItem struct {
	ID       string               `json:"id,omitempty"`
	Title    string               `json:"title"`
	Detail   string               `json:"detail,omitempty"`
	Badge    string               `json:"badge,omitempty"`
	Disabled bool                 `json:"disabled,omitempty"`
	Fields   []PluginSettingField `json:"fields,omitempty"`
	Action   string               `json:"action,omitempty"`
	Column   string               `json:"column,omitempty"`
	Meta     map[string]string    `json:"meta,omitempty"`
}

type PluginSettingSection struct {
	Title string              `json:"title"`
	Kind  string              `json:"kind,omitempty"`
	Items []PluginSettingItem `json:"items"`
	Note  string              `json:"note,omitempty"`
}

type PluginSettingPage struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Owner       string                 `json:"owner"`
	Layout      string                 `json:"layout,omitempty"`
	Metrics     []PluginSettingMetric  `json:"metrics,omitempty"`
	Sections    []PluginSettingSection `json:"sections"`
	Actions     []PluginSettingAction  `json:"actions"`
	Settings    []PluginSettingField   `json:"settings,omitempty"`
	SaveAction  string                 `json:"saveAction,omitempty"`
	Footer      string                 `json:"footer,omitempty"`
}

type PluginSettingResult struct {
	Output string             `json:"output,omitempty"`
	Page   *PluginSettingPage `json:"page,omitempty"`
}

func (a *App) pluginManager() (*plugin.Manager, error) {
	a.pluginMu.Lock()
	defer a.pluginMu.Unlock()
	if a.pluginMgr != nil {
		return a.pluginMgr, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if !cfg.Plugins.Enabled {
		return nil, nil
	}

	mgr := plugin.NewManager(logger.New(logger.LevelWarn), cfg, nil, nil, nil, nil, nil, nil, nil)
	mgr.SetWorkspaceRoot(a.currentWorkspace())
	mgr.SetExcludedPlugins(cfg.Plugins.Exclude)
	if err := mgr.LoadPlugins(); err != nil {
		mgr.Close()
		return nil, err
	}
	a.pluginMgr = mgr
	return mgr, nil
}

func (a *App) resetPluginManager() {
	a.pluginMu.Lock()
	mgr := a.pluginMgr
	a.pluginMgr = nil
	a.pluginMu.Unlock()
	if mgr != nil {
		mgr.Close()
	}
	a.refreshDesktopView()
}

func (a *App) PluginSettingsPages() ([]PluginSettingPage, error) {
	mgr, err := a.pluginManager()
	if err != nil {
		return []PluginSettingPage{}, err
	}
	if mgr == nil {
		return []PluginSettingPage{}, nil
	}
	return pluginPages(), nil
}

func (a *App) PluginSettingsRefresh() ([]PluginSettingPage, error) {
	mgr, err := a.pluginManager()
	if err != nil {
		return []PluginSettingPage{}, err
	}
	if mgr == nil {
		return []PluginSettingPage{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, instance := range mgr.ListPlugins() {
		if !pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
			continue
		}
		_, _ = mgr.RefreshTUIPages(ctx, instance.Name)
	}
	return pluginPages(), nil
}

func (a *App) PluginSettingsAction(owner, pageID, actionID string, values map[string]string) (PluginSettingResult, error) {
	if owner == desktopViewOwner {
		if a.viewHooks != nil {
			a.viewHooks.saveSettings(strings.TrimPrefix(pageID, "view:"), values)
		}
		return PluginSettingResult{}, nil
	}
	mgr, err := a.pluginManager()
	if err != nil {
		return PluginSettingResult{}, err
	}
	if mgr == nil {
		return PluginSettingResult{}, errors.New("plugins are disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := mgr.ExecuteTUIAction(ctx, owner, pkgplugin.TUIActionRequest{
		PageID:   pageID,
		ActionID: actionID,
		Values:   values,
	})
	if err != nil {
		return PluginSettingResult{}, err
	}
	result := PluginSettingResult{}
	if res != nil {
		result.Output = res.Output
		if res.Page != nil {
			page := toPluginSettingPage(*res.Page)
			result.Page = &page
		}
	}
	return result, nil
}

func pluginPages() []PluginSettingPage {
	pages := ext.Pages()
	out := make([]PluginSettingPage, 0, len(pages))
	for _, page := range pages {
		out = append(out, toPluginSettingPage(page))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Owner != out[j].Owner {
			return out[i].Owner < out[j].Owner
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func toPluginSettingPage(page pkgplugin.TUIPage) PluginSettingPage {
	out := PluginSettingPage{
		ID:          page.ID,
		Title:       page.Title,
		Description: page.Description,
		Owner:       page.Owner,
		Layout:      page.Layout,
		Footer:      page.Footer,
		Sections:    make([]PluginSettingSection, 0, len(page.Sections)),
		Actions:     make([]PluginSettingAction, 0, len(page.Actions)),
		Settings:    toPluginSettingFields(page.Settings),
		SaveAction:  page.SaveAction,
	}
	for _, metric := range page.Metrics {
		out.Metrics = append(out.Metrics, PluginSettingMetric{
			Label:  metric.Label,
			Value:  metric.Value,
			Detail: metric.Detail,
			Badge:  metric.Badge,
		})
	}
	for _, section := range page.Sections {
		entry := PluginSettingSection{
			Title: section.Title,
			Kind:  section.Kind,
			Note:  section.Note,
			Items: make([]PluginSettingItem, 0, len(section.Items)),
		}
		for _, item := range section.Items {
			entry.Items = append(entry.Items, PluginSettingItem{
				ID:       item.ID,
				Title:    item.Title,
				Detail:   item.Detail,
				Badge:    item.Badge,
				Disabled: item.Disabled,
				Fields:   toPluginSettingFields(item.Fields),
				Action:   item.Action,
				Column:   item.Column,
				Meta:     item.Meta,
			})
		}
		out.Sections = append(out.Sections, entry)
	}
	for _, action := range page.Actions {
		entry := PluginSettingAction{
			ID:          action.ID,
			Label:       action.Label,
			Description: action.Description,
			Confirm:     action.Confirm,
			Fields:      toPluginSettingFields(action.Fields),
		}
		out.Actions = append(out.Actions, entry)
	}
	return out
}

func toPluginSettingFields(fields []pkgplugin.TUIField) []PluginSettingField {
	out := make([]PluginSettingField, 0, len(fields))
	for _, field := range fields {
		out = append(out, PluginSettingField{
			ID:          field.ID,
			Label:       field.Label,
			Placeholder: field.Placeholder,
			Value:       field.Value,
			Secret:      field.Secret,
			Required:    field.Required,
			Options:     append([]string(nil), field.Options...),
		})
	}
	return out
}
