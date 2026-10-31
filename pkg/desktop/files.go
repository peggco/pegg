//go:build gui

package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxIndexedFiles = 40000

var (
	fileIndexMu   sync.Mutex
	fileIndexRoot string
	fileIndex     []string
	fileIndexAt   time.Time
)

var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".pegg": true, ".idea": true, ".vscode": true,
	"node_modules": true, "vendor": true, "bin": true, "dist": true, "build": true, "out": true,
	"target": true, "Pods": true, ".venv": true, "venv": true, "__pycache__": true,
	".next": true, ".nuxt": true, "coverage": true, ".cache": true, ".turbo": true,
}

func (a *App) SearchFiles(query string, limit int) ([]string, error) {
	root := a.currentWorkspace()
	if root == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	files := indexFiles(root)
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		if len(files) > limit {
			files = files[:limit]
		}
		return files, nil
	}
	type scored struct {
		path  string
		score int
	}
	results := make([]scored, 0, limit*2)
	for _, path := range files {
		lower := strings.ToLower(path)
		base := strings.ToLower(filepath.Base(path))
		score := 0
		switch {
		case base == q:
			score = 100
		case strings.HasPrefix(base, q):
			score = 80
		case strings.Contains(base, q):
			score = 60
		case strings.HasPrefix(lower, q):
			score = 45
		case strings.Contains(lower, q):
			score = 30
		}
		if score > 0 {
			results = append(results, scored{path: path, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return len(results[i].path) < len(results[j].path)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	out := make([]string, len(results))
	for i, item := range results {
		out[i] = item.path
	}
	return out, nil
}

func indexFiles(root string) []string {
	fileIndexMu.Lock()
	if fileIndexRoot == root && time.Since(fileIndexAt) < 20*time.Second {
		files := fileIndex
		fileIndexMu.Unlock()
		return files
	}
	fileIndexMu.Unlock()

	files := make([]string, 0, 4096)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(files) >= maxIndexedFiles {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})

	fileIndexMu.Lock()
	fileIndexRoot = root
	fileIndex = files
	fileIndexAt = time.Now()
	fileIndexMu.Unlock()
	return files
}

func resetFileIndex() {
	fileIndexMu.Lock()
	fileIndexRoot = ""
	fileIndex = nil
	fileIndexAt = time.Time{}
	fileIndexMu.Unlock()
}
