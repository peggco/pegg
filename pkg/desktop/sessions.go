//go:build gui

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
	"github.com/peggco/pegg/pkg/desktop/internal/binary"
	"github.com/peggco/pegg/pkg/desktop/internal/prefs"
	"github.com/peggco/pegg/pkg/desktop/internal/workspace"
)

func (a *App) ensureClient() (*acpclient.Client, error) {
	a.mu.Lock()
	if a.client != nil && a.client.Running() {
		c := a.client
		a.mu.Unlock()
		return c, nil
	}
	a.mu.Unlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil && a.client.Running() {
		return a.client, nil
	}
	if a.closing {
		return nil, errors.New("app is shutting down")
	}
	st := a.binary
	if !st.Found {
		p := a.prefs.Get()
		st = binary.Resolve(context.Background(), p.BinaryPath, p.Workspace)
		a.binary = st
	}
	if !st.Found {
		return nil, fmt.Errorf("pegg binary not found: %s", st.Error)
	}
	cwd := a.currentWorkspace()
	if _, err := workspace.Valid(cwd); err != nil {
		cwd, _ = os.UserHomeDir()
	}
	client := acpclient.New(acpclient.Options{
		Binary:  st.Path,
		Cwd:     cwd,
		Version: appVersion,
		Callbacks: acpclient.Callbacks{
			OnUpdate:        a.enqueueUpdate,
			OnServerRequest: a.handleServerRequest,
			OnStatus: func(s acpclient.Status) {
				a.logf("acp status: %s %s", s.State, s.Message)
				a.emit("acp:status", s)
				if s.State == "exited" || s.State == "error" {
					a.clearPending()
				}
			},
			ReadTextFile: workspace.ReadTextFile,
			Logf:         a.logf,
		},
	})
	if err := client.Start(context.Background()); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := client.Request(ctx, "initialize", acpclient.InitializeParams{
		ProtocolVersion: 1,
		ClientInfo:      acpclient.ClientInfo{Name: "pegg-desktop", Title: "Pegg Desktop", Version: appVersion},
		ClientCapabilities: acpclient.ClientCapabilities{
			FS:       &acpclient.FSCapabilities{ReadTextFile: true, WriteTextFile: false},
			Terminal: true,
		},
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}
	var initRes acpclient.InitializeResult
	if err := json.Unmarshal(raw, &initRes); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize response: %w", err)
	}
	a.client = client
	a.clientAt = cwd
	a.initRes = &initRes
	a.logf("acp initialized: %+v", initRes.AgentInfo)
	return client, nil
}

func (a *App) Capabilities() *acpclient.PeggCapabilities {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.initRes == nil || a.initRes.AgentCapabilities.Pegg == nil {
		return &acpclient.PeggCapabilities{Ask: true, Permissions: true, Models: true, Commands: true, Attachments: true, Diffs: true, Plan: true, History: true}
	}
	return a.initRes.AgentCapabilities.Pegg
}

func (a *App) enqueueUpdate(sessionID string, raw json.RawMessage) {
	cp := append(json.RawMessage(nil), raw...)
	a.updatesMu.Lock()
	a.updates = append(a.updates, updateItem{SessionID: sessionID, Update: cp})
	n := len(a.updates)
	a.updatesMu.Unlock()
	if n >= 1024 {
		a.flushUpdates()
	}
}

func (a *App) flushLoop(ctx context.Context) {
	t := time.NewTicker(16 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.flushUpdates()
		}
	}
}

func (a *App) flushUpdates() {
	a.updatesMu.Lock()
	if len(a.updates) == 0 {
		a.updatesMu.Unlock()
		return
	}
	batch := a.updates
	a.updates = nil
	a.updatesMu.Unlock()

	type group struct {
		updates []json.RawMessage
	}
	groups := map[string]*group{}
	order := make([]string, 0, 4)
	for _, item := range batch {
		g, ok := groups[item.SessionID]
		if !ok {
			g = &group{}
			groups[item.SessionID] = g
			order = append(order, item.SessionID)
		}
		g.updates = append(g.updates, item.Update)
	}
	for _, sid := range order {
		payload := struct {
			SessionID string            `json:"sessionId"`
			Updates   []json.RawMessage `json:"updates"`
		}{SessionID: sid, Updates: groups[sid].updates}
		raw, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		a.emit("acp:update", string(raw))
	}
}

