package vfs

import (
	"fmt"
	"os"

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
	return New(cwd, opts)
}
