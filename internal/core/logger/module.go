package logger

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/peggco/pegg/internal/core/config"
)

type DriverFactory func(cfg config.LoggerConfig) (Handler, error)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]DriverFactory)
)

func RegisterDriver(name string, factory DriverFactory) {
	if name == "" || factory == nil {
		return
	}
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = factory
}

func NewFromConfig(cfg config.LoggerConfig) (*Logger, error) {
	driversMu.RLock()
	factory, ok := drivers[cfg.Driver]
	driversMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("logger: no driver registered for %q", cfg.Driver)
	}

	h, err := factory(cfg)
	if err != nil {
		return nil, err
	}

	return New(resolveLevel(cfg), h), nil
}

func resolveLevel(cfg config.LoggerConfig) Level {
	if debugEnabled(os.Getenv("PEGG_DEBUG")) {
		return LevelDebug
	}
	if level := strings.TrimSpace(os.Getenv("PEGG_LOG_LEVEL")); level != "" {
		return ParseLevel(level)
	}
	if cfg.Level != "" {
		return ParseLevel(cfg.Level)
	}
	return ParseLevel(config.DefaultLoggerLevel)
}

func debugEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "debug":
		return true
	default:
		return false
	}
}

func LoggerModule(cfg config.LoggerConfig) (*Logger, error) {
	if cfg.Driver == "" {
		return nil, errors.New("logger: driver name is empty")
	}
	return NewFromConfig(cfg)
}