func (a *App) handleServerRequest(id, method string, params json.RawMessage) {
	switch method {
	case "session/request_permission":
		a.sessionsMu.Lock()
		a.pending[id] = pendingRequest{ID: id, Method: method}
		a.sessionsMu.Unlock()
		a.emit("acp:permission", map[string]any{"requestId": id, "params": json.RawMessage(params)})
	case "pegg/ask":
		a.sessionsMu.Lock()
		a.pending[id] = pendingRequest{ID: id, Method: method}
		a.sessionsMu.Unlock()
		a.emit("acp:ask", map[string]any{"requestId": id, "params": json.RawMessage(params)})
	default:
		a.mu.Lock()
		c := a.client
		a.mu.Unlock()
		if c != nil {
			_ = c.Respond(id, nil, &acpclient.RPCError{Code: -32601, Message: "unsupported client method: " + method})
		}
	}
}

func (a *App) clearPending() {
	a.sessionsMu.Lock()
	a.pending = map[string]pendingRequest{}
	a.sessionsMu.Unlock()
	a.emit("acp:pending-clear", nil)
}

func (a *App) RespondPermission(requestID, optionID string) error {
	a.sessionsMu.Lock()
	req, ok := a.pending[requestID]
	if ok && req.Method == "session/request_permission" {
		delete(a.pending, requestID)
	}
	a.sessionsMu.Unlock()
	if !ok {
		return errors.New("permission request is no longer active")
	}
	a.mu.Lock()
	c := a.client
	a.mu.Unlock()
	if c == nil {
		return acpclient.ErrClosed
	}
	return c.Respond(requestID, map[string]any{
		"outcome": map[string]string{"outcome": "selected", "optionId": optionID},
	})
}

func (a *App) RespondAsk(requestID string, answers map[string]string) error {
	a.sessionsMu.Lock()
	req, ok := a.pending[requestID]
	if ok && req.Method == "pegg/ask" {
		delete(a.pending, requestID)
	}
	a.sessionsMu.Unlock()
	if !ok {
		return errors.New("question is no longer active")
	}
	if answers == nil {
		answers = map[string]string{}
	}
	a.mu.Lock()
	c := a.client
	a.mu.Unlock()
	if c == nil {
		return acpclient.ErrClosed
	}
	return c.Respond(requestID, map[string]any{"answers": answers})
}

func (a *App) clientOrErr() (*acpclient.Client, error) {
	a.mu.Lock()
	c := a.client
	a.mu.Unlock()
	if c == nil || !c.Running() {
		return nil, acpclient.ErrClosed
	}
	return c, nil
}

func (a *App) sessionByACP(id string) *SessionInfo {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	return a.sessions[id]
}

func (a *App) sessionByPegg(id string) *SessionInfo {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	for _, s := range a.sessions {
		if s.PeggID == id {
			return s
		}
	}
	return nil
}

func (a *App) resolveSession(id string) (*SessionInfo, error) {
	if id == "" {
		a.sessionsMu.Lock()
		id = a.current
		a.sessionsMu.Unlock()
	}
	if s := a.sessionByACP(id); s != nil {
		return s, nil
	}
	if s := a.sessionByPegg(id); s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("session not found: %s", id)
}

func (a *App) rememberSession(s *SessionInfo) {
	a.sessionsMu.Lock()
	a.sessions[s.ACPID] = s
	a.current = s.ACPID
	a.sessionsMu.Unlock()
}

func (a *App) CurrentSession() *SessionInfo {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	return a.sessions[a.current]
}

func (a *App) ResetSession() {
	a.sessionsMu.Lock()
	a.current = ""
	a.sessionsMu.Unlock()
}

func (a *App) ForgetSession(acpID string) {
	a.sessionsMu.Lock()
	delete(a.sessions, acpID)
	if a.current == acpID {
		a.current = ""
	}
	a.sessionsMu.Unlock()
}

func (a *App) saveLastSession(id string) {
	if id != "" {
		_ = a.prefs.SetLastSession(id)
	}
}

func (a *App) SavePrefsOnly(p prefs.Prefs) prefs.Prefs {
	_ = a.prefs.Update(func(cur *prefs.Prefs) {
		cur.SendOnEnter = p.SendOnEnter
		cur.AutoDownload = p.AutoDownload
		if p.FontScale > 0 {
			cur.FontScale = p.FontScale
		}
		cur.SidebarCollapsed = p.SidebarCollapsed
	})
	return a.prefs.Get()
}

