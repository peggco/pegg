//go:build gui

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goruntime "runtime"

	"github.com/peggco/pegg/internal/core/config"
	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/plugin"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
	"github.com/peggco/pegg/pkg/desktop/internal/binary"
	"github.com/peggco/pegg/pkg/desktop/internal/prefs"
	"github.com/peggco/pegg/pkg/desktop/internal/selfupdate"
	"github.com/peggco/pegg/pkg/desktop/internal/themes"
	"github.com/peggco/pegg/pkg/desktop/internal/workspace"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = config.AppVersion

type SessionInfo struct {
	ACPID            string                   `json:"acpId"`
	PeggID           string                   `json:"peggId"`
	Cwd              string                   `json:"cwd"`
	Title            string                   `json:"title"`
	Mode             string                   `json:"mode"`
	Provider         string                   `json:"provider"`
	Model            string                   `json:"model"`
	PermissionMode   string                   `json:"permissionMode"`
	ReasoningEffort  string                   `json:"reasoningEffort,omitempty"`
	Commands         []acpclient.SlashCommand `json:"commands"`
	HistoryOldestSeq int                      `json:"historyOldestSeq,omitempty"`
	HistoryHasMore   bool                     `json:"historyHasMore,omitempty"`
}

type BootstrapInfo struct {
	Platform         string            `json:"platform"`
	Arch             string            `json:"arch"`
	Version          string            `json:"version"`
	Home             string            `json:"home"`
	Prefs            prefs.Prefs       `json:"prefs"`
	Themes           []themes.Theme    `json:"themes"`
	Binary           binary.Status     `json:"binary"`
	SelfUpdate       selfupdate.Status `json:"selfUpdate"`
	DefaultWorkspace string            `json:"defaultWorkspace"`
	Workspace        string            `json:"workspace"`
	LogPath          string            `json:"logPath"`
	Language         string            `json:"language"`
	NativeTitlebar   bool              `json:"nativeTitlebar"`
	ProviderSetup    ProviderSetupInfo `json:"providerSetup"`
}

type RemoveRecentResult struct {
	Recents   []prefs.Recent `json:"recents"`
	Workspace string         `json:"workspace"`
}

type pendingRequest struct {
	ID     string
	Method string
}

type updateItem struct {
	SessionID string
	Update    json.RawMessage
}

type App struct {
	ctx context.Context

	prefs      *prefs.Store
	theme      []themes.Theme
	binary     binary.Status
	selfUpdate *selfupdate.Manager

	mu       sync.Mutex
	client   *acpclient.Client
	clientAt string
	initRes  *acpclient.InitializeResult
	closing  bool

	sessionsMu sync.Mutex
	sessions   map[string]*SessionInfo
	current    string
	pending    map[string]pendingRequest

	updatesMu sync.Mutex
	updates   []updateItem
	flushOnce sync.Once

	logMu sync.Mutex
	log   *os.File

	connectMu       sync.Mutex
	connectCmd      *exec.Cmd
	connectDone     chan struct{}
	connectState    ConnectStatus
	connectStopping bool
	connectDir      string

	teamMu    sync.Mutex
	teamCmd   *exec.Cmd
	teamDone  chan struct{}
	teamState TeamStatus

	peggMu    sync.Mutex
	peggCmd   *exec.Cmd
	peggDone  chan struct{}
	peggState ForeverStatus

	pluginMu  sync.Mutex
	pluginMgr *plugin.Manager

	viewHooks  *desktopViewHooks
	viewCancel context.CancelFunc
}

