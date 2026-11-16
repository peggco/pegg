package acp

import (
	"context"
	"fmt"
	"sort"

	json "github.com/goccy/go-json"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type PluginPageProvider interface {
	Pages(owner string) []pkgplugin.TUIPage
	PageOwners() []string
	RefreshTUIPages(ctx context.Context, name string) ([]pkgplugin.TUIPage, error)
	ExecuteTUIAction(ctx context.Context, name string, req pkgplugin.TUIActionRequest) (*pkgplugin.TUIActionResult, error)
}

type pluginPageField struct {
	ID          string   `json:"id"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       string   `json:"value,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type pluginPageAction struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
	Fields      []pluginPageField `json:"fields,omitempty"`
}

type pluginPageMetric struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	Badge  string `json:"badge,omitempty"`
}

type pluginPageItem struct {
	ID       string            `json:"id,omitempty"`
	Title    string            `json:"title"`
	Detail   string            `json:"detail,omitempty"`
	Badge    string            `json:"badge,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Fields   []pluginPageField `json:"fields,omitempty"`
	Action   string            `json:"action,omitempty"`
	Column   string            `json:"column,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

type pluginPageSection struct {
	Title string           `json:"title"`
	Kind  string           `json:"kind,omitempty"`
	Items []pluginPageItem `json:"items"`
	Note  string           `json:"note,omitempty"`
}

type pluginPage struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Owner       string              `json:"owner"`
	Layout      string              `json:"layout,omitempty"`
	Metrics     []pluginPageMetric  `json:"metrics,omitempty"`
	Sections    []pluginPageSection `json:"sections"`
	Actions     []pluginPageAction  `json:"actions"`
	Settings    []pluginPageField   `json:"settings,omitempty"`
	SaveAction  string              `json:"saveAction,omitempty"`
	Footer      string              `json:"footer,omitempty"`
}

type pluginPagesListParams struct {
	Owner string `json:"owner,omitempty"`
}

type pluginPagesListResult struct {
	Pages []pluginPage `json:"pages"`
}

type pluginPageGetParams struct {
	Owner  string `json:"owner"`
	PageID string `json:"pageId"`
}

type pluginPageGetResult struct {
	Page pluginPage `json:"page"`
}

type pluginPageActionParams struct {
	Owner    string            `json:"owner"`
	PageID   string            `json:"pageId"`
	ActionID string            `json:"actionId"`
	Values   map[string]string `json:"values,omitempty"`
}

type pluginPageActionResult struct {
	Output string      `json:"output,omitempty"`
	Page   *pluginPage `json:"page,omitempty"`
}

func (s *Server) handlePluginPagesList(_ context.Context, params json.RawMessage) (any, error) {
	if s.plugins == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "plugin pages are not available"}
	}
	var req pluginPagesListParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}

	owners := []string{req.Owner}
	if req.Owner == "" {
		owners = s.plugins.PageOwners()
	}

	pages := make([]pluginPage, 0)
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		for _, page := range s.plugins.Pages(owner) {
			pages = append(pages, toPluginPage(page))
		}
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Owner != pages[j].Owner {
			return pages[i].Owner < pages[j].Owner
		}
		return pages[i].Title < pages[j].Title
	})
	return pluginPagesListResult{Pages: pages}, nil
}

func (s *Server) handlePluginPageGet(ctx context.Context, params json.RawMessage) (any, error) {
	if s.plugins == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "plugin pages are not available"}
	}
	var req pluginPageGetParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	if req.Owner == "" || req.PageID == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "owner and pageId are required"}
	}

	pages, err := s.plugins.RefreshTUIPages(ctx, req.Owner)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: err.Error()}
	}
	for _, page := range pages {
		if page.ID == req.PageID {
			return pluginPageGetResult{Page: toPluginPage(page)}, nil
		}
	}
	return nil, &RPCError{Code: ErrCodeInvalidParams, Message: fmt.Sprintf("plugin page %q not found", req.PageID)}
}

func (s *Server) handlePluginPageAction(ctx context.Context, params json.RawMessage) (any, error) {
	if s.plugins == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "plugin pages are not available"}
	}
	var req pluginPageActionParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &req)
	}
	if req.Owner == "" || req.PageID == "" || req.ActionID == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "owner, pageId and actionId are required"}
	}

	res, err := s.plugins.ExecuteTUIAction(ctx, req.Owner, pkgplugin.TUIActionRequest{
		PageID:   req.PageID,
		ActionID: req.ActionID,
		Values:   req.Values,
	})
	if err != nil {
		return nil, err
	}
	out := pluginPageActionResult{}
	if res != nil {
		out.Output = res.Output
		if res.Page != nil {
			page := toPluginPage(*res.Page)
			out.Page = &page
		}
	}
	return out, nil
}

func toPluginPage(page pkgplugin.TUIPage) pluginPage {
	out := pluginPage{
		ID:          page.ID,
		Title:       page.Title,
		Description: page.Description,
		Owner:       page.Owner,
		Layout:      page.Layout,
		Sections:    make([]pluginPageSection, 0, len(page.Sections)),
		Actions:     make([]pluginPageAction, 0, len(page.Actions)),
		Settings:    toPluginPageFields(page.Settings),
		SaveAction:  page.SaveAction,
		Footer:      page.Footer,
	}
	for _, metric := range page.Metrics {
		out.Metrics = append(out.Metrics, pluginPageMetric{
			Label:  metric.Label,
			Value:  metric.Value,
			Detail: metric.Detail,
			Badge:  metric.Badge,
		})
	}
	for _, section := range page.Sections {
		items := make([]pluginPageItem, 0, len(section.Items))
		for _, item := range section.Items {
			items = append(items, pluginPageItem{
				ID:       item.ID,
				Title:    item.Title,
				Detail:   item.Detail,
				Badge:    item.Badge,
				Disabled: item.Disabled,
				Fields:   toPluginPageFields(item.Fields),
				Action:   item.Action,
				Column:   item.Column,
				Meta:     item.Meta,
			})
		}
		out.Sections = append(out.Sections, pluginPageSection{
			Title: section.Title,
			Kind:  section.Kind,
			Items: items,
			Note:  section.Note,
		})
	}
	for _, action := range page.Actions {
		out.Actions = append(out.Actions, pluginPageAction{
			ID:          action.ID,
			Label:       action.Label,
			Description: action.Description,
			Confirm:     action.Confirm,
			Fields:      toPluginPageFields(action.Fields),
		})
	}
	return out
}

func toPluginPageFields(fields []pkgplugin.TUIField) []pluginPageField {
	out := make([]pluginPageField, 0, len(fields))
	for _, field := range fields {
		out = append(out, pluginPageField{
			ID:          field.ID,
			Label:       field.Label,
			Placeholder: field.Placeholder,
			Value:       field.Value,
			Secret:      field.Secret,
			Required:    field.Required,
			Options:     field.Options,
		})
	}
	return out
}
