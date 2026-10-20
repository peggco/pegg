package sdk

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pegg-sdk-home")
	if err != nil {
		fmt.Fprintln(os.Stderr, "sdk tests: create temp home:", err)
		os.Exit(1)
	}
	if err := os.Setenv("HOME", dir); err != nil {
		fmt.Fprintln(os.Stderr, "sdk tests: sandbox home:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
