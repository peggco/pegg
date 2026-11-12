package install

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Source struct {
	Owner string
	Repo  string
}

var sourceSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ParseSource(value string) (Source, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return Source{}, fmt.Errorf("expected owner/repo, got %q", value)
	}
	owner, repo := parts[0], parts[1]
	if !sourceSegment.MatchString(owner) || !sourceSegment.MatchString(repo) {
		return Source{}, fmt.Errorf("invalid owner/repo %q", value)
	}
	if owner == "." || owner == ".." || repo == "." || repo == ".." {
		return Source{}, fmt.Errorf("invalid owner/repo %q", value)
	}
	return Source{Owner: owner, Repo: repo}, nil
}

func (s Source) Slug() string {
	return s.Owner + "/" + s.Repo
}

func (s Source) DefaultName() string {
	return sanitizeName(s.Repo)
}

func sanitizeName(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	if ext := filepath.Ext(value); ext != "" {
		value = strings.TrimSuffix(value, ext)
	}
	if value == "." || value == ".." || value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), ".-_")
	if name == "" {
		return ""
	}
	return name
}

func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("plugin name %q must not contain path separators", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("plugin name %q must not start with a dot", name)
	}
	if filepath.Ext(name) != "" {
		return fmt.Errorf("plugin name %q must not have an extension", name)
	}
	return nil
}
