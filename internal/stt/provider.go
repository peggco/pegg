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

type DriverFactory func(cfg config.LLMConfig, settings *config.STTConfig) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)

	driversMu sync.RWMutex
	drivers   = make(map[string]DriverFactory)
)

func RegisterProvider(name string, factory Factory) {
	if name == "" || factory == nil {
		return
	}
	registryMu.Lock()
	registry[name] = factory
	registryMu.Unlock()
}

func RegisterDriver(name string, factory DriverFactory) {
	if name == "" || factory == nil {
		return
	}
	driversMu.Lock()
	drivers[name] = factory
	driversMu.Unlock()
}

func NewProvider(name string, cfg config.LLMConfig, settings *config.STTConfig) (Provider, error) {
	if name == "" {
		name = cfg.Provider
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if ok {
		return factory(cfg, settings)
	}

	if cfg.Driver == "" {
		return nil, fmt.Errorf("stt: no provider registered for %q", name)
	}

	driversMu.RLock()
	driverFactory, ok := drivers[cfg.Driver]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("stt: no driver registered for %q", cfg.Driver)
	}
	return driverFactory(cfg, settings)
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

func HasDriver(name string) bool {
	driversMu.RLock()
	_, ok := drivers[name]
	driversMu.RUnlock()
	return ok
}

func ListDrivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
