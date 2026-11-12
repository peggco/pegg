package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

type PluginEntry = Entry

type PluginSpec struct {
	Owner string
	Repo  string
}

func (s PluginSpec) String() string {
	return s.Owner + "/" + s.Repo
}

var pluginSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ParsePluginSpec(raw string) (PluginSpec, error) {
	value := strings.Trim(strings.TrimSpace(raw), "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return PluginSpec{}, fmt.Errorf("%w: %q must be <owner>/<repo>", ErrBadSpec, raw)
	}
	owner, repo := parts[0], parts[1]
	if !pluginSegmentPattern.MatchString(owner) || !pluginSegmentPattern.MatchString(repo) {
		return PluginSpec{}, fmt.Errorf("%w: invalid owner/repo %q", ErrBadSpec, raw)
	}
	if owner == "." || owner == ".." || repo == "." || repo == ".." {
		return PluginSpec{}, fmt.Errorf("%w: invalid owner/repo %q", ErrBadSpec, raw)
	}
	return PluginSpec{Owner: owner, Repo: repo}, nil
}

func ParsePluginID(raw string) (string, error) {
	spec, err := ParsePluginSpec(raw)
	if err != nil {
		return "", err
	}
	return spec.String(), nil
}

func ValidatePluginID(id string) error {
	_, err := ParsePluginSpec(id)
	return err
}
