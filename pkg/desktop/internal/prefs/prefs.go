package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	ThemeModeAuto  = "auto"
	ThemeModeLight = "light"
	ThemeModeDark  = "dark"
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
	LastSession      string   `json:"lastSession"`
	Window           Window   `json:"window"`
}

func Defaults() Prefs {
	return Prefs{
		Theme:        "dark",
		ThemeMode:    ThemeModeAuto,
		AutoDownload: true,
		SendOnEnter:  true,
		FontScale:    1,
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
	switch data.Theme {
	case "pegg-light":
		data.Theme = "light"
	case "pegg-dark":
		data.Theme = "dark"
	case "":
		data.Theme = def.Theme
	}
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
	return data
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
			p.Theme = theme
		}
		if mode != "" {
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
