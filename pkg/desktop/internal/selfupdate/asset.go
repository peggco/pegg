package selfupdate

import (
	"fmt"
	"regexp"
)

func AssetFilter(goos, goarch string) string {
	ext := `\.tar\.gz`
	if goos == "windows" || goos == "darwin" {
		ext = `\.zip`
	}
	return fmt.Sprintf(`(?i)^pegg-desktop_.*_%s_%s%s$`, regexp.QuoteMeta(goos), regexp.QuoteMeta(goarch), ext)
}

func BinaryName(goos string) string {
	if goos == "windows" {
		return "pegg-desktop.exe"
	}
	return "pegg-desktop"
}

func ChecksumAssetName() string {
	return "pegg-desktop_checksums.txt"
}

func AgentAssetName(goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("pegg_%s_%s.%s", goos, goarch, ext)
}

func SupportedTarget(goos, dir string) bool {
	return supportedTarget(goos, dir)
}
