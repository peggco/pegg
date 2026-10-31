//go:build gui

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goruntime "runtime"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
	"github.com/peggco/pegg/pkg/desktop/internal/binary"
	"github.com/peggco/pegg/pkg/desktop/internal/prefs"
	"github.com/peggco/pegg/pkg/desktop/internal/themes"
	"github.com/peggco/pegg/pkg/desktop/internal/workspace"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = config.AppVersion

type SessionInfo struct {
	ACPID           string                   `json:"acpId"`
	PeggID          string                   `json:"peggId"`
	Cwd             string                   `json:"cwd"`
	Title           string                   `json:"title"`
	Mode            string                   `json:"mode"`
	Provider        string                   `json:"provider"`
	Model           string                   `json:"model"`
	PermissionMode  string                   `json:"permissionMode"`
	ReasoningEffort string                   `json:"reasoningEffort,omitempty"`
	Commands        []acpclient.SlashCommand `json:"commands"`
}

type BootstrapInfo struct {
	Platform         string         `json:"platform"`
	Arch             string         `json:"arch"`
	Version          string         `json:"version"`
	Home             string         `json:"home"`
	Prefs            prefs.Prefs    `json:"prefs"`
	Themes           []themes.Theme `json:"themes"`
	Binary           binary.Status  `json:"binary"`
	DefaultWorkspace string         `json:"defaultWorkspace"`
	Workspace        string         `json:"workspace"`
	LogPath          string         `json:"logPath"`
	Language         string         `json:"language"`
	NativeTitlebar   bool           `json:"nativeTitlebar"`
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

	prefs  *prefs.Store
	theme  []themes.Theme
	binary binary.Status

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

	teamMu    sync.Mutex
	teamCmd   *exec.Cmd
	teamDone  chan struct{}
	teamState TeamStatus
}

func NewApp() *App {
	store, err := prefs.Load()
	if err != nil {
		store, _ = prefs.Load()
	}
	a := &App{
		prefs:    store,
		theme:    themes.All(),
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
	a.flushOnce.Do(func() { go a.flushLoop(ctx) })
	if a.prefs != nil {
		p := a.prefs.Get()
		a.binary = binary.Resolve(ctx, p.BinaryPath, p.Workspace)
	}
	a.logf("pegg desktop %s %s/%s", appVersion, goruntime.GOOS, goruntime.GOARCH)
	go a.autoStartConnect()
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.saveWindow(ctx)
	return false
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	a.closing = true
	c := a.client
	a.mu.Unlock()
	if c != nil {
		c.Close()
	}
	_, _ = a.ConnectStop()
	_, _ = a.TeamCancel()
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
	wailsruntime.EventsEmit(a.ctx, name, data)
}

func (a *App) Bootstrap() (BootstrapInfo, error) {
	p := prefs.Defaults()
	if a.prefs != nil {
		p = a.prefs.Get()
	}
	home, _ := os.UserHomeDir()
	ws := p.Workspace
	if ws == "" {
		ws = home
	}
	if _, err := workspace.Valid(ws); err != nil {
		ws = home
	}
	logPath, _ := prefs.Path()
	logPath = filepath.Join(filepath.Dir(logPath), "desktop.log")
	if !a.binary.Found {
		a.binary = binary.Resolve(context.Background(), p.BinaryPath, ws)
	}
	return BootstrapInfo{
		Platform:         goruntime.GOOS,
		Arch:             goruntime.GOARCH,
		Version:          appVersion,
		Home:             home,
		Prefs:            p,
		Themes:           a.theme,
		Binary:           a.binary,
		DefaultWorkspace: ws,
		Workspace:        ws,
		LogPath:          logPath,
		Language:         currentLanguage(),
		NativeTitlebar:   nativeTitlebar(),
	}, nil
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
	resetFileIndex()
	if err := a.prefs.TouchWorkspace(abs, workspace.Name(abs)); err != nil {
		return "", err
	}
	return abs, nil
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
	})
	return a.prefs.Get(), err
}

func (a *App) SetTheme(name, mode string) prefs.Prefs {
	if name == "" && mode == "" {
		return a.prefs.Get()
	}
	_ = a.prefs.SetTheme(name, mode)
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
	if err != nil || cfg == nil || cfg.Language == "" {
		return "en"
	}
	return cfg.Language
}
