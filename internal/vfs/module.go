package vfs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

func VFSModule(log *logger.Logger) (*VFS, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("bootstrap: get working directory: %w", err)
	}
	opts := Options{Log: log}
	if path, err := DefaultSnapshotPath(cwd); err != nil {
		if log != nil {
			log.Fwarn("vfs: resolve snapshot store path: %v", err)
		}
	} else {
		opts.SnapshotPath = path
	}
	if root, err := config.ProjectRoot(); err == nil {
		local := filepath.Join(filepath.Clean(cwd), config.ProjectConfigDirName)
		if filepath.Clean(root) != local {
			opts.ProjectRoot = root
		}
	} else if log != nil {
		log.Fwarn("vfs: resolve project state directory: %v", err)
	}
	return New(cwd, opts)
}
