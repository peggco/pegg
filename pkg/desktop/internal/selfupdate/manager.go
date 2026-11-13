package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	goselfupdate "github.com/creativeprojects/go-selfupdate"

	"github.com/peggco/pegg/internal/core/update"
	"github.com/peggco/pegg/pkg/desktop/internal/binary"
)

const (
	repoSlug          = "peggco/pegg"
	manifestName      = "manifest.json"
	agentManifestName = "agent-manifest.json"
)

type manifest struct {
	Version string `json:"version"`
	Asset   string `json:"asset"`
	SHA256  string `json:"sha256"`
	Path    string `json:"path"`
}

type Manager struct {
	CurrentVersion string
	GOOS           string
	GOARCH         string
	TargetPath     string
	CacheDir       string
	BaseURL        string
	HTTP           *http.Client

	mu sync.Mutex

	status Status

	assetName     string
	assetURL      string
	stagedPath    string
	stagedVersion string

	agentVersion       string
	agentSupported     bool
	agentTarget        string
	agentAssetName     string
	agentStagedPath    string
	agentStagedVersion string
}

func NewManager(currentVersion, targetPath, cacheDir string) *Manager {
	m := &Manager{
		CurrentVersion: currentVersion,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		TargetPath:     targetPath,
		CacheDir:       cacheDir,
		BaseURL:        binary.DefaultBaseURL,
		HTTP:           &http.Client{Timeout: 30 * time.Minute},
	}
	m.status = Status{
		CurrentVersion: currentVersion,
		State:          StateIdle,
		Supported:      supportedTarget(m.GOOS, filepath.Dir(targetPath)),
		TargetPath:     targetPath,
	}
	m.loadStaged()
	return m
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) SetAgent(version string, supported bool, target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentVersion = version
	m.agentSupported = supported
	m.agentTarget = target
	if m.agentStagedVersion != "" && (version == "" || !update.IsNewerThan(version, m.agentStagedVersion)) {
		m.clearAgentStagedLocked()
	}
	if m.status.LatestVersion == "" && m.agentStagedVersion != "" {
		m.status.LatestVersion = m.agentStagedVersion
	}
	m.recomputeLocked()
}

func (m *Manager) Check(ctx context.Context) (Status, error) {
	m.mu.Lock()
	m.status.State = StateChecking
	m.status.Error = ""
	m.mu.Unlock()

	updater, err := goselfupdate.NewUpdater(goselfupdate.Config{Filters: []string{AssetFilter(m.GOOS, m.GOARCH)}})
	if err != nil {
		return m.fail(fmt.Errorf("create updater: %w", err))
	}
	release, found, err := updater.DetectLatest(ctx, goselfupdate.ParseSlug(repoSlug))
	if err != nil {
		return m.fail(fmt.Errorf("detect latest: %w", err))
	}
	if !found {
		m.mu.Lock()
		m.assetName = ""
		m.assetURL = ""
		m.status.LatestVersion = ""
		m.status.ReleaseURL = ""
		m.status.State = StateIdle
		m.recomputeLocked()
		status := m.status
		m.mu.Unlock()
		return status, nil
	}

	latest := release.Version()
	m.mu.Lock()
	m.assetName = release.AssetName
	m.assetURL = release.AssetURL
	m.agentAssetName = AgentAssetName(m.GOOS, m.GOARCH)
	m.status.LatestVersion = latest
	m.status.ReleaseURL = release.URL
	if m.stagedVersion != "" && m.stagedVersion != latest {
		m.clearDesktopStagedLocked()
	}
	if m.agentStagedVersion != "" && m.agentStagedVersion != latest {
		m.clearAgentStagedLocked()
	}
	m.status.State = StateIdle
	m.recomputeLocked()
	status := m.status
	m.mu.Unlock()
	return status, nil
}

