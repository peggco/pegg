package stt

import (
	"sort"

	"github.com/peggco/pegg/internal/core/config"
)

var providerBaseURLs = map[string]string{
	"assemblyai": "https://api.assemblyai.com",
	"openai":     "https://api.openai.com/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"together":   "https://api.together.xyz/v1",
	"fireworks":  "https://api.fireworks.ai/inference/v1",
}

var providerModels = map[string][]Model{
	"assemblyai": {
		{Provider: "assemblyai", ID: "best", Name: "Best (async)", Streaming: false},
		{Provider: "assemblyai", ID: "nano", Name: "Nano (async)", Streaming: false},
		{Provider: "assemblyai", ID: "universal-3-6-pro", Name: "Universal 3.6 Pro (stream)", Streaming: true},
		{Provider: "assemblyai", ID: "universal-3-5-pro", Name: "Universal 3.5 Pro (stream)", Streaming: true},
		{Provider: "assemblyai", ID: "universal-streaming-english", Name: "Universal Streaming English", Streaming: true},
		{Provider: "assemblyai", ID: "universal-streaming-multilingual", Name: "Universal Streaming Multilingual", Streaming: true},
	},
	"openai": {
		{Provider: "openai", ID: "whisper-1", Name: "Whisper 1"},
		{Provider: "openai", ID: "gpt-4o-transcribe", Name: "GPT-4o Transcribe"},
		{Provider: "openai", ID: "gpt-4o-mini-transcribe", Name: "GPT-4o mini Transcribe"},
	},
	"groq": {
		{Provider: "groq", ID: "whisper-large-v3", Name: "Whisper Large v3"},
		{Provider: "groq", ID: "whisper-large-v3-turbo", Name: "Whisper Large v3 Turbo"},
		{Provider: "groq", ID: "distil-whisper-large-v3-en", Name: "Distil Whisper Large v3 EN"},
	},
	"together": {
		{Provider: "together", ID: "openai/whisper-large-v3", Name: "Whisper Large v3"},
	},
	"fireworks": {
		{Provider: "fireworks", ID: "whisper-v3", Name: "Whisper v3"},
	},
}

func BaseURL(provider string) string { return providerBaseURLs[provider] }

func Providers() []string {
	names := make([]string, 0, len(providerBaseURLs))
	for name := range providerBaseURLs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ModelsFor(provider string) []Model {
	models := providerModels[provider]
	out := make([]Model, len(models))
	copy(out, models)
	return out
}

func AllModels() []Model {
	var out []Model
	for _, name := range Providers() {
		out = append(out, ModelsFor(name)...)
	}
	return out
}

func ModelsForConfigured(configured []string) []Model {
	known := make(map[string]struct{}, len(configured))
	for _, name := range configured {
		known[name] = struct{}{}
	}
	var out []Model
	for _, name := range Providers() {
		if _, ok := known[name]; !ok {
			continue
		}
		out = append(out, ModelsFor(name)...)
	}
	return out
}

func DefaultModel(provider string) string {
	switch provider {
	case "assemblyai":
		return "best"
	default:
		if models := ModelsFor(provider); len(models) > 0 {
			return models[0].ID
		}
		return "whisper-1"
	}
}

func DefaultStreamModel(provider string) string {
	switch provider {
	case "assemblyai":
		return "universal-3-6-pro"
	default:
		return ""
	}
}

func IsStreamModel(provider, id string) bool {
	for _, model := range ModelsFor(provider) {
		if model.ID == id && model.Streaming {
			return true
		}
	}
	return false
}

func IsAsyncModel(provider, id string) bool {
	for _, model := range ModelsFor(provider) {
		if model.ID == id && !model.Streaming {
			return true
		}
	}
	return false
}

func ModelsForConfig(provider string, cfg *config.STTConfig) []Model {
	if cfg != nil {
		if entry, ok := cfg.ProviderConfig(provider); ok && len(entry.Models) > 0 {
			seen := make(map[string]struct{}, len(entry.Models))
			out := make([]Model, 0, len(entry.Models))
			for _, m := range entry.Models {
				if m.ID == "" {
					continue
				}
				if _, ok := seen[m.ID]; ok {
					continue
				}
				seen[m.ID] = struct{}{}
				out = append(out, Model{
					Provider:  provider,
					ID:        m.ID,
					Name:      m.Name,
					Streaming: m.Streaming,
				})
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return ModelsFor(provider)
}

func ProvidersForConfig(cfg *config.STTConfig) []string {
	seen := make(map[string]struct{})
	add := func(name string) {
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, name := range Providers() {
		add(name)
	}
	if cfg != nil {
		for _, p := range cfg.Providers {
			add(p.Provider)
		}
		if cfg.Provider != "" && cfg.APIKey != "" {
			add(cfg.Provider)
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func DefaultModelForConfig(provider string, cfg *config.STTConfig) string {
	if models := ModelsForConfig(provider, cfg); len(models) > 0 {
		return models[0].ID
	}
	return DefaultModel(provider)
}

func IsStreamModelConfig(provider, id string, cfg *config.STTConfig) bool {
	for _, model := range ModelsForConfig(provider, cfg) {
		if model.ID == id {
			return model.Streaming
		}
	}
	return IsStreamModel(provider, id)
}
