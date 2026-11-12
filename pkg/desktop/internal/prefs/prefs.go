package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ThemeModeAuto  = "auto"
	ThemeModeLight = "light"
	ThemeModeDark  = "dark"

	DefaultSidebarWidth = 272
)

type Window struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	X      int `json:"x,omitempty"`
	Y      int `json:"y,omitempty"`
}

type Recent struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	OpenedAt time.Time `json:"openedAt"`
}

type Prefs struct {
	Theme            string   `json:"theme"`
	ThemeMode        string   `json:"themeMode"`
	Workspace        string   `json:"workspace"`
	Recents          []Recent `json:"recents"`
	BinaryPath       string   `json:"binaryPath"`
	AutoDownload     bool     `json:"autoDownload"`
	SendOnEnter      bool     `json:"sendOnEnter"`
	FontScale        float64  `json:"fontScale"`
	SidebarCollapsed bool     `json:"sidebarCollapsed"`
	SidebarWidth     int      `json:"sidebarWidth"`
	SidebarMode      string   `json:"sidebarMode"`
	LayoutMode       string   `json:"layoutMode"`
	Direction        string   `json:"direction"`
	ChatHistoryMode  string   `json:"chatHistoryMode"`
	LastSession      string   `json:"lastSession"`
	Window           Window   `json:"window"`
}

func Defaults() Prefs {
	return Prefs{
		Theme:           "dark",
		ThemeMode:       ThemeModeAuto,
		AutoDownload:    true,
		SendOnEnter:     true,
		FontScale:       1,
		SidebarWidth:    DefaultSidebarWidth,
		SidebarMode:     "sidebar",
		LayoutMode:      "default",
		Direction:       "ltr",
		ChatHistoryMode: "compact",
	}
}

type Store struct {
	mu   sync.RWMutex
	path string
	data Prefs
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pegg", "desktop.json"), nil
}

func Load() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, data: Defaults()}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) > 0 {
		var data Prefs
		if err := json.Unmarshal(raw, &data); err != nil {
			return s, nil
		}
		s.data = merge(data)
	}
	return s, nil
}

func merge(data Prefs) Prefs {
	def := Defaults()
	data.Theme = normalizeTheme(data.Theme)
	if data.ThemeMode == "" {
		data.ThemeMode = def.ThemeMode
	}
	if data.FontScale <= 0 {
		data.FontScale = def.FontScale
	}
	if data.Window.Width <= 0 {
		data.Window.Width = 1280
	}
	if data.Window.Height <= 0 {
		data.Window.Height = 840
	}
	if data.SidebarWidth < 160 || data.SidebarWidth > 600 {
		data.SidebarWidth = def.SidebarWidth
	}
	data.SidebarMode = normalizeSidebarMode(data.SidebarMode)
	data.LayoutMode = normalizeLayoutMode(data.LayoutMode)
	data.Direction = normalizeDirection(data.Direction)
	data.ChatHistoryMode = normalizeChatHistoryMode(data.ChatHistoryMode)
	return data
}

func normalizeTheme(name string) string {
	switch name {
	case "light", "dark":
		return name
	case "pegg-light", "catppuccin-latte", "tokyonight-day", "gruvbox-light",
		"solarized-light", "rosepine-dawn", "everforest-light":
		return "light"
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "light") || strings.Contains(lower, "latte") ||
		strings.Contains(lower, "dawn") || strings.Contains(lower, "day") {
		return "light"
	}
	return "dark"
}

func normalizeSidebarMode(mode string) string {
	if mode == "inset" || mode == "floating" {
		return mode
	}
	return "sidebar"
}

func normalizeLayoutMode(mode string) string {
	if mode == "compact" || mode == "full" {
		return mode
	}
	return "default"
}

func normalizeDirection(direction string) string {
	if direction == "rtl" {
		return "rtl"
	}
	return "ltr"
}

func normalizeChatHistoryMode(mode string) string {
	if mode == "detailed" {
		return "detailed"
	}
	return "compact"
}

func (s *Store) Get() Prefs {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.data
	out.Recents = append([]Recent(nil), s.data.Recents...)
	return out
}

func (s *Store) Save() error {
	s.mu.RLock()
	data := s.data
	s.mu.RUnlock()
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Update(fn func(*Prefs)) error {
	s.mu.Lock()
	fn(&s.data)
	s.data = merge(s.data)
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) SetTheme(theme, mode string) error {
	return s.Update(func(p *Prefs) {
		if theme != "" {
			p.Theme = normalizeTheme(theme)
		}
		switch mode {
		case ThemeModeAuto, ThemeModeLight, ThemeModeDark:
			p.ThemeMode = mode
		}
	})
}

func (s *Store) TouchWorkspace(path, name string) error {
	return s.Update(func(p *Prefs) {
		p.Workspace = path
		filtered := p.Recents[:0]
		for _, r := range p.Recents {
			if r.Path != path {
				filtered = append(filtered, r)
			}
		}
		p.Recents = append([]Recent{{Path: path, Name: name, OpenedAt: time.Now()}}, filtered...)
		if len(p.Recents) > 12 {
			p.Recents = p.Recents[:12]
		}
	})
}

func (s *Store) SetWindow(w Window) error {
	return s.Update(func(p *Prefs) { p.Window = w })
}

func (s *Store) SetBinary(path string) error {
	return s.Update(func(p *Prefs) { p.BinaryPath = path })
}

func (s *Store) SetLastSession(id string) error {
	return s.Update(func(p *Prefs) { p.LastSession = id })
}
