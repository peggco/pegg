//go:build gui

package main

import (
	"context"
	"sync"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
)

type RecommendedDefaults struct {
	Available bool     `json:"available"`
	Changes   []string `json:"changes"`
	Skipped   []string `json:"skipped"`
	Error     string   `json:"error,omitempty"`
}

var (
	recommendedDefaultsMu sync.Mutex
	recommendedDefaults   *pegglogin.Defaults
)

func (a *App) FetchRecommendedDefaults() RecommendedDefaults {
	d, err := pegglogin.FetchDefaults(context.Background(), pegglogin.DefaultsEndpoint(""), nil)
	if err != nil {
		return RecommendedDefaults{Error: err.Error()}
	}
	cfg, err := config.Load()
	if err != nil {
		return RecommendedDefaults{Error: err.Error()}
	}
	report := pegglogin.ApplyDefaults(cfg, d, pegglogin.ProviderName)

	recommendedDefaultsMu.Lock()
	recommendedDefaults = d
	recommendedDefaultsMu.Unlock()

	changes := report.Changes
	if changes == nil {
		changes = []string{}
	}
	skipped := report.Skipped
	if skipped == nil {
		skipped = []string{}
	}

	return RecommendedDefaults{
		Available: !report.Empty(),
		Changes:   changes,
		Skipped:   skipped,
	}
}

func (a *App) ApplyRecommendedDefaults() error {
	recommendedDefaultsMu.Lock()
	d := recommendedDefaults
	recommendedDefaultsMu.Unlock()

	if d == nil {
		fetched, err := pegglogin.FetchDefaults(context.Background(), pegglogin.DefaultsEndpoint(""), nil)
		if err != nil {
			return err
		}
		d = fetched
	}

	return config.Update(func(cfg *config.Config) error {
		pegglogin.ApplyDefaults(cfg, d, pegglogin.ProviderName)
		return nil
	})
}