type NewSessionRequest struct {
	Cwd             string `json:"cwd"`
	Mode            string `json:"mode,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

func (a *App) NewSession(req NewSessionRequest) (*SessionInfo, error) {
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}
	cwd := strings.TrimSpace(req.Cwd)
	if cwd == "" {
		cwd = a.currentWorkspace()
	}
	abs, err := workspace.Valid(cwd)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	raw, err := client.Request(ctx, "session/new", acpclient.SessionNewParams{Cwd: abs})
	if err != nil {
		return nil, err
	}
	var res acpclient.SessionNewResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	info := &SessionInfo{
		ACPID:          res.SessionID,
		PeggID:         res.PeggSessionID,
		Cwd:            abs,
		Title:          "New session",
		Mode:           res.Mode,
		Provider:       res.Provider,
		Model:          res.Model,
		PermissionMode: res.PermissionMode,
		Commands:       res.Commands,
	}
	if info.Mode == "" {
		info.Mode = "auto"
	}
	if req.Mode != "" && req.Mode != info.Mode {
		if err := a.setMode(client, info, req.Mode); err != nil {
			a.logf("set mode: %v", err)
		}
	}
	if req.Provider != "" && req.Model != "" {
		if err := a.setModel(client, info, req.Provider, req.Model, req.ReasoningEffort); err != nil {
			a.logf("set model: %v", err)
		}
	}
	a.rememberSession(info)
	_ = a.prefs.TouchWorkspace(abs, workspace.Name(abs))
	a.saveLastSession(info.PeggID)
	return info, nil
}

type LoadSessionRequest struct {
	PeggID   string `json:"peggId"`
	Cwd      string `json:"cwd,omitempty"`
	Title    string `json:"title,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

func (a *App) LoadSession(req LoadSessionRequest) (*SessionInfo, error) {
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.PeggID) == "" {
		return nil, errors.New("session id is required")
	}
	cwd := strings.TrimSpace(req.Cwd)
	if cwd == "" {
		cwd = a.currentWorkspace()
	}
	abs, err := workspace.Valid(cwd)
	if err != nil {
		abs, _ = os.UserHomeDir()
	}
	a.emit("acp:loading", map[string]any{"sessionId": req.PeggID, "loading": true})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	raw, err := client.Request(ctx, "session/load", acpclient.SessionLoadParams{
		SessionID:    req.PeggID,
		Cwd:          abs,
		HistoryLimit: 50,
	})
	a.emit("acp:loading", map[string]any{"sessionId": req.PeggID, "loading": false})
	if err != nil {
		return nil, err
	}
	var history acpclient.SessionHistoryResult
	_ = json.Unmarshal(raw, &history)
	info := &SessionInfo{
		ACPID:            req.PeggID,
		PeggID:           req.PeggID,
		Cwd:              abs,
		Title:            req.Title,
		Mode:             req.Mode,
		Provider:         req.Provider,
		Model:            req.Model,
		HistoryOldestSeq: history.OldestSeq,
		HistoryHasMore:   history.HasMore,
	}
	if info.Title == "" {
		info.Title = "Session"
	}
	a.rememberSession(info)
	a.saveLastSession(info.PeggID)
	_ = a.prefs.TouchWorkspace(abs, workspace.Name(abs))
	return info, nil
}

func (a *App) ListSessions(cwd string, all bool) (acpclient.SessionListResult, error) {
	a.logf("session/list requested all=%v cwd=%s", all, cwd)
	client, err := a.ensureClient()
	if err != nil {
		return acpclient.SessionListResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	params := acpclient.SessionListParams{All: all, Limit: 200}
	if !all {
		params.Cwd = cwd
	}
	raw, err := client.Request(ctx, "session/list", params)
	if err != nil {
		return acpclient.SessionListResult{}, err
	}
	var res acpclient.SessionListResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	a.logf("session/list all=%v cwd=%s -> %d sessions", all, cwd, len(res.Sessions))
	return res, nil
}

func (a *App) LoadSessionsPage(cwd string, all bool, offset, limit int) (acpclient.SessionListResult, error) {
	client, err := a.ensureClient()
	if err != nil {
		return acpclient.SessionListResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	raw, err := client.Request(ctx, "session/list", acpclient.SessionListParams{
		Cwd: cwd, All: all, Offset: offset, Limit: limit,
	})
	if err != nil {
		return acpclient.SessionListResult{}, err
	}
	var res acpclient.SessionListResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return acpclient.SessionListResult{}, err
	}
	return res, nil
}

type LoadHistoryRequest struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
	BeforeSeq int    `json:"beforeSeq"`
	Limit     int    `json:"limit"`
}

