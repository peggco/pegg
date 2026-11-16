package config

import (
	"encoding/json"
	"strings"
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

func PluginOptions(name string) (map[string]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	raw, ok := cfg.Plugins.Options[name]
	if !ok || len(raw) == 0 {
		return map[string]string{}, nil
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		return map[string]string{}, nil
	}
	if values == nil {
		values = map[string]string{}
	}
	return values, nil
}

func SetPluginOptions(name string, values map[string]string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	clean := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		clean[key] = value
	}
	return Update(func(cfg *Config) error {
		if len(clean) == 0 {
			delete(cfg.Plugins.Options, name)
			return nil
		}
		data, err := json.Marshal(clean)
		if err != nil {
			return err
		}
		if cfg.Plugins.Options == nil {
			cfg.Plugins.Options = make(map[string]json.RawMessage)
		}
		cfg.Plugins.Options[name] = json.RawMessage(data)
		return nil
	})
}
