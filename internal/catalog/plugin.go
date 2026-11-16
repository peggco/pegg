package catalog

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/peggco/pegg/internal/plugin/install"
)

type PluginEntry = Entry

type PluginCatalogItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Source       string   `json:"source"`
	URL          string   `json:"url,omitempty"`
	License      string   `json:"license,omitempty"`
	Official     bool     `json:"official"`
	Tags         []string `json:"tags,omitempty"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	Repository   string   `json:"repository,omitempty"`
	Homepage     string   `json:"homepage,omitempty"`
	RequiredEnv  []string `json:"requiredEnv,omitempty"`

	InstalledVersion string `json:"installedVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable,omitempty"`
}

func PluginItems(ctx context.Context, client *Client, opts SearchOptions, pluginDir string) ([]PluginCatalogItem, *int, error) {
	if client == nil {
		client = NewClient()
	}
	res, err := client.Search(ctx, KindPlugin, opts)
	if err != nil {
		return nil, nil, err
	}
	items := make([]PluginCatalogItem, 0, len(res.Items))
	for _, entry := range res.Items {
		items = append(items, ToPluginCatalogItem(entry))
	}
	return EnrichPluginItems(items, pluginDir), res.NextCursor, nil
}

func ToPluginCatalogItem(entry Entry) PluginCatalogItem {
	id := entry.ID
	if id == "" {
		id = entry.Source
	}
	if id == "" {
		id = entry.Name
	}
	source := entry.Source
	if source == "" {
		source = entry.Repository
	}
	url := entry.URL
	if url == "" {
		url = entry.Homepage
	}
	if url == "" && entry.Repository != "" {
		url = "https://github.com/" + entry.Repository
	}
	return PluginCatalogItem{
		ID:           id,
		Name:         entry.Name,
		Description:  entry.Description,
		Source:       source,
		URL:          url,
		License:      entry.License,
		Official:     entry.Official,
		Tags:         entry.Tags,
		Version:      entry.Version,
		Capabilities: entry.Capabilities,
		Platforms:    entry.Platforms,
		Repository:   entry.Repository,
		Homepage:     entry.Homepage,
		RequiredEnv:  entry.RequiredEnv,
	}
}

func EnrichPluginItems(items []PluginCatalogItem, pluginDir string) []PluginCatalogItem {
	if len(items) == 0 || pluginDir == "" {
		return items
	}
	installed, err := install.List(pluginDir)
	if err != nil || len(installed) == 0 {
		return items
	}
	versions := make(map[string]string, len(installed))
	for _, item := range installed {
		versions[strings.ToLower(item.Name)] = item.Version
	}
	for i := range items {
		version, ok := installedPluginVersion(items[i], versions)
		if !ok {
			continue
		}
		items[i].InstalledVersion = version
		items[i].UpdateAvailable = pluginUpdateAvailable(version, items[i].Version)
	}
	return items
}

func installedPluginVersion(item PluginCatalogItem, versions map[string]string) (string, bool) {
	candidates := []string{item.ID, item.Name, item.Source}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		names := []string{candidate}
		if base := candidate[strings.LastIndex(candidate, "/")+1:]; base != candidate {
			names = append(names, base)
		}
		for _, name := range names {
			if version, ok := versions[strings.ToLower(name)]; ok {
				return version, true
			}
		}
	}
	return "", false
}

func pluginUpdateAvailable(installedVersion, catalogVersion string) bool {
	if strings.TrimSpace(installedVersion) == "" || strings.TrimSpace(catalogVersion) == "" {
		return false
	}
	return install.VersionNewer(installedVersion, catalogVersion)
}

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
