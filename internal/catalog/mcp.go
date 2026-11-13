package catalog

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/peggco/pegg/internal/core/config"
)

type MCPEntry = Entry

var (
	mcpIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	placeholderPat = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)
)

func ValidMCPID(id string) bool {
	return mcpIDPattern.MatchString(strings.TrimSpace(id))
}

func ParseMCPID(raw string) (string, error) {
	id := strings.Trim(strings.TrimSpace(raw), "/")
	if !ValidMCPID(id) {
		return "", fmt.Errorf("%w: %q must match %s", ErrBadSpec, raw, mcpIDPattern.String())
	}
	return id, nil
}

func (e Entry) Config(values map[string]string) (config.MCPServerConfig, []string, error) {
	transport := e.Transport
	kind := strings.ToLower(strings.TrimSpace(transport.Type))

	remote := kind == "sse" || kind == "http" || kind == "streamable-http" || kind == "streamable"
	local := kind == "stdio" || kind == "local"
	if kind == "" {
		switch {
		case strings.TrimSpace(transport.URL) != "":
			remote = true
		case strings.TrimSpace(transport.Command) != "":
			local = true
		}
	}

	if !remote && !local {
		return config.MCPServerConfig{}, nil, fmt.Errorf("%w: unknown transport %q", ErrBadSpec, transport.Type)
	}
	if local && strings.TrimSpace(transport.Command) == "" {
		return config.MCPServerConfig{}, nil, fmt.Errorf("%w: stdio transport requires command", ErrBadSpec)
	}
	if remote && strings.TrimSpace(transport.URL) == "" {
		return config.MCPServerConfig{}, nil, fmt.Errorf("%w: %s transport requires url", ErrBadSpec, kind)
	}

	resolver := newResolver(values)
	var cfg config.MCPServerConfig
	if local {
		cfg.Type = "stdio"
		cfg.Command = resolver.resolve(transport.Command)
		cfg.Args = resolver.resolveSlice(transport.Args)
		cfg.Env = resolver.resolveMap(transport.Env)
	} else {
		switch kind {
		case "sse":
			cfg.Type = "sse"
		default:
			cfg.Type = "streamable-http"
		}
		cfg.URL = resolver.resolve(transport.URL)
		cfg.Headers = resolver.resolveMap(transport.Headers)
	}
	for _, key := range e.RequiredEnv {
		resolver.require(key)
	}

	return cfg, resolver.missingKeys(), nil
}

type resolver struct {
	values  map[string]string
	missing map[string]bool
}

func newResolver(values map[string]string) *resolver {
	return &resolver{values: values, missing: map[string]bool{}}
}

func (r *resolver) resolve(value string) string {
	return placeholderPat.ReplaceAllStringFunc(value, func(match string) string {
		key := match[2 : len(match)-1]
		if resolved, ok := r.lookup(key); ok {
			return resolved
		}
		r.missing[key] = true
		return match
	})
}

func (r *resolver) resolveSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = r.resolve(value)
	}
	return out
}

func (r *resolver) resolveMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = r.resolve(value)
	}
	return out
}

func (r *resolver) lookup(key string) (string, bool) {
	if value, ok := r.values[key]; ok && value != "" {
		return value, true
	}
	if value := os.Getenv(key); value != "" {
		return value, true
	}
	return "", false
}

func (r *resolver) require(key string) {
	if _, ok := r.lookup(key); !ok {
		r.missing[key] = true
	}
}

func (r *resolver) missingKeys() []string {
	if len(r.missing) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.missing))
	for key := range r.missing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