func NewApp() *App {
	store, err := prefs.Load()
	if err != nil {
		store, _ = prefs.Load()
	}
	a := &App{
		prefs:    store,
		theme:    themes.Desktop(),
		sessions: map[string]*SessionInfo{},
		pending:  map[string]pendingRequest{},
	}
	if prefsPath, err := prefs.Path(); err == nil {
		a.openLog(filepath.Join(filepath.Dir(prefsPath), "desktop.log"))
	}
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	viewCtx, viewCancel := context.WithCancel(ctx)
	a.viewCancel = viewCancel
	a.viewHooks = newDesktopViewHooks(a)
	a.refreshDesktopView()
	go func() {
		if _, err := a.pluginManager(); err != nil {
			a.logf("plugin: %v", err)
		}
		a.refreshDesktopView()
	}()
	go a.desktopViewLoop(viewCtx)
	a.flushOnce.Do(func() { go a.flushLoop(ctx) })
	if a.prefs != nil {
		p := a.prefs.Get()
		a.binary = binary.Resolve(ctx, p.BinaryPath, p.Workspace)
	}
	a.logf("pegg desktop %s %s/%s", appVersion, goruntime.GOOS, goruntime.GOARCH)
	a.initSelfUpdate()
	go a.autoCheckDesktopUpdate()
	go a.autoStartConnect()
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.saveWindow(ctx)
	return false
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	a.closing = true
	if a.viewCancel != nil {
		a.viewCancel()
	}
	c := a.client
	a.mu.Unlock()
	if c != nil {
		c.Close()
	}
	_, _ = a.ConnectStop()
	_, _ = a.TeamCancel()
	_, _ = a.PeggCancel()
	a.resetPluginManager()
	a.logMu.Lock()
	if a.log != nil {
		_ = a.log.Close()
	}
	a.logMu.Unlock()
}

func (a *App) openLog(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	a.log = f
}

func (a *App) logf(format string, args ...any) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	if a.log == nil {
		return
	}
	_, _ = fmt.Fprintf(a.log, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	if corehook.HasFilter(HookEmit) {
		if raw, err := json.Marshal(data); err == nil {
			if hooked, errs := corehook.ApplyFilter(context.Background(), HookEmit, EmitHook{Name: name, Data: raw}); len(errs) == 0 {
				name = hooked.Name
				data = json.RawMessage(hooked.Data)
			}
		}
	}
	wailsruntime.EventsEmit(a.ctx, name, data)
}

func (a *App) Bootstrap() (BootstrapInfo, error) {
	p := prefs.Defaults()
	if a.prefs != nil {
		p = a.prefs.Get()
	}
	home, _ := os.UserHomeDir()
	ws := ""
	if p.Workspace != "" {
		if abs, err := workspace.Valid(p.Workspace); err == nil {
			ws = abs
		}
	}
	defaultWS := ws
	if defaultWS == "" {
		defaultWS = home
	}
	logPath, _ := prefs.Path()
	logPath = filepath.Join(filepath.Dir(logPath), "desktop.log")
	if !a.binary.Found {
		a.binary = binary.Resolve(context.Background(), p.BinaryPath, defaultWS)
	}
	providerSetup, _ := a.ProviderSetup()
	info := BootstrapInfo{
		Platform:         goruntime.GOOS,
		Arch:             goruntime.GOARCH,
		Version:          appVersion,
		Home:             home,
		Prefs:            p,
		Themes:           a.theme,
		Binary:           a.binary,
		SelfUpdate:       a.selfUpdateStatus(),
		DefaultWorkspace: defaultWS,
		Workspace:        ws,
		LogPath:          logPath,
		Language:         currentLanguage(),
		NativeTitlebar:   nativeTitlebar(),
		ProviderSetup:    providerSetup,
	}
	if hooked, errs := corehook.ApplyFilter(context.Background(), HookBootstrap, BootstrapHook{
		Platform:  info.Platform,
		Version:   info.Version,
		Workspace: info.Workspace,
		Language:  info.Language,
	}); len(errs) == 0 {
		if hooked.Workspace != "" {
			info.Workspace = hooked.Workspace
		}
		if hooked.Language != "" {
			info.Language = hooked.Language
		}
	}
	return info, nil
}

func (a *App) Log(message string) {
	a.logf("ui: %s", message)
}

func (a *App) Platform() string { return goruntime.GOOS }
func (a *App) Version() string  { return appVersion }

func (a *App) LogPath() string {
	p, _ := prefs.Path()
	return filepath.Join(filepath.Dir(p), "desktop.log")
}

func (a *App) BinaryStatus() binary.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.binary
}

func (a *App) FindBinary() binary.Status {
	p := a.prefs.Get()
	st := binary.Resolve(context.Background(), p.BinaryPath, p.Workspace)
	a.mu.Lock()
	a.binary = st
	a.mu.Unlock()
	if st.Found && p.BinaryPath == "" {
		_ = a.prefs.SetBinary("")
	}
	return st
}

