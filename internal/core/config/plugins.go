package config

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func PluginExcluded(name string) (bool, error) {
	cfg, err := Load()
	if err != nil {
		return false, err
	}
	for _, excluded := range cfg.Plugins.Exclude {
		if excluded == name {
			return true, nil
		}
	}
	return false, nil
}

// NormalizePluginName produces the canonical form used to match plugin config
// keys. It lowercases the name, drops a leading "pegg-" prefix, and collapses
// spaces, underscores, dots, and repeated separators into single hyphens, so
// "Kanban GitHub", "kanban_github", and "pegg-kanban-github" all match.
func NormalizePluginName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "pegg-")
	var b strings.Builder
	pendingDash := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(r)
			continue
		}
		pendingDash = true
	}
	return strings.Trim(b.String(), "-")
}

// EnvFor returns the merged environment configured for any of the given plugin
// names. Exact matches are merged first, then normalized matches, with later
// entries overriding earlier ones.
func (p PluginConfig) EnvFor(names ...string) map[string]string {
	if len(p.Env) == 0 {
		return nil
	}
	wanted := make(map[string]bool, len(names)*2)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		wanted[name] = true
		if normalized := NormalizePluginName(name); normalized != "" {
			wanted[normalized] = true
		}
	}
	keys := make([]string, 0, len(p.Env))
	for key := range p.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string)
	for _, key := range keys {
		if !wanted[key] && !wanted[NormalizePluginName(key)] {
			continue
		}
		for envKey, value := range p.Env[key] {
			out[envKey] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PluginEnv returns the configured environment for a plugin.
func PluginEnv(name string) (map[string]string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	return cfg.Plugins.EnvFor(name), nil
}

// SetPluginEnv merges values into the plugin's env map and persists the result.
func SetPluginEnv(name string, values map[string]string) (bool, error) {
	changed := false
	err := Update(func(cfg *Config) error {
		if len(values) == 0 {
			return nil
		}
		key := pluginEnvKey(cfg.Plugins, name)
		if key == "" {
			return fmt.Errorf("plugin name is required")
		}
		if cfg.Plugins.Env == nil {
			cfg.Plugins.Env = make(map[string]map[string]string)
		}
		entry := cfg.Plugins.Env[key]
		if entry == nil {
			entry = make(map[string]string, len(values))
		}
		for envKey, value := range values {
			envKey = strings.TrimSpace(envKey)
			if err := validateEnvKey(envKey); err != nil {
				return err
			}
			if strings.ContainsRune(value, 0) {
				return fmt.Errorf("environment variable %q contains a NUL byte", envKey)
			}
			if existing, ok := entry[envKey]; ok && existing == value {
				continue
			}
			entry[envKey] = value
			changed = true
		}
		cfg.Plugins.Env[key] = entry
		return nil
	})
	return changed, err
}

// UnsetPluginEnv removes keys from the plugin's env map and persists the result.
func UnsetPluginEnv(name string, keys ...string) (bool, error) {
	changed := false
	err := Update(func(cfg *Config) error {
		key := pluginEnvKey(cfg.Plugins, name)
		if key == "" {
			return fmt.Errorf("plugin name is required")
		}
		entry := cfg.Plugins.Env[key]
		if entry == nil {
			return nil
		}
		for _, envKey := range keys {
			envKey = strings.TrimSpace(envKey)
			if _, ok := entry[envKey]; !ok {
				continue
			}
			delete(entry, envKey)
			changed = true
		}
		if len(entry) == 0 {
			delete(cfg.Plugins.Env, key)
			if len(cfg.Plugins.Env) == 0 {
				cfg.Plugins.Env = nil
			}
			return nil
		}
		cfg.Plugins.Env[key] = entry
		return nil
	})
	return changed, err
}

func pluginEnvKey(plugins PluginConfig, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if _, ok := plugins.Env[name]; ok {
		return name
	}
	normalized := NormalizePluginName(name)
	for key := range plugins.Env {
		if NormalizePluginName(key) == normalized {
			return key
		}
	}
	return name
}

func validateEnvKey(key string) error {
	if key == "" {
		return fmt.Errorf("environment variable name is empty")
	}
	if strings.ContainsAny(key, "=\x00") {
		return fmt.Errorf("invalid environment variable name %q", key)
	}
	return nil
}

func ExcludePlugin(names ...string) (bool, error) {
	return updatePluginExclude(names, true)
}

func IncludePlugin(names ...string) (bool, error) {
	return updatePluginExclude(names, false)
}

func updatePluginExclude(names []string, add bool) (bool, error) {
	changed := false
	err := Update(func(cfg *Config) error {
		seen := make(map[string]bool, len(cfg.Plugins.Exclude))
		for _, existing := range cfg.Plugins.Exclude {
			seen[existing] = true
		}
		if add {
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name == "" || seen[name] {
					continue
				}
				cfg.Plugins.Exclude = append(cfg.Plugins.Exclude, name)
				seen[name] = true
				changed = true
			}
			return nil
		}
		remove := make(map[string]bool, len(names))
		for _, name := range names {
			if name = strings.TrimSpace(name); name != "" {
				remove[name] = true
			}
		}
		kept := make([]string, 0, len(cfg.Plugins.Exclude))
		for _, existing := range cfg.Plugins.Exclude {
			if remove[existing] {
				changed = true
				continue
			}
			kept = append(kept, existing)
		}
		cfg.Plugins.Exclude = kept
		return nil
	})
	return changed, err
}
