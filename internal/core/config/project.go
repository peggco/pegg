package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	projectRootOnce     sync.Once
	projectRootPath     string
	projectRootFallback bool
	projectRootReason   string
	projectRootErr      error
)

func ProjectLocalRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return filepath.Join(cwd, ProjectConfigDirName), nil
}

func ProjectRoot() (string, error) {
	projectRootOnce.Do(resolveProjectRoot)
	return projectRootPath, projectRootErr
}

func ProjectRootFallback() bool {
	projectRootOnce.Do(resolveProjectRoot)
	return projectRootFallback
}

func ProjectRootNotice() string {
	projectRootOnce.Do(resolveProjectRoot)
	return projectRootReason
}

func ProjectReadPaths(sub string) []string {
	root, err := ProjectRoot()
	if err != nil {
		return nil
	}
	var out []string
	local, err := ProjectLocalRoot()
	if err == nil {
		candidate := filepath.Join(local, sub)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			out = append(out, candidate)
		}
	}
	resolved := filepath.Join(root, sub)
	if len(out) == 0 || out[0] != resolved {
		out = append(out, resolved)
	}
	return out
}

func GetProjectConfigPath(paths ...string) (string, error) {
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	allPaths := append([]string{root}, paths...)
	return filepath.Join(allPaths...), nil
}

func EnsureProjectConfigDir() error {
	root, err := ProjectRoot()
	if err != nil {
		return err
	}
	return os.MkdirAll(root, 0o755)
}

func ProjectRootFor(dir string) (string, bool, error) {
	if dir == "" {
		root, err := ProjectRoot()
		return root, ProjectRootFallback(), err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	local := filepath.Join(abs, ProjectConfigDirName)
	if usable, _ := localUsable(local, abs); usable {
		return local, false, nil
	}
	global, err := GetConfigDir()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(global, "projects", WorkspaceKey(abs)), true, nil
}

func WorkspaceKey(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	return hex.EncodeToString(sum[:])[:16]
}

func resolveProjectRoot() {
	cwd, err := os.Getwd()
	if err != nil {
		projectRootErr = fmt.Errorf("failed to get working directory: %w", err)
		return
	}
	local := filepath.Join(cwd, ProjectConfigDirName)
	usable, reason := localUsable(local, cwd)
	if usable {
		projectRootPath = local
		return
	}
	global, err := GetConfigDir()
	if err != nil {
		projectRootErr = fmt.Errorf("failed to resolve project state directory: %w", err)
		return
	}
	fallback := filepath.Join(global, "projects", WorkspaceKey(cwd))
	projectRootPath = fallback
	projectRootFallback = true
	projectRootReason = fmt.Sprintf(
		"project state directory %s is unusable (%s); using %s instead",
		local, reason, fallback,
	)
}

func localUsable(local, parent string) (bool, string) {
	info, err := os.Stat(local)
	switch {
	case err == nil:
		if !info.IsDir() {
			return false, "path exists and is not a directory"
		}
		if err := probeWritable(local); err != nil {
			return false, fmt.Sprintf("directory is not writable: %v", err)
		}
		return true, ""
	case !os.IsNotExist(err):
		return false, fmt.Sprintf("path cannot be inspected: %v", err)
	}
	if err := probeWritable(parent); err != nil {
		return false, fmt.Sprintf("working directory is not writable: %v", err)
	}
	return true, ""
}

func probeWritable(dir string) error {
	file, err := os.CreateTemp(dir, ".pegg-probe-*")
	if err != nil {
		return err
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return nil
}

func ResetProjectRoot() {
	projectRootOnce = sync.Once{}
	projectRootPath = ""
	projectRootFallback = false
	projectRootReason = ""
	projectRootErr = nil
}
