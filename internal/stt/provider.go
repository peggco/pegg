package stt

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/peggco/pegg/internal/core/config"
)

var (
	ErrNotConfigured     = errors.New("speech-to-text is not configured")
	ErrStreamUnsupported = errors.New("speech-to-text provider does not support streaming")
)

type Factory func(cfg config.LLMConfig, settings *config.STTConfig) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

func RegisterProvider(name string, factory Factory) {
	if name == "" || factory == nil {
		return
	}
	registryMu.Lock()
	registry[name] = factory
	registryMu.Unlock()
}

func NewProvider(name string, cfg config.LLMConfig, settings *config.STTConfig) (Provider, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("stt: unknown provider %q", name)
	}
	return factory(cfg, settings)
}

func HasProvider(name string) bool {
	registryMu.RLock()
	_, ok := registry[name]
	registryMu.RUnlock()
	return ok
}

func ListProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
