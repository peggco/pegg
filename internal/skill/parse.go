package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const SkillFileName = "SKILL.md"

type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
	WhenToUse     string            `yaml:"when_to_use"`
	ArgumentHint  string            `yaml:"argument-hint"`
	Arguments     []string          `yaml:"arguments"`
	Context       string            `yaml:"context"`
}

func ParseFile(path, source string) (*Skill, error) {
	return parseSKILL(path, source)
}

func parseSKILL(path, source string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	s := &Skill{
		Name:          fm.Name,
		Description:   fm.Description,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      fm.Metadata,
		AllowedTools:  strings.Fields(fm.AllowedTools),
		WhenToUse:     fm.WhenToUse,
		ArgumentHint:  fm.ArgumentHint,
		Arguments:     fm.Arguments,
		Context:       fm.Context,
		Instructions:  body,
		Source:        source,
		Path:          dir,
	}
	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	scripts := filepath.Join(dir, "scripts")
	if fi, err := os.Stat(scripts); err == nil && fi.IsDir() {
		s.HasScripts = true
		s.ScriptsPath = scripts
	}
	return s, nil
}

func parseFrontmatter(content string) (frontmatter, string, error) {
	var fm frontmatter
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, "", fmt.Errorf("%w: %s must start with a --- frontmatter block", ErrInvalid, SkillFileName)
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return fm, "", fmt.Errorf("%w: unclosed frontmatter block", ErrInvalid)
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return fm, "", fmt.Errorf("%w: parse frontmatter: %v", ErrInvalid, err)
	}
	body := strings.Join(lines[end+1:], "\n")
	return fm, strings.TrimLeft(body, "\r\n"), nil
}
