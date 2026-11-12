package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/plugin"
)

const stateDirName = ".state"

type Metadata struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	Tag         string    `json:"tag"`
	Version     string    `json:"version"`
	Asset       string    `json:"asset"`
	URL         string    `json:"url"`
	SHA256      string    `json:"sha256"`
	InstalledAt time.Time `json:"installed_at"`
}

type Installed struct {
	Name        string
	Path        string
	Source      string
	Tag         string
	Version     string
	Asset       string
	SHA256      string
	InstalledAt time.Time
	Manual      bool
}

func StateDir(pluginDir string) string {
	return filepath.Join(pluginDir, stateDirName)
}

func ReadMetadata(pluginDir, name string) (*Metadata, error) {
	data, err := os.ReadFile(metadataPath(pluginDir, name))
	if err != nil {
		return nil, err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("decode metadata for %s: %w", name, err)
	}
	return &metadata, nil
}

func WriteMetadata(pluginDir string, metadata *Metadata) error {
	if metadata == nil || metadata.Name == "" {
		return fmt.Errorf("metadata name is required")
	}
	dir := StateDir(pluginDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".metadata-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, metadataPath(pluginDir, metadata.Name))
}

func DeleteMetadata(pluginDir, name string) error {
	err := os.Remove(metadataPath(pluginDir, name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func ListMetadata(pluginDir string) ([]*Metadata, error) {
	entries, err := os.ReadDir(StateDir(pluginDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Metadata
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(StateDir(pluginDir), entry.Name()))
		if err != nil {
			continue
		}
		var metadata Metadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}
		out = append(out, &metadata)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func FindMetadata(pluginDir, nameOrSource string) (*Metadata, error) {
	value := strings.TrimSpace(nameOrSource)
	if value == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	list, err := ListMetadata(pluginDir)
	if err != nil {
		return nil, err
	}
	var matches []*Metadata
	for _, metadata := range list {
		if strings.EqualFold(metadata.Name, value) ||
			strings.EqualFold(metadata.Source, value) ||
			strings.EqualFold(filepath.Base(metadata.Source), value) {
			matches = append(matches, metadata)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.Name+" ("+match.Source+")")
		}
		return nil, fmt.Errorf("ambiguous plugin %q: %s", value, strings.Join(names, ", "))
	}
	return nil, os.ErrNotExist
}

func List(pluginDir string) ([]Installed, error) {
	paths, err := plugin.ScanPlugins(pluginDir)
	if err != nil {
		return nil, err
	}
	out := make([]Installed, 0, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		item := Installed{Name: name, Path: path, Manual: true}
		if metadata, err := ReadMetadata(pluginDir, name); err == nil {
			item.Source = metadata.Source
			item.Tag = metadata.Tag
			item.Version = metadata.Version
			item.Asset = metadata.Asset
			item.SHA256 = metadata.SHA256
			item.InstalledAt = metadata.InstalledAt
			item.Manual = false
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Resolve(pluginDir, nameOrSource string) (*Installed, error) {
	value := strings.TrimSpace(nameOrSource)
	if value == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	if strings.Contains(value, "/") {
		source, err := ParseSource(value)
		if err != nil {
			return nil, err
		}
		if metadata, err := FindMetadata(pluginDir, source.Slug()); err == nil {
			return installedFromMetadata(pluginDir, metadata), nil
		}
		path := filepath.Join(pluginDir, source.DefaultName())
		if _, err := os.Stat(path); err == nil {
			return &Installed{Name: source.DefaultName(), Path: path, Manual: true}, nil
		}
		return nil, fmt.Errorf("plugin %q is not installed", value)
	}
	if metadata, err := FindMetadata(pluginDir, value); err == nil {
		return installedFromMetadata(pluginDir, metadata), nil
	}
	if err := ValidateName(value); err != nil {
		return nil, err
	}
	path := filepath.Join(pluginDir, value)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("plugin %q is not installed", value)
	}
	return &Installed{Name: value, Path: path, Manual: true}, nil
}

func installedFromMetadata(pluginDir string, metadata *Metadata) *Installed {
	return &Installed{
		Name:        metadata.Name,
		Path:        filepath.Join(pluginDir, metadata.Name),
		Source:      metadata.Source,
		Tag:         metadata.Tag,
		Version:     metadata.Version,
		Asset:       metadata.Asset,
		SHA256:      metadata.SHA256,
		InstalledAt: metadata.InstalledAt,
	}
}

func metadataPath(pluginDir, name string) string {
	return filepath.Join(StateDir(pluginDir), name+".json")
}
