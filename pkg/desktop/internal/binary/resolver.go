package binary

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Status struct {
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"`
	Managed bool   `json:"managed,omitempty"`
	Error   string `json:"error,omitempty"`
}

func Name() string {
	if runtime.GOOS == "windows" {
		return "pegg.exe"
	}
	return "pegg"
}

func ManagedDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pegg", "bin")
}

func SearchDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		add(dir)
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		add(filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "pegg"))
		add(filepath.Join(home, "scoop", "shims"))
	} else {
		add(filepath.Join(home, ".pegg", "bin"))
		add(filepath.Join(home, ".local", "bin"))
		add(filepath.Join(home, "bin"))
		add("/usr/local/bin")
		add("/opt/homebrew/bin")
	}
	return dirs
}

func Candidates(custom string, extra ...string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	add(custom)
	for _, dir := range extra {
		add(filepath.Join(dir, "bin", Name()))
		add(filepath.Join(dir, Name()))
	}
	if p, err := exec.LookPath(Name()); err == nil {
		add(p)
	}
	for _, dir := range SearchDirs() {
		add(filepath.Join(dir, Name()))
	}
	return out
}

func Probe(ctx context.Context, path string) Status {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Status{Path: path, Error: "not found"}
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, path, "version")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Status{Path: path, Error: strings.TrimSpace(string(out)) + " " + err.Error()}
	}
	version := parseVersion(string(out))
	if version == "" {
		return Status{Path: path, Error: "unexpected version output"}
	}
	return Status{Found: true, Path: path, Version: version}
}

func Resolve(ctx context.Context, custom string, extra ...string) Status {
	var firstErr string
	for _, path := range Candidates(custom, extra...) {
		st := Probe(ctx, path)
		if st.Found {
			if custom != "" && path == custom {
				st.Source = "custom"
			} else if strings.HasPrefix(path, ManagedDir()) {
				st.Source = "managed"
				st.Managed = true
			} else {
				st.Source = "installed"
			}
			return st
		}
		if firstErr == "" && st.Path != "" && st.Error != "not found" {
			firstErr = st.Path + ": " + st.Error
		}
	}
	return Status{Error: firstErrOr(firstErr, "pegg binary not found")}
}

func firstErrOr(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func parseVersion(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "version" && i+1 < len(fields) {
				return strings.TrimSpace(fields[i+1])
			}
		}
		if strings.HasPrefix(line, "v") || strings.Contains(line, ".") {
			return line
		}
	}
	return ""
}
