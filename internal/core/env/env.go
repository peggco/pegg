package env

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const shellEnvTimeout = 2 * time.Second

var importedKeys = []string{
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"GH_HOST",
	"GH_ENTERPRISE_TOKEN",
	"GITLAB_TOKEN",
	"PEGG_GITHUB_TOKEN",
	"PEGG_GITHUB_REPO",
	"PEGG_GITHUB_OWNER",
	"PEGG_GITHUB_REPO_NAME",
	"PEGG_GITHUB_API_URL",
	"PEGG_GITHUB_WORKTREE_ROOT",
	"PEGG_GITHUB_DEFAULT_BASE",
	"PEGG_GITHUB_TIMEOUT_SECONDS",
	"PEGG_JIRA_TOKEN",
	"PEGG_BITBUCKET_API_TOKEN",
	"PEGG_BITBUCKET_ACCESS_TOKEN",
	"PEGG_BITBUCKET_TOKEN",
}

var shellEnvFunc = readLoginShellEnv

var (
	cacheMu   sync.Mutex
	cacheOnce bool
	cacheEnv  map[string]string
)

func Apply() []string {
	base := os.Environ()
	merged := Augment(base)

	baseValues := parseEnv(base)
	mergedValues := parseEnv(merged)

	changed := make([]string, 0, len(mergedValues))
	for key, value := range mergedValues {
		if baseValues[key] == value {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			continue
		}
		changed = append(changed, key)
	}
	sort.Strings(changed)
	return changed
}

func Augment(base []string) []string {
	shell := loginEnv()
	if len(shell) == 0 {
		return base
	}

	values := parseEnv(base)
	order := envOrder(base)

	if extraPath := strings.TrimSpace(shell["PATH"]); extraPath != "" {
		values["PATH"] = mergePath(values["PATH"], extraPath)
	}
	for _, key := range importedKeys {
		if values[key] != "" {
			continue
		}
		if value := shell[key]; value != "" {
			values[key] = value
		}
	}

	if len(order) == 0 {
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		order = keys
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(order))
	for _, key := range order {
		if seen[key] {
			continue
		}
		value, ok := values[key]
		if !ok {
			continue
		}
		seen[key] = true
		out = append(out, key+"="+value)
	}
	for _, key := range importedKeys {
		if seen[key] {
			continue
		}
		if value := values[key]; value != "" {
			seen[key] = true
			out = append(out, key+"="+value)
		}
	}
	return out
}

func ResetCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheOnce = false
	cacheEnv = nil
}

func loginEnv() map[string]string {
	if !enabled() {
		return nil
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheOnce {
		return cacheEnv
	}
	cacheOnce = true

	ctx, cancel := context.WithTimeout(context.Background(), shellEnvTimeout)
	defer cancel()
	env, err := shellEnvFunc(ctx)
	if err != nil {
		cacheEnv = nil
		return nil
	}
	cacheEnv = env
	return cacheEnv
}

func enabled() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if strings.TrimSpace(os.Getenv("PEGG_SHELL_ENV")) == "0" {
		return false
	}
	return strings.TrimSpace(os.Getenv("SHELL")) != ""
}

func readLoginShellEnv(ctx context.Context) (map[string]string, error) {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return nil, exec.ErrNotFound
	}
	cmd := exec.CommandContext(ctx, shell, "-ilc", "env -0")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseShellOutput(out), nil
}

func parseShellOutput(out []byte) map[string]string {
	values := make(map[string]string)
	chunk := make([]byte, 0, 128)
	flush := func() {
		if len(chunk) == 0 {
			return
		}
		entry := string(chunk)
		chunk = chunk[:0]
		if idx := strings.LastIndexByte(entry, '\n'); idx >= 0 {
			entry = entry[idx+1:]
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !validKey(key) {
			return
		}
		values[key] = value
	}
	for _, b := range out {
		if b == 0 {
			flush()
			continue
		}
		chunk = append(chunk, b)
	}
	flush()
	return values
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func parseEnv(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	return values
}

func envOrder(env []string) []string {
	order := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			order = append(order, key)
		}
	}
	return order
}

func mergePath(base, extra string) string {
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	seen := make(map[string]bool)
	dirs := make([]string, 0, len(strings.Split(base, sep))+len(strings.Split(extra, sep)))
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		if seen[clean] {
			return
		}
		seen[clean] = true
		dirs = append(dirs, dir)
	}
	for _, dir := range strings.Split(base, sep) {
		add(dir)
	}
	for _, dir := range strings.Split(extra, sep) {
		add(dir)
	}
	return strings.Join(dirs, sep)
}
