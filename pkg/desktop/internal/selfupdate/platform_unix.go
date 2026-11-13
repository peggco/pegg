//go:build !windows

package selfupdate

import (
	"os"
	"os/exec"

	"github.com/peggco/pegg/pkg/desktop/internal/binary"
)

func supportedTarget(goos, dir string) bool {
	if goos == "darwin" {
		return false
	}
	return writableDir(dir)
}

func writableDir(dir string) bool {
	if dir == "" {
		return false
	}
	file, err := os.CreateTemp(dir, ".pegg-write-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func apply(target, staged string) error {
	tmp := target + ".new"
	if err := binary.CopyFile(staged, tmp); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func Relaunch(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Dir, _ = os.Getwd()
	return cmd.Start()
}