func (a *App) LoadHistory(req LoadHistoryRequest) (acpclient.SessionHistoryResult, error) {
	client, err := a.ensureClient()
	if err != nil {
		return acpclient.SessionHistoryResult{}, err
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return acpclient.SessionHistoryResult{}, errors.New("session id is required")
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := client.Request(ctx, "session/history", acpclient.SessionHistoryParams{
		SessionID: req.SessionID, Cwd: req.Cwd, BeforeSeq: req.BeforeSeq, Limit: req.Limit,
	})
	if err != nil {
		return acpclient.SessionHistoryResult{}, err
	}
	var res acpclient.SessionHistoryResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return acpclient.SessionHistoryResult{}, err
	}
	return res, nil
}

type PromptAttachment struct {
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Data      string `json:"data,omitempty"`
	Text      string `json:"text,omitempty"`
	Path      string `json:"path,omitempty"`
	Size      int    `json:"size,omitempty"`
}

type PromptRequest struct {
	SessionID   string             `json:"sessionId"`
	Text        string             `json:"text"`
	Attachments []PromptAttachment `json:"attachments,omitempty"`
}

func (a *App) SendPrompt(req PromptRequest) (acpclient.SessionPromptResult, error) {
	info, err := a.resolveSession(req.SessionID)
	if err != nil {
		return acpclient.SessionPromptResult{}, err
	}
	client, err := a.ensureClient()
	if err != nil {
		return acpclient.SessionPromptResult{}, err
	}
	blocks := promptBlocks(req.Text, req.Attachments)
	if len(blocks) == 0 {
		return acpclient.SessionPromptResult{}, errors.New("message is empty")
	}
	raw, err := client.Request(context.Background(), "session/prompt", acpclient.SessionPromptParams{
		SessionID: info.ACPID,
		Prompt:    blocks,
	})
	if err != nil {
		return acpclient.SessionPromptResult{}, err
	}
	var res acpclient.SessionPromptResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	a.saveLastSession(info.PeggID)
	return res, nil
}

func promptBlocks(text string, atts []PromptAttachment) []acpclient.ContentBlock {
	blocks := make([]acpclient.ContentBlock, 0, len(atts)+1)
	if strings.TrimSpace(text) != "" {
		blocks = append(blocks, acpclient.ContentBlock{Type: "text", Text: text})
	}
	for _, att := range atts {
		switch {
		case att.Text != "":
			blocks = append(blocks, acpclient.ContentBlock{
				Type: "resource",
				Resource: &acpclient.EmbeddedResource{
					URI:      fileURI(att.Path, att.Name),
					MimeType: att.MediaType,
					Text:     att.Text,
				},
			})
		case strings.HasPrefix(att.MediaType, "image/") && att.Data != "":
			blocks = append(blocks, acpclient.ContentBlock{Type: "image", Data: att.Data, MimeType: att.MediaType})
		case strings.HasPrefix(att.MediaType, "audio/") && att.Data != "":
			blocks = append(blocks, acpclient.ContentBlock{Type: "audio", Data: att.Data, MimeType: att.MediaType})
		case att.Data != "":
			blocks = append(blocks, acpclient.ContentBlock{
				Type: "resource",
				Resource: &acpclient.EmbeddedResource{
					URI:      fileURI(att.Path, att.Name),
					MimeType: att.MediaType,
					Blob:     att.Data,
				},
			})
		}
	}
	return blocks
}

func fileURI(path, name string) string {
	if path != "" {
		return "file://" + path
	}
	return "file://" + name
}

func (a *App) Cancel(sessionID string) error {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return err
	}
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.Notify("session/cancel", map[string]any{"sessionId": info.ACPID})
}

