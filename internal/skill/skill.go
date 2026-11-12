package skill

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
	ToolBundles   []string
	WhenToUse     string
	ArgumentHint  string
	Arguments     []string
	Context       string
	Instructions  string
	Source        string
	Path          string
	ScriptsPath   string
	HasScripts    bool
}

var (
	ErrNilSkill  = errors.New("skill: nil skill")
	ErrEmptyName = errors.New("skill: empty name")
	ErrDuplicate = errors.New("skill: already registered")
	ErrNotFound  = errors.New("skill: not found")
	ErrInvalid   = errors.New("skill: invalid")
)

var nameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const maxNameLen = 64

func ValidateName(name string) error {
	if name == "" {
		return ErrEmptyName
	}
	if len(name) > maxNameLen || !nameRe.MatchString(name) {
		return fmt.Errorf("%w: name %q must be 1-%d lowercase alphanumeric characters or hyphens", ErrInvalid, name, maxNameLen)
	}
	return nil
}

func validName(name string) bool {
	return name != "" && len(name) <= maxNameLen && nameRe.MatchString(name)
}

type registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

var reg = &registry{skills: make(map[string]*Skill)}

var (
	dynamicMu     sync.RWMutex
	dynamicSkills []*Skill
)

func RegisterDynamic(s *Skill) error {
	if s == nil {
		return ErrNilSkill
	}
	if err := ValidateName(s.Name); err != nil {
		return err
	}
	dynamicMu.Lock()
	replaced := false
	for i, existing := range dynamicSkills {
		if existing != nil && existing.Name == s.Name {
			dynamicSkills[i] = s
			replaced = true
			break
		}
	}
	if !replaced {
		dynamicSkills = append(dynamicSkills, s)
	}
	dynamicMu.Unlock()
	registerOrReplace(s)
	return nil
}

func dynamicSnapshot() []*Skill {
	dynamicMu.RLock()
	defer dynamicMu.RUnlock()
	out := make([]*Skill, len(dynamicSkills))
	copy(out, dynamicSkills)
	return out
}

func Register(s *Skill) error {
	if s == nil {
		return ErrNilSkill
	}
	if err := ValidateName(s.Name); err != nil {
		return err
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if _, ok := reg.skills[s.Name]; ok {
		return fmt.Errorf("%w: %q", ErrDuplicate, s.Name)
	}
	reg.skills[s.Name] = s
	return nil
}

func registerOrReplace(s *Skill) {
	if s == nil || !validName(s.Name) {
		return
	}
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.skills[s.Name] = s
}

func Get(name string) (*Skill, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	s, ok := reg.skills[name]
	return s, ok
}

func Has(name string) bool {
	_, ok := Get(name)
	return ok
}

func List() []*Skill {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	out := make([]*Skill, 0, len(reg.skills))
	for _, s := range reg.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
