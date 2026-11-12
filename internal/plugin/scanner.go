package plugin

import (
	"os"
	"path/filepath"
)

func scanPlugins(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var plugins []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if entry.Name()[0] == '.' {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&0111 == 0 {
			continue
		}

		if ext := filepath.Ext(entry.Name()); ext != "" {
			continue
		}

		plugins = append(plugins, filepath.Join(dir, entry.Name()))
	}

	return plugins, nil
}

func ScanPlugins(dir string) ([]string, error) {
	return scanPlugins(dir)
}
