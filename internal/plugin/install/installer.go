package install

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/peggco/pegg/internal/plugin"
)

type Options struct {
	Source          Source
	Version         string
	AssetPattern    string
	BinaryPath      string
	Name            string
	Force           bool
	SkipChecksum    bool
	RequireChecksum bool
	Dir             string
	GOOS            string
	GOARCH          string
	Check           bool
	Out             io.Writer
}

type Result struct {
	Name            string
	Path            string
	Source          string
	Tag             string
	Version         string
	Asset           string
	SHA256          string
	UpdateAvailable bool
	Updated         bool
	PreviousVersion string
}

func Install(ctx context.Context, client *Client, opts Options) (*Result, error) {
	release, err := resolveRelease(ctx, client, opts)
	if err != nil {
		return nil, err
	}
	return installRelease(ctx, client, opts, release)
}

func Update(ctx context.Context, client *Client, opts Options) (*Result, error) {
	pluginDir, err := resolveDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	metadata, err := FindMetadata(pluginDir, opts.Source.Slug())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin %s was not installed by `pegg plugins install`", opts.Source.Slug())
		}
		return nil, err
	}
	source, err := ParseSource(metadata.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid stored source %q: %w", metadata.Source, err)
	}
	opts.Source = source
	opts.Name = metadata.Name
	release, err := resolveRelease(ctx, client, opts)
	if err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	result := &Result{
		Name:            metadata.Name,
		Path:            filepath.Join(pluginDir, metadata.Name),
		Source:          metadata.Source,
		Tag:             metadata.Tag,
		Version:         metadata.Version,
		PreviousVersion: metadata.Version,
	}
	if !opts.Force && !VersionNewer(metadata.Version, latest) {
		return result, nil
	}
	result.UpdateAvailable = true
	if opts.Check {
		result.Tag = release.TagName
		result.Version = latest
		return result, nil
	}
	opts.Force = true
	installed, err := installRelease(ctx, client, opts, release)
	if err != nil {
		return nil, err
	}
	installed.UpdateAvailable = true
	installed.Updated = true
	installed.PreviousVersion = metadata.Version
	return installed, nil
}

func Remove(dir, nameOrSource string) error {
	pluginDir, err := resolveDir(dir)
	if err != nil {
		return err
	}
	item, err := Resolve(pluginDir, nameOrSource)
	if err != nil {
		return err
	}
	if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if !item.Manual {
		if err := DeleteMetadata(pluginDir, item.Name); err != nil {
			return err
		}
	}
	return nil
}

func VersionNewer(current, latest string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	if latest == "" {
		return false
	}
	if current == "" {
		return true
	}
	currentVersion, err1 := semver.NewVersion(current)
	latestVersion, err2 := semver.NewVersion(latest)
	if err1 == nil && err2 == nil {
		return latestVersion.GreaterThan(currentVersion)
	}
	return current != latest
}

func resolveRelease(ctx context.Context, client *Client, opts Options) (*Release, error) {
	if client == nil {
		return nil, fmt.Errorf("GitHub client is required")
	}
	if strings.TrimSpace(opts.Version) != "" {
		return client.ByTag(ctx, opts.Source, opts.Version)
	}
	return client.Latest(ctx, opts.Source)
}

func installRelease(ctx context.Context, client *Client, opts Options, release *Release) (*Result, error) {
	if opts.Source.Owner == "" || opts.Source.Repo == "" {
		return nil, fmt.Errorf("owner/repo is required")
	}
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("the plugin scanner currently supports extensionless Unix plugin binaries; Windows plugins are not supported")
	}
	pluginDir, err := resolveDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = opts.Source.DefaultName()
	}
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	asset, err := SelectAsset(release, SelectOptions{
		Pattern: opts.AssetPattern,
		GOOS:    goos,
		GOARCH:  goarch,
		Repo:    opts.Source.Repo,
	})
	if err != nil {
		return nil, err
	}

	expected := ""
	if !opts.SkipChecksum && os.Getenv("PEGG_SKIP_PLUGIN_CHECKSUM") != "1" {
		expected, err = ExpectedSHA256(ctx, client, release, asset)
		if err != nil {
			if opts.RequireChecksum {
				return nil, err
			}
			warnf(opts.Out, "checksum unavailable for %s: %v", asset.Name, err)
		}
		if expected == "" {
			if opts.RequireChecksum {
				return nil, fmt.Errorf("checksum is required but not available for %s", asset.Name)
			}
			warnf(opts.Out, "no checksum found for %s; installing without verification", asset.Name)
		}
	}

	data, err := client.Download(ctx, asset.URL, client.MaxAssetBytes)
	if err != nil {
		return nil, err
	}
	if expected != "" {
		if err := VerifySHA256(data, expected); err != nil {
			return nil, fmt.Errorf("%s: %w", asset.Name, err)
		}
	}

	binary, err := ExtractBinary(asset.Name, data, ExtractOptions{
		BinaryPath:  opts.BinaryPath,
		InstallName: name,
		Repo:        opts.Source.Repo,
	})
	if err != nil {
		return nil, err
	}
	if len(binary) == 0 {
		return nil, fmt.Errorf("selected binary is empty")
	}

	target := filepath.Join(pluginDir, name)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", target)
	}
	if _, err := os.Stat(target); err == nil && !opts.Force {
		return nil, fmt.Errorf("plugin %s is already installed (use --force to replace)", name)
	}
	if existing, err := ReadMetadata(pluginDir, name); err == nil &&
		!strings.EqualFold(existing.Source, opts.Source.Slug()) && !opts.Force {
		return nil, fmt.Errorf("plugin %s is managed by %s (use --force to replace)", name, existing.Source)
	}

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return nil, err
	}
	backup := ""
	if _, err := os.Stat(target); err == nil {
		backup = filepath.Join(pluginDir, fmt.Sprintf(".%s.backup-%d", name, time.Now().UnixNano()))
		if err := os.Rename(target, backup); err != nil {
			return nil, err
		}
	}
	if err := writeExecutable(pluginDir, target, binary); err != nil {
		restoreBackup(target, backup)
		return nil, err
	}

	metadata := &Metadata{
		Name:        name,
		Source:      opts.Source.Slug(),
		Tag:         release.TagName,
		Version:     strings.TrimPrefix(strings.TrimSpace(release.TagName), "v"),
		Asset:       asset.Name,
		URL:         asset.URL,
		SHA256:      SHA256Hex(data),
		InstalledAt: time.Now().UTC(),
	}
	if err := WriteMetadata(pluginDir, metadata); err != nil {
		_ = os.Remove(target)
		restoreBackup(target, backup)
		return nil, err
	}
	if backup != "" {
		_ = os.Remove(backup)
	}
	return &Result{
		Name:    name,
		Path:    target,
		Source:  metadata.Source,
		Tag:     metadata.Tag,
		Version: metadata.Version,
		Asset:   metadata.Asset,
		SHA256:  metadata.SHA256,
	}, nil
}

func writeExecutable(dir, target string, data []byte) error {
	temp, err := os.CreateTemp(dir, ".pegg-plugin-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o755); err != nil {
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
	return os.Rename(tempName, target)
}

func restoreBackup(target, backup string) {
	if backup == "" {
		return
	}
	_ = os.Rename(backup, target)
}

func resolveDir(dir string) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	return plugin.GetPluginDir()
}

func warnf(out io.Writer, format string, args ...any) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "warning: "+format+"\n", args...)
}
