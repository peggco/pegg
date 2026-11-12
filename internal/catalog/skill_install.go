package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peggco/pegg/internal/skill"
)

const sidecarName = ".pegg-skill.json"

type InstalledSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path"`
	SkillID     string `json:"skillId"`
	Scope       string `json:"scope,omitempty"`
}

type sidecar struct {
	Source  string `json:"source"`
	SkillID string `json:"skillId"`
}

func InstallSkill(ctx context.Context, c *Client, spec SkillSpec, dir string) (InstalledSkill, error) {
	if strings.TrimSpace(dir) == "" {
		return InstalledSkill{}, fmt.Errorf("%w: target directory is empty", ErrBadSpec)
	}
	if !validSkillID(spec.SkillID) {
		return InstalledSkill{}, fmt.Errorf("%w: %q", ErrBadSpec, spec.SkillID)
	}
	data, _, err := c.ResolveSkill(ctx, spec)
	if err != nil {
		return InstalledSkill{}, err
	}
	if !hasFrontmatter(data) {
		return InstalledSkill{}, fmt.Errorf("%w: %s has no frontmatter", ErrBadSpec, spec.String())
	}

	target := filepath.Join(dir, spec.SkillID)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return InstalledSkill{}, fmt.Errorf("catalog: create %s: %w", target, err)
	}
	if err := writeAtomic(filepath.Join(target, skill.SkillFileName), data); err != nil {
		return InstalledSkill{}, err
	}
	meta, _ := json.Marshal(sidecar{Source: spec.Source, SkillID: spec.SkillID})
	if err := writeAtomic(filepath.Join(target, sidecarName), meta); err != nil {
		return InstalledSkill{}, err
	}

	out := InstalledSkill{
		Name:    spec.SkillID,
		Source:  spec.Source,
		Path:    target,
		SkillID: spec.SkillID,
	}
	if parsed, err := skill.ParseFile(filepath.Join(target, skill.SkillFileName), spec.Source); err == nil {
		out.Name = parsed.Name
		out.Description = parsed.Description
	}
	return out, nil
}

func ListInstalledSkills(dir, scope string) []InstalledSkill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []InstalledSkill{}
	}
	out := make([]InstalledSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		parsed, err := skill.ParseFile(filepath.Join(path, skill.SkillFileName), dir)
		if err != nil {
			continue
		}
		item := InstalledSkill{
			Name:        parsed.Name,
			Description: parsed.Description,
			Path:        path,
			SkillID:     entry.Name(),
			Scope:       scope,
		}
		if raw, err := os.ReadFile(filepath.Join(path, sidecarName)); err == nil {
			var meta sidecar
			if json.Unmarshal(raw, &meta) == nil {
				if meta.Source != "" {
					item.Source = meta.Source
				}
				if meta.SkillID != "" {
					item.SkillID = meta.SkillID
				}
			}
		}
		out = append(out, item)
	}
	return out
}

func RemoveSkill(dir, name string) error {
	name = strings.TrimSpace(name)
	if !validSkillID(name) {
		return fmt.Errorf("%w: %q", ErrBadSpec, name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSkillNotFound, name)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.Name() == name {
			return os.RemoveAll(path)
		}
		if parsed, err := skill.ParseFile(filepath.Join(path, skill.SkillFileName), dir); err == nil && parsed.Name == name {
			return os.RemoveAll(path)
		}
	}
	return fmt.Errorf("%w: %s", ErrSkillNotFound, name)
}

func validSkillID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, `/\`)
}

func hasFrontmatter(data []byte) bool {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return false
	}
	rest := text[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	return idx >= 0
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".skill-*")
	if err != nil {
		return fmt.Errorf("catalog: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("catalog: write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("catalog: close %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("catalog: chmod %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("catalog: move %s: %w", path, err)
	}
	return nil
}
