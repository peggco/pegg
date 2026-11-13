package config

import "strings"

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
