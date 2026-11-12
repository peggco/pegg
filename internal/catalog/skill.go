package catalog

import (
	"errors"
	"strings"
)

var (
	ErrSkillNotFound     = errors.New("catalog: skill not found")
	ErrSourceUnsupported = errors.New("catalog: source is not a GitHub repository")
)

type RemoteSkill = Entry

type SkillSpec struct {
	Source  string
	SkillID string
}

func (s SkillSpec) String() string { return s.Source + "/" + s.SkillID }

func isGitHubSource(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}