func (m *Manager) Download(ctx context.Context, progress func(Status)) (Status, error) {
	status := m.Status()
	if status.LatestVersion == "" {
		checked, err := m.Check(ctx)
		if err != nil {
			return checked, err
		}
		status = checked
	}
	desktopNeeded := status.DesktopAvailable && status.Supported && !status.DesktopReady
	agentNeeded := status.AgentAvailable && status.AgentSupported && !status.AgentReady
	if !desktopNeeded && !agentNeeded {
		return status, nil
	}

	m.mu.Lock()
	assetName := m.assetName
	agentAssetName := m.agentAssetName
	latest := m.status.LatestVersion
	m.mu.Unlock()
	if assetName == "" || agentAssetName == "" {
		return status, errors.New("update metadata is missing")
	}

	if err := os.MkdirAll(m.CacheDir, 0o755); err != nil {
		return m.fail(err)
	}
	workDir, err := os.MkdirTemp(m.CacheDir, ".download-*")
	if err != nil {
		return m.fail(err)
	}
	defer os.RemoveAll(workDir)

	report := func(state State, percent int) {
		m.setProgress(state, percent)
		if progress != nil {
			progress(m.Status())
		}
	}
	report(StateDownloading, 5)

	rangeFor := func(start, end int) func(int) {
		return func(percent int) {
			report(StateDownloading, start+(end-start)*percent/100)
		}
	}
	desktopRange := rangeFor(5, 85)
	agentRange := rangeFor(5, 85)
	if desktopNeeded && agentNeeded {
		desktopRange = rangeFor(5, 45)
		agentRange = rangeFor(50, 90)
	}

	if desktopNeeded {
		dest := filepath.Join(m.CacheDir, latest, BinaryName(m.GOOS))
		sha, err := m.stage(ctx, workDir, assetName, m.assetURLFor(latest, assetName), m.assetURLFor(latest, ChecksumAssetName()), BinaryName(m.GOOS), dest, desktopRange)
		if err != nil {
			return m.fail(err)
		}
		m.mu.Lock()
		m.stagedPath = dest
		m.stagedVersion = latest
		m.mu.Unlock()
		if err := writeManifest(filepath.Join(m.CacheDir, manifestName), manifest{Version: latest, Asset: assetName, SHA256: sha, Path: dest}); err != nil {
			return m.fail(err)
		}
	}

	if agentNeeded {
		dest := filepath.Join(m.CacheDir, latest, binary.Name())
		sha, err := m.stage(ctx, workDir, agentAssetName, m.assetURLFor(latest, agentAssetName), m.assetURLFor(latest, "checksums.txt"), binary.Name(), dest, agentRange)
		if err != nil {
			return m.fail(err)
		}
		m.mu.Lock()
		m.agentStagedPath = dest
		m.agentStagedVersion = latest
		m.mu.Unlock()
		if err := writeManifest(filepath.Join(m.CacheDir, agentManifestName), manifest{Version: latest, Asset: agentAssetName, SHA256: sha, Path: dest}); err != nil {
			return m.fail(err)
		}
	}

	m.mu.Lock()
	m.status.Progress = 100
	m.status.Error = ""
	m.status.State = StateIdle
	m.recomputeLocked()
	status = m.status
	m.mu.Unlock()
	if progress != nil {
		progress(status)
	}
	return status, nil
}

func (m *Manager) ApplyDesktop(ctx context.Context) error {
	m.mu.Lock()
	staged, version := m.stagedPath, m.stagedVersion
	target := m.TargetPath
	supported := m.status.Supported
	m.mu.Unlock()

	if staged == "" {
		return nil
	}
	if !supported {
		return errors.New("desktop self-update is not supported for this install")
	}
	if !update.IsNewerThan(m.CurrentVersion, version) {
		return fmt.Errorf("staged desktop version %s is not newer than %s", version, m.CurrentVersion)
	}
	if _, err := os.Stat(staged); err != nil {
		return fmt.Errorf("staged desktop binary: %w", err)
	}
	if err := apply(target, staged); err != nil {
		return fmt.Errorf("apply desktop update: %w", err)
	}
	m.mu.Lock()
	m.clearDesktopStagedLocked()
	m.status.State = StateIdle
	m.recomputeLocked()
	m.mu.Unlock()
	return nil
}

