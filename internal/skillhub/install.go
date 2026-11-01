package skillhub

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

type Installed struct {
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

func Install(ctx context.Context, c *Client, spec Spec, dir string) (Installed, error) {
	if strings.TrimSpace(dir) == "" {
		return Installed{}, fmt.Errorf("%w: target directory is empty", ErrBadSpec)
	}
	if !validSkillID(spec.SkillID) {
		return Installed{}, fmt.Errorf("%w: %q", ErrBadSpec, spec.SkillID)
	}
	data, _, err := c.Resolve(ctx, spec)
	if err != nil {
		return Installed{}, err
	}
	if !hasFrontmatter(data) {
		return Installed{}, fmt.Errorf("%w: %s has no frontmatter", ErrBadSpec, spec.String())
	}

	target := filepath.Join(dir, spec.SkillID)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return Installed{}, fmt.Errorf("skillhub: create %s: %w", target, err)
	}
	if err := writeAtomic(filepath.Join(target, skill.SkillFileName), data); err != nil {
		return Installed{}, err
	}
	meta, _ := json.Marshal(sidecar{Source: spec.Source, SkillID: spec.SkillID})
	if err := writeAtomic(filepath.Join(target, sidecarName), meta); err != nil {
		return Installed{}, err
	}

	out := Installed{
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

func ListInstalled(dir, scope string) []Installed {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []Installed{}
	}
	out := make([]Installed, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		parsed, err := skill.ParseFile(filepath.Join(path, skill.SkillFileName), dir)
		if err != nil {
			continue
		}
		item := Installed{
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

func Remove(dir, name string) error {
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
		return fmt.Errorf("skillhub: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("skillhub: write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("skillhub: close %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("skillhub: chmod %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("skillhub: move %s: %w", path, err)
	}
	return nil
}
