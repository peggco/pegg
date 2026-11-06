package connect

import (
	"errors"
	"path/filepath"
)

var ErrAlreadyRunning = errors.New("connect: another `pegg connect` is already running for this device")

const lockFileName = "connect.lock"

func lockPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, lockFileName), nil
}
