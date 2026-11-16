package acp

import (
	"context"
	"strings"
	"testing"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type fakePluginPages struct {
	pages      map[string][]pkgplugin.TUIPage
	owners     []string
	refreshErr error
	result     *pkgplugin.TUIActionResult
	actionErr  error
	lastOwner  string
	lastReq    pkgplugin.TUIActionRequest
}

func (f *fakePluginPages) Pages(owner string) []pkgplugin.TUIPage {
	return f.pages[owner]
}

func (f *fakePluginPages) PageOwners() []string {
	return f.owners
}

func (f *fakePluginPages) RefreshTUIPages(_ context.Context, name string) ([]pkgplugin.TUIPage, error) {
	f.lastOwner = name
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.pages[name], nil
}

func (f *fakePluginPages) ExecuteTUIAction(_ context.Context, name string, req pkgplugin.TUIActionRequest) (*pkgplugin.TUIActionResult, error) {
	f.lastOwner = name
	f.lastReq = req
	if f.actionErr != nil {
		return nil, f.actionErr
	}
	return f.result, nil
}

func kanbanTestPage() pkgplugin.TUIPage {
	return pkgplugin.TUIPage{
		ID:          "kanban",
		Title:       "Kanban",
		Description: "Plan work",
		Owner:       "kanban",
		Metrics:     []pkgplugin.TUIMetric{{Label: "Cards", Value: "3"}},
		Sections: []pkgplugin.TUISection{{
			Title: "Board",
			Kind:  "board",
			Items: []pkgplugin.TUIItem{{
				ID:     "card-1",
				Title:  "Ship it",
				Column: "backlog",
				Action: "card.update",
				Fields: []pkgplugin.TUIField{{ID: "column", Value: "backlog"}},
				Meta:   map[string]string{"runAction": "card.run"},
			}},
		}},
		Actions:  []pkgplugin.TUIAction{{ID: "card.add", Label: "Add card"}},
		Settings: []pkgplugin.TUIField{{ID: "port", Value: "7878"}},
	}
}

func TestPluginPagesList(t *testing.T) {
	srv := &Server{plugins: &fakePluginPages{
		owners: []string{"kanban", "notify"},
		pages: map[string][]pkgplugin.TUIPage{
			"kanban": {kanbanTestPage()},
			"notify": {{ID: "notify", Title: "Notify", Owner: "notify"}},
		},
	}}

	result, err := srv.handlePluginPagesList(context.Background(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	list, ok := result.(pluginPagesListResult)
	if !ok {
		t.Fatalf("unexpected result %T", result)
	}
	if len(list.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(list.Pages))
	}
	if list.Pages[0].Owner != "kanban" || list.Pages[1].Owner != "notify" {
		t.Fatalf("pages not sorted by owner: %+v", list.Pages)
	}
	page := list.Pages[0]
	if page.Title != "Kanban" || len(page.Sections) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	item := page.Sections[0].Items[0]
	if item.Column != "backlog" || item.Meta["runAction"] != "card.run" || item.Action != "card.update" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if len(page.Settings) != 1 || page.Settings[0].ID != "port" {
		t.Fatalf("unexpected settings: %+v", page.Settings)
	}
}

func TestPluginPagesListOwnerFilter(t *testing.T) {
	fake := &fakePluginPages{
		owners: []string{"kanban"},
		pages:  map[string][]pkgplugin.TUIPage{"kanban": {kanbanTestPage()}},
	}
	srv := &Server{plugins: fake}

	result, err := srv.handlePluginPagesList(context.Background(), toJSON(t, pluginPagesListParams{Owner: "kanban"}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	list := result.(pluginPagesListResult)
	if len(list.Pages) != 1 || list.Pages[0].Owner != "kanban" {
		t.Fatalf("unexpected pages: %+v", list.Pages)
	}
}

func TestPluginPageGetRefreshes(t *testing.T) {
	fake := &fakePluginPages{
		pages: map[string][]pkgplugin.TUIPage{"kanban": {kanbanTestPage()}},
	}
	srv := &Server{plugins: fake}

	result, err := srv.handlePluginPageGet(context.Background(), toJSON(t, pluginPageGetParams{Owner: "kanban", PageID: "kanban"}))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fake.lastOwner != "kanban" {
		t.Fatalf("expected refresh for kanban, got %q", fake.lastOwner)
	}
	page := result.(pluginPageGetResult).Page
	if page.ID != "kanban" || page.Title != "Kanban" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestPluginPageGetUnknown(t *testing.T) {
	srv := &Server{plugins: &fakePluginPages{pages: map[string][]pkgplugin.TUIPage{"kanban": {kanbanTestPage()}}}}

	_, err := srv.handlePluginPageGet(context.Background(), toJSON(t, pluginPageGetParams{Owner: "kanban", PageID: "missing"}))
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected RPCError, got %T", err)
	}
	if rpcErr.Code != ErrCodeInvalidParams {
		t.Fatalf("expected invalid params, got %d", rpcErr.Code)
	}
}

func TestPluginPageGetRequiresIDs(t *testing.T) {
	srv := &Server{plugins: &fakePluginPages{}}

	_, err := srv.handlePluginPageGet(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginPageAction(t *testing.T) {
	page := kanbanTestPage()
	fake := &fakePluginPages{result: &pkgplugin.TUIActionResult{Output: "moved", Page: &page}}
	srv := &Server{plugins: fake}

	result, err := srv.handlePluginPageAction(context.Background(), toJSON(t, pluginPageActionParams{
		Owner:    "kanban",
		PageID:   "kanban",
		ActionID: "card.update",
		Values:   map[string]string{"id": "card-1", "column": "ready"},
	}))
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	if fake.lastReq.PageID != "kanban" || fake.lastReq.ActionID != "card.update" {
		t.Fatalf("unexpected request: %+v", fake.lastReq)
	}
	if fake.lastReq.Values["column"] != "ready" {
		t.Fatalf("unexpected values: %+v", fake.lastReq.Values)
	}
	res := result.(pluginPageActionResult)
	if res.Output != "moved" || res.Page == nil || res.Page.ID != "kanban" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestPluginPageActionRequiresIDs(t *testing.T) {
	srv := &Server{plugins: &fakePluginPages{}}

	_, err := srv.handlePluginPageAction(context.Background(), toJSON(t, pluginPageActionParams{Owner: "kanban"}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginPagesUnavailable(t *testing.T) {
	srv := &Server{}

	_, err := srv.handlePluginPagesList(context.Background(), nil)
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected RPCError, got %T", err)
	}
	if rpcErr.Code != ErrCodeInternal {
		t.Fatalf("expected internal error, got %d", rpcErr.Code)
	}
}

func TestPluginPagesDispatch(t *testing.T) {
	srv := newTestServer(t)
	srv.transport = newBufferTransport()
	initServer(t, srv)
	srv.plugins = &fakePluginPages{
		owners: []string{"kanban"},
		pages:  map[string][]pkgplugin.TUIPage{"kanban": {kanbanTestPage()}},
	}

	resp := sendRequest(t, srv, context.Background(), "_pegg/plugins/pages/list", map[string]any{})
	if !strings.Contains(resp, `"id":"kanban"`) {
		t.Fatalf("expected kanban page in response, got %s", resp)
	}
	if !strings.Contains(resp, `"column":"backlog"`) {
		t.Fatalf("expected board column in response, got %s", resp)
	}
}
