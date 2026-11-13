package selfupdate

import (
	"regexp"
	"testing"
)

func TestAssetFilter(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		match  string
		reject []string
	}{
		{
			goos:   "linux",
			goarch: "amd64",
			match:  "pegg-desktop_0.2.5_linux_amd64.tar.gz",
			reject: []string{
				"pegg_linux_amd64.tar.gz",
				"pegg-desktop_0.2.5_linux_amd64.deb",
				"pegg-desktop-0.2.5.amd64.rpm",
				"pegg-desktop_0.2.5_windows_amd64.zip",
			},
		},
		{
			goos:   "windows",
			goarch: "amd64",
			match:  "pegg-desktop_0.2.5_windows_amd64.zip",
			reject: []string{
				"pegg_windows_amd64.zip",
				"pegg-desktop_0.2.5_linux_amd64.tar.gz",
			},
		},
		{
			goos:   "darwin",
			goarch: "arm64",
			match:  "pegg-desktop_0.2.5_darwin_arm64.zip",
			reject: []string{
				"pegg_darwin_arm64.tar.gz",
				"pegg-desktop_0.2.5_darwin_amd64.zip",
			},
		},
	}
	for _, tc := range cases {
		re := regexp.MustCompile(AssetFilter(tc.goos, tc.goarch))
		if !re.MatchString(tc.match) {
			t.Errorf("%s/%s filter did not match %q", tc.goos, tc.goarch, tc.match)
		}
		for _, name := range tc.reject {
			if re.MatchString(name) {
				t.Errorf("%s/%s filter matched %q", tc.goos, tc.goarch, name)
			}
		}
	}
}

func TestBinaryName(t *testing.T) {
	if got := BinaryName("windows"); got != "pegg-desktop.exe" {
		t.Fatalf("windows = %q", got)
	}
	if got := BinaryName("linux"); got != "pegg-desktop" {
		t.Fatalf("linux = %q", got)
	}
}

func TestChecksumAssetName(t *testing.T) {
	if got := ChecksumAssetName(); got != "pegg-desktop_checksums.txt" {
		t.Fatalf("checksum asset = %q", got)
	}
}