func (a *App) setMode(client *acpclient.Client, info *SessionInfo, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "session/set_mode", acpclient.SessionSetModeParams{SessionID: info.ACPID, ModeID: mode}); err != nil {
		return err
	}
	info.Mode = mode
	return nil
}

func (a *App) SetMode(sessionID, mode string) (*SessionInfo, error) {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return nil, err
	}
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}
	if err := a.setMode(client, info, mode); err != nil {
		return nil, err
	}
	return info, nil
}

func (a *App) setModel(client *acpclient.Client, info *SessionInfo, provider, model, effort string) error {
	if provider == "" || model == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "session/set_model", acpclient.SessionSetModelParams{
		SessionID:       info.ACPID,
		Provider:        provider,
		Model:           model,
		ReasoningEffort: effort,
	}); err != nil {
		return err
	}
	info.Provider = provider
	info.Model = model
	info.ReasoningEffort = effort
	return nil
}

func (a *App) SetModel(sessionID, provider, model string) (*SessionInfo, error) {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return nil, err
	}
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}
	if err := a.setModel(client, info, provider, model, ""); err != nil {
		return nil, err
	}
	return info, nil
}

func (a *App) SetReasoningEffort(sessionID, provider, model, effort string) (*SessionInfo, error) {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return nil, err
	}
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}
	if provider == "" {
		provider = info.Provider
	}
	if model == "" {
		model = info.Model
	}
	if err := a.setModel(client, info, provider, model, effort); err != nil {
		return nil, err
	}
	return info, nil
}

func (a *App) ListModels(provider string) (acpclient.ModelListResult, error) {
	client, err := a.ensureClient()
	if err != nil {
		return acpclient.ModelListResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	params := map[string]string{}
	if provider != "" {
		params["provider"] = provider
	}
	raw, err := client.Request(ctx, "pegg/models", params)
	if err != nil {
		return acpclient.ModelListResult{}, err
	}
	var res acpclient.ModelListResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	return res, nil
}

func (a *App) RenameSession(sessionID, title string) error {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return err
	}
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "session/rename", acpclient.SessionRenameParams{SessionID: info.PeggID, Title: title}); err != nil {
		return err
	}
	info.Title = title
	return nil
}

func (a *App) DeleteSession(sessionID string) error {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return err
	}
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := client.Request(ctx, "session/delete", acpclient.SessionIDParams{SessionID: info.PeggID}); err != nil {
		return err
	}
	a.ForgetSession(info.ACPID)
	return nil
}

func (a *App) ForkSession(sessionID, messageID string, userMessageIndex int) (acpclient.SessionForkResult, error) {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return acpclient.SessionForkResult{}, err
	}
	client, err := a.clientOrErr()
	if err != nil {
		return acpclient.SessionForkResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	params := acpclient.SessionForkParams{SessionID: info.ACPID, MessageID: messageID}
	if userMessageIndex >= 0 {
		index := userMessageIndex
		params.UserMessageIndex = &index
	}
	raw, err := client.Request(ctx, "session/fork", params)
	if err != nil {
		return acpclient.SessionForkResult{}, err
	}
	var res acpclient.SessionForkResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	return res, nil
}

func (a *App) RevertSession(sessionID, messageID string, userMessageIndex int) (acpclient.SessionRevertResult, error) {
	info, err := a.resolveSession(sessionID)
	if err != nil {
		return acpclient.SessionRevertResult{}, err
	}
	client, err := a.clientOrErr()
	if err != nil {
		return acpclient.SessionRevertResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	params := acpclient.SessionForkParams{SessionID: info.ACPID, MessageID: messageID}
	if userMessageIndex >= 0 {
		index := userMessageIndex
		params.UserMessageIndex = &index
	}
	raw, err := client.Request(ctx, "session/revert", params)
	if err != nil {
		return acpclient.SessionRevertResult{}, err
	}
	var res acpclient.SessionRevertResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	return res, nil
}

func (a *App) RecordStart() error {
	client, err := a.ensureClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = client.Request(ctx, "stt/record/start", map[string]any{})
	return err
}

func (a *App) RecordStop() (acpclient.STTResult, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return acpclient.STTResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	raw, err := client.Request(ctx, "stt/record/stop", map[string]any{})
	if err != nil {
		return acpclient.STTResult{}, err
	}
	var res acpclient.STTResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return res, err
	}
	return res, nil
}