func (a *App) SetBinaryPath(path string) (binary.Status, error) {
	if strings.TrimSpace(path) == "" {
		_ = a.prefs.SetBinary("")
		return a.FindBinary(), nil
	}
	st := binary.Probe(context.Background(), path)
	if !st.Found {
		return st, fmt.Errorf("not a working pegg binary: %s", st.Error)
	}
	st.Source = "custom"
	_ = a.prefs.SetBinary(path)
	a.mu.Lock()
	a.binary = st
	a.mu.Unlock()
	return st, nil
}

func (a *App) PickBinary() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Select the pegg binary",
		Filters: []wailsruntime.FileFilter{{DisplayName: "pegg", Pattern: "pegg*"}},
	})
}

func (a *App) InstallBinary() (binary.Status, error) {
	dl := binary.NewDownloader()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	st, err := dl.Install(ctx, "latest", func(p binary.Progress) {
		a.emit("binary:progress", p)
	})
	if err != nil {
		a.emit("binary:progress", binary.Progress{Stage: "Error", Percent: 100, Detail: err.Error()})
		return st, err
	}
	_ = a.prefs.SetBinary("")
	a.mu.Lock()
	a.binary = st
	a.mu.Unlock()
	return st, nil
}

func (a *App) PickWorkspace() (string, error) {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                "Open a project",
		DefaultDirectory:     a.currentWorkspace(),
		CanCreateDirectories: true,
	})
	if err != nil || path == "" {
		return "", err
	}
	return a.OpenWorkspace(path)
}

func (a *App) OpenWorkspace(path string) (string, error) {
	abs, err := workspace.Valid(path)
	if err != nil {
		return "", err
	}
	previous := a.prefs.Get().Workspace
	resetFileIndex()
	if err := a.prefs.TouchWorkspace(abs, workspace.Name(abs)); err != nil {
		return "", err
	}
	if previous != abs {
		a.resetWorkspaceState()
	}
	a.syncConnectWorkspace(abs)
	return abs, nil
}

// resetWorkspaceState drops everything scoped to the project that was open
// before. The ACP agent is started with the workspace as its working
// directory, so a switch has to stop it; otherwise the next session, command
// list, and skill list still come from the previous project.
func (a *App) resetWorkspaceState() {
	a.mu.Lock()
	client := a.client
	a.client = nil
	a.clientAt = ""
	a.initRes = nil
	a.mu.Unlock()
	if client != nil {
		client.Close()
	}

	a.sessionsMu.Lock()
	a.sessions = map[string]*SessionInfo{}
	a.current = ""
	a.pending = map[string]pendingRequest{}
	a.sessionsMu.Unlock()

	a.updatesMu.Lock()
	a.updates = nil
	a.updatesMu.Unlock()

	a.emit("acp:pending-clear", nil)
	a.resetPluginManager()
}

// RemoveRecent forgets a local project. When the removed project is the open
// one, the most recent session's project is selected instead.
func (a *App) RemoveRecent(path string) (RemoveRecentResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		p := a.prefs.Get()
		return RemoveRecentResult{Recents: p.Recents, Workspace: p.Workspace}, errors.New("no project path given")
	}
	active := a.prefs.Get().Workspace == path
	next := ""
	if active {
		next = a.fallbackWorkspace(path)
	}
	if err := a.prefs.RemoveRecent(path); err != nil {
		p := a.prefs.Get()
		return RemoveRecentResult{Recents: p.Recents, Workspace: p.Workspace}, err
	}
	if active {
		if next != "" {
			if err := a.prefs.TouchWorkspace(next, workspace.Name(next)); err != nil {
				a.logf("remove project: select %s: %v", next, err)
			}
		}
		resetFileIndex()
		a.resetWorkspaceState()
		a.syncConnectWorkspace(next)
	}
	a.refreshDesktopView()
	p := a.prefs.Get()
	return RemoveRecentResult{Recents: p.Recents, Workspace: p.Workspace}, nil
}

func (a *App) fallbackWorkspace(exclude string) string {
	if dir := a.mostRecentSessionProject(exclude); dir != "" {
		return dir
	}
	for _, recent := range a.prefs.Get().Recents {
		if recent.Path == exclude {
			continue
		}
		if abs, err := workspace.Valid(recent.Path); err == nil {
			return abs
		}
	}
	return ""
}