func (m *Manager) ApplyAgent(ctx context.Context) error {
	m.mu.Lock()
	staged, version := m.agentStagedPath, m.agentStagedVersion
	target := m.agentTarget
	supported := m.agentSupported
	current := m.agentVersion
	m.mu.Unlock()

	if staged == "" {
		return nil
	}
	if !supported {
		return errors.New("agent self-update is not supported for this install")
	}
	if !update.IsNewerThan(current, version) {
		return fmt.Errorf("staged agent version %s is not newer than %s", version, current)
	}
	if _, err := os.Stat(staged); err != nil {
		return fmt.Errorf("staged agent binary: %w", err)
	}
	if err := apply(target, staged); err != nil {
		return fmt.Errorf("apply agent update: %w", err)
	}
	m.mu.Lock()
	m.clearAgentStagedLocked()
	m.status.State = StateIdle
	m.recomputeLocked()
	m.mu.Unlock()
	return nil
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	desktopVersion := m.stagedVersion
	agentVersion := m.agentStagedVersion
	agentTarget := m.agentTarget
	m.mu.Unlock()

	_ = os.Remove(m.TargetPath + ".old")
	_ = os.Remove(m.TargetPath + ".new")
	if agentTarget != "" {
		_ = os.Remove(agentTarget + ".old")
		_ = os.Remove(agentTarget + ".new")
	}

	entries, err := os.ReadDir(m.CacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == desktopVersion || name == agentVersion {
			continue
		}
		_ = os.RemoveAll(filepath.Join(m.CacheDir, name))
	}
}

func (m *Manager) Target() string {
	return m.TargetPath
}

func (m *Manager) AgentTarget() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agentTarget
}

func (m *Manager) assetURLFor(version, asset string) string {
	base := strings.TrimRight(m.BaseURL, "/")
	return fmt.Sprintf("%s/v%s/%s", base, strings.TrimPrefix(version, "v"), asset)
}

