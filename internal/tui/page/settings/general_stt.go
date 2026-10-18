package settings

import (
	"strings"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
)

func (s *Settings) sttConfig() *config.STTConfig {
	if s.deps.Config == nil {
		s.deps.Config = config.DefaultConfig()
	}
	if s.deps.Config.STT == nil {
		s.deps.Config.STT = &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
	}
	return s.deps.Config.STT
}

func (s *Settings) saveSTTConfig(cfg *config.STTConfig) {
	if err := config.UpsertSTT(cfg); err != nil {
		s.errMsg = i18n.Tf("settings.stt.saveError", err.Error())
		return
	}
	if fresh, err := config.Load(); err == nil {
		s.deps.Config = fresh
	}
	s.errMsg = ""
	if s.onSTTChange != nil {
		s.onSTTChange()
	}
	s.back()
	if s.requestRedraw != nil {
		s.requestRedraw()
	}
}

func (s *Settings) sttProviderValue() string {
	cfg := s.sttConfig()
	if cfg.Provider == "" {
		return i18n.T("settings.voice.notConfigured")
	}
	return cfg.Provider
}

func (s *Settings) sttModelValue() string {
	cfg := s.sttConfig()
	if cfg.Provider == "" || cfg.Model == "" {
		return i18n.T("settings.voice.notConfigured")
	}
	return cfg.Provider + "/" + cfg.Model
}

func (s *Settings) sttModelKnown(provider, model string) bool {
	for _, m := range stt.ModelsFor(provider) {
		if m.ID == model {
			return true
		}
	}
	return false
}

func (s *Settings) openSTTProviders() {
	l := components.NewList(i18n.T("settings.stt.providerTitle"))
	l.SetSearchable(true)
	cfg := s.sttConfig()
	var items []components.ListItem
	for _, name := range stt.Providers() {
		_, configured := cfg.ProviderConfig(name)
		detail, marked := i18n.T("settings.provider.notConfigured"), false
		if configured {
			detail, marked = i18n.T("settings.provider.configured"), true
		}
		items = append(items, components.ListItem{Label: name, Detail: detail, Marked: marked, Data: name})
	}
	l.SetItems(items)
	l.SetOnSelect(func(_ int, item components.ListItem) {
		name, _ := item.Data.(string)
		if name == "" {
			return
		}
		if _, ok := cfg.ProviderConfig(name); ok {
			s.openSTTProviderChoice(name)
			return
		}
		s.openSTTAPIKey(name)
	})
	s.openSub(&listModal{title: i18n.T("settings.stt.providerTitle"), list: l, onBack: s.back})
}

func (s *Settings) openSTTProviderChoice(name string) {
	l := components.NewList(i18n.Tf("settings.stt.providerChoice", name))
	l.SetItems([]components.ListItem{
		{Label: i18n.T("settings.provider.useExisting"), Detail: i18n.T("settings.provider.useExistingDetail")},
		{Label: i18n.T("settings.provider.configureAgain"), Detail: i18n.T("settings.provider.configureAgainDetail")},
	})
	l.SetOnSelect(func(i int, _ components.ListItem) {
		if i == 1 {
			s.openSTTAPIKey(name)
			return
		}
		s.selectSTTProvider(name)
	})
	s.openSub(&listModal{title: i18n.T("settings.stt.alreadyConfigured"), list: l, onBack: s.back})
}

func (s *Settings) selectSTTProvider(name string) {
	cfg := s.sttConfig()
	cfg.Provider = name
	if cfg.Model == "" || !s.sttModelKnown(name, cfg.Model) {
		cfg.Model = stt.DefaultModel(name)
	}
	s.saveSTTConfig(cfg)
}

func (s *Settings) openSTTAPIKey(name string) {
	f := components.NewField(i18n.Tf("settings.stt.apiKeyTitle", name))
	f.SetMask('*')
	if entry, ok := s.sttConfig().ProviderConfig(name); ok && entry.APIKey != "" {
		f.SetPlaceholder(i18n.T("settings.voice.apiKeyKeep"))
	} else {
		f.SetPlaceholder(i18n.T("settings.voice.apiKeyPlaceholder"))
	}
	f.Focus()
	f.SetOnEnter(func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			s.back()
			return
		}
		cfg := s.sttConfig()
		entry, _ := cfg.ProviderConfig(name)
		entry.Provider = name
		entry.APIKey = key
		cfg.UpsertProvider(entry)
		cfg.Provider = name
		if cfg.Model == "" || !s.sttModelKnown(name, cfg.Model) {
			cfg.Model = stt.DefaultModel(name)
		}
		s.saveSTTConfig(cfg)
	})
	f.SetOnCancel(s.back)
	s.openSub(&fieldModal{title: i18n.Tf("settings.stt.apiKeyTitle", name), field: f})
}

func (s *Settings) openSTTModels() {
	l := components.NewList(i18n.T("settings.stt.modelTitle"))
	l.SetSearchable(true)
	cfg := s.sttConfig()
	var items []components.ListItem
	for _, provider := range cfg.Providers {
		for _, model := range stt.ModelsFor(provider.Provider) {
			hint := ""
			if model.Streaming {
				hint = i18n.T("settings.stt.streamBadge")
			}
			items = append(items, components.ListItem{
				Label:  provider.Provider + "/" + model.ID,
				Hint:   hint,
				Marked: provider.Provider == cfg.Provider && model.ID == cfg.Model,
				Data:   []string{provider.Provider, model.ID},
			})
		}
	}
	if len(items) == 0 {
		items = append(items, components.ListItem{Label: i18n.T("settings.stt.noProviders"), Disabled: true})
	}
	l.SetItems(items)
	l.SelectFirstEnabled()
	l.SetOnSelect(func(_ int, item components.ListItem) {
		data, _ := item.Data.([]string)
		if len(data) != 2 {
			return
		}
		cfg := s.sttConfig()
		cfg.Provider = data[0]
		cfg.Model = data[1]
		s.saveSTTConfig(cfg)
	})
	s.openSub(&listModal{title: i18n.T("settings.stt.modelTitle"), list: l, onBack: s.back})
}
