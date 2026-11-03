package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func LoadDir(dir, source string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("skill: read dir %s: %w", dir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		loadSkillDir(filepath.Join(dir, e.Name()), source)
	}
	return nil
}

func loadSkillDir(dir, source string) {
	s, err := parseSKILL(filepath.Join(dir, SkillFileName), source)
	if err != nil {
		return
	}
	registerOrReplace(s)
}

func LoadSkillDir(dir, source string) error {
	s, err := parseSKILL(filepath.Join(dir, SkillFileName), source)
	if err != nil {
		return err
	}
	registerOrReplace(s)
	return nil
}

func LoadDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := LoadDir(dir, dir); err != nil {
			return err
		}
	}
	return nil
}