func (m *Manager) stage(
	ctx context.Context,
	workDir, assetName, assetURL, checksumsURL, member, dest string,
	progress func(int),
) (string, error) {
	archive := filepath.Join(workDir, assetName)
	if err := binary.DownloadTo(ctx, m.HTTP, assetURL, archive, progress); err != nil {
		return "", fmt.Errorf("download %s: %w", assetName, err)
	}
	sums, err := binary.FetchChecksums(ctx, m.HTTP, checksumsURL)
	if err != nil {
		return "", fmt.Errorf("checksums: %w", err)
	}
	want, ok := sums[assetName]
	if !ok {
		return "", fmt.Errorf("checksums file has no entry for %s", assetName)
	}
	got, err := binary.FileSHA256(archive)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(got, want) {
		return "", fmt.Errorf("checksum mismatch for %s", assetName)
	}
	binPath := filepath.Join(workDir, member)
	if strings.HasSuffix(assetName, ".zip") {
		err = binary.ExtractZip(archive, member, binPath)
	} else {
		err = binary.ExtractTarGz(archive, member, binPath)
	}
	if err != nil {
		return "", fmt.Errorf("extract %s: %w", assetName, err)
	}
	if m.GOOS != "windows" {
		_ = os.Chmod(binPath, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := binary.CopyFile(binPath, dest); err != nil {
		return "", err
	}
	if m.GOOS != "windows" {
		_ = os.Chmod(dest, 0o755)
	}
	_ = os.Remove(archive)
	_ = os.Remove(binPath)
	return got, nil
}

func (m *Manager) loadStaged() {
	desktopManifest := filepath.Join(m.CacheDir, manifestName)
	if item, ok := readManifest(desktopManifest); ok {
		if update.IsNewerThan(m.CurrentVersion, item.Version) {
			if _, err := os.Stat(item.Path); err == nil {
				m.stagedPath = item.Path
				m.stagedVersion = item.Version
				m.status.LatestVersion = item.Version
			} else {
				_ = os.Remove(desktopManifest)
			}
		} else {
			_ = os.Remove(item.Path)
			_ = os.Remove(desktopManifest)
		}
	}

	agentManifest := filepath.Join(m.CacheDir, agentManifestName)
	if item, ok := readManifest(agentManifest); ok {
		if m.agentVersion == "" || update.IsNewerThan(m.agentVersion, item.Version) {
			if _, err := os.Stat(item.Path); err == nil {
				m.agentStagedPath = item.Path
				m.agentStagedVersion = item.Version
				if m.status.LatestVersion == "" || update.IsNewerThan(m.status.LatestVersion, item.Version) {
					m.status.LatestVersion = item.Version
				}
			} else {
				_ = os.Remove(agentManifest)
			}
		} else {
			_ = os.Remove(item.Path)
			_ = os.Remove(agentManifest)
		}
	}
	m.recomputeLocked()
}

func readManifest(path string) (manifest, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, false
	}
	var item manifest
	if err := json.Unmarshal(data, &item); err != nil || item.Version == "" || item.Path == "" {
		return manifest{}, false
	}
	return item, true
}

func writeManifest(path string, item manifest) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (m *Manager) clearDesktopStagedLocked() {
	if m.stagedPath != "" {
		_ = os.Remove(m.stagedPath)
	}
	_ = os.Remove(filepath.Join(m.CacheDir, manifestName))
	m.stagedPath = ""
	m.stagedVersion = ""
	m.status.DesktopReady = false
}

func (m *Manager) clearAgentStagedLocked() {
	if m.agentStagedPath != "" {
		_ = os.Remove(m.agentStagedPath)
	}
	_ = os.Remove(filepath.Join(m.CacheDir, agentManifestName))
	m.agentStagedPath = ""
	m.agentStagedVersion = ""
	m.status.AgentReady = false
}

func (m *Manager) recomputeLocked() {
	latest := m.status.LatestVersion
	if latest == "" {
		m.status.Available = false
		m.status.Ready = false
		m.status.DesktopAvailable = false
		m.status.DesktopReady = false
		m.status.AgentAvailable = false
		m.status.AgentReady = false
		if m.status.State != StateChecking && m.status.State != StateError && m.status.State != StateDownloading {
			m.status.State = StateIdle
		}
		return
	}

	desktopAvailable := update.IsNewerThan(m.CurrentVersion, latest)
	agentAvailable := m.agentVersion != "" && update.IsNewerThan(m.agentVersion, latest)
	m.status.DesktopAvailable = desktopAvailable
	m.status.AgentCurrentVersion = m.agentVersion
	m.status.AgentLatestVersion = latest
	m.status.AgentAvailable = agentAvailable
	m.status.AgentSupported = m.agentSupported
	m.status.AgentTargetPath = m.agentTarget
	m.status.DesktopReady = desktopAvailable && m.stagedVersion == latest
	m.status.AgentReady = agentAvailable && m.agentStagedVersion == latest
	m.status.Available = desktopAvailable || agentAvailable

	desktopSatisfied := !desktopAvailable || !m.status.Supported || m.status.DesktopReady
	agentSatisfied := !agentAvailable || !m.agentSupported || m.status.AgentReady
	supportedAvailable := (desktopAvailable && m.status.Supported) || (agentAvailable && m.agentSupported)
	m.status.Ready = supportedAvailable && desktopSatisfied && agentSatisfied
	if m.status.State == StateChecking || m.status.State == StateError || m.status.State == StateDownloading {
		return
	}
	switch {
	case m.status.Ready:
		m.status.State = StateReady
	case m.status.Available:
		if supportedAvailable {
			m.status.State = StateAvailable
		} else {
			m.status.State = StateUnsupported
		}
	default:
		m.status.State = StateIdle
	}
}

func (m *Manager) setProgress(state State, percent int) {
	m.mu.Lock()
	m.status.State = state
	m.status.Progress = percent
	m.mu.Unlock()
}

func (m *Manager) fail(err error) (Status, error) {
	m.mu.Lock()
	m.status.State = StateError
	m.status.Error = err.Error()
	status := m.status
	m.mu.Unlock()
	return status, err
}