func (a *App) currentWorkspace() string {
	p := a.prefs.Get()
	if p.Workspace != "" {
		return p.Workspace
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *App) SavePrefs(p prefs.Prefs) (prefs.Prefs, error) {
	err := a.prefs.Update(func(cur *prefs.Prefs) {
		if p.Theme != "" {
			cur.Theme = p.Theme
		}
		if p.ThemeMode != "" {
			cur.ThemeMode = p.ThemeMode
		}
		cur.SendOnEnter = p.SendOnEnter
		cur.AutoDownload = p.AutoDownload
		if p.FontScale > 0 {
			cur.FontScale = p.FontScale
		}
		cur.SidebarCollapsed = p.SidebarCollapsed
		if p.SidebarWidth > 0 {
			cur.SidebarWidth = p.SidebarWidth
		}
		if p.SidebarMode != "" {
			cur.SidebarMode = p.SidebarMode
		}
		if p.LayoutMode != "" {
			cur.LayoutMode = p.LayoutMode
		}
		if p.Direction != "" {
			cur.Direction = p.Direction
		}
		if p.ChatHistoryMode != "" {
			cur.ChatHistoryMode = p.ChatHistoryMode
		}
	})
	a.refreshDesktopView()
	return a.prefs.Get(), err
}

func (a *App) SetTheme(name, mode string) prefs.Prefs {
	if name == "" && mode == "" {
		return a.prefs.Get()
	}
	if hooked, errs := corehook.ApplyFilter(context.Background(), HookTheme, ThemeHook{Name: name, Mode: mode}); len(errs) == 0 {
		name, mode = hooked.Name, hooked.Mode
	}
	_ = a.prefs.SetTheme(name, mode)
	a.refreshDesktopView()
	return a.prefs.Get()
}

func (a *App) ApplyWindowTheme(dark bool) {
	if a.ctx == nil {
		return
	}
	if dark {
		wailsruntime.WindowSetDarkTheme(a.ctx)
	} else {
		wailsruntime.WindowSetLightTheme(a.ctx)
	}
}

func (a *App) Themes() []themes.Theme { return a.theme }

func (a *App) Recents() []prefs.Recent { return a.prefs.Get().Recents }

func (a *App) WindowMinimise()       { wailsruntime.WindowMinimise(a.ctx) }
func (a *App) WindowToggleMaximise() { wailsruntime.WindowToggleMaximise(a.ctx) }
func (a *App) WindowClose()          { wailsruntime.Quit(a.ctx) }
func (a *App) WindowMaximised() bool {
	return wailsruntime.WindowIsMaximised(a.ctx)
}

func (a *App) OpenExternal(url string) {
	lower := strings.ToLower(strings.TrimSpace(url))
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		wailsruntime.BrowserOpenURL(a.ctx, url)
	}
}

func (a *App) RevealPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return reveal(abs)
}

func (a *App) OpenPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", abs)
	}
	return openWith(abs)
}

func (a *App) SaveCopy(path, name string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	src, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", src)
	}
	suggested := filepath.Base(src)
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		suggested = filepath.Base(trimmed)
	}
	target, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save a copy",
		DefaultFilename: suggested,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(target) == "" {
		return "", nil
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return target, nil
}

func (a *App) ReadAttachment(path string) (workspace.Attachment, error) {
	return workspace.ReadAttachment(path)
}

func (a *App) AttachFiles() ([]workspace.Attachment, error) {
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Attach files",
	})
	if err != nil {
		return nil, err
	}
	out := make([]workspace.Attachment, 0, len(paths))
	for _, path := range paths {
		att, err := workspace.ReadAttachment(path)
		if err != nil {
			return out, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		out = append(out, att)
	}
	return out, nil
}

func (a *App) saveWindow(ctx context.Context) {
	if a.ctx == nil {
		return
	}
	w, h := wailsruntime.WindowGetSize(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)
	if w > 0 && h > 0 {
		_ = a.prefs.SetWindow(prefs.Window{Width: w, Height: h, X: x, Y: y})
	}
}

func currentLanguage() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil || !configLanguageSet() {
		return ""
	}
	return cfg.Language
}

func configLanguageSet() bool {
	path, err := config.GetConfigPath(config.GlobalConfigFileName)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw["language"]
	return ok
}
