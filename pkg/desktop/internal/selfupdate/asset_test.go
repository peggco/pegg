package selfupdate

import (
	"fmt"
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

var releaseAssets = []string{
	"checksums.txt",
	"nfpm.yaml",
	"pegg-desktop-0.2.4.amd64.rpm",
	"pegg-desktop_0.2.4_amd64.deb",
	"pegg-desktop_0.2.4_darwin_amd64.zip",
	"pegg-desktop_0.2.4_darwin_arm64.zip",
	"pegg-desktop_0.2.4_linux_amd64.tar.gz",
	"pegg-desktop_0.2.4_windows_amd64.zip",
	"pegg-desktop_0.2.4_windows_arm64.zip",
	"pegg-desktop_checksums.txt",
	"pegg-intellij-0.2.4-signed.zip",
	"pegg-intellij-0.2.4.zip",
	"pegg-vscode-0.2.4.vsix",
	"pegg_darwin_amd64.tar.gz",
	"pegg_darwin_arm64.tar.gz",
	"pegg_linux_amd64.tar.gz",
	"pegg_linux_arm64.tar.gz",
	"pegg_windows_amd64.zip",
	"pegg_windows_arm64.zip",
}

func TestAssetFilterMatchesExactlyOne(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			if goos == "linux" && goarch == "arm64" {
				continue
			}
			ext := "tar.gz"
			if goos == "windows" || goos == "darwin" {
				ext = "zip"
			}
			want := fmt.Sprintf("pegg-desktop_0.2.4_%s_%s.%s", goos, goarch, ext)
			filter := regexp.MustCompile(AssetFilter(goos, goarch))
			matches := 0
			for _, name := range releaseAssets {
				if !filter.MatchString(name) {
					continue
				}
				matches++
				if name != want {
					t.Errorf("%s/%s filter matched %q, want %q", goos, goarch, name, want)
				}
			}
			if matches != 1 {
				t.Errorf("%s/%s filter matched %d assets, want exactly 1", goos, goarch, matches)
			}
			if filter.MatchString(AgentAssetName(goos, goarch)) {
				t.Errorf("%s/%s filter matched the agent archive %q", goos, goarch, AgentAssetName(goos, goarch))
			}
		}
	}
}

func TestAgentAssetName(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"linux", "amd64", "pegg_linux_amd64.tar.gz"},
		{"linux", "arm64", "pegg_linux_arm64.tar.gz"},
		{"darwin", "amd64", "pegg_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "pegg_darwin_arm64.tar.gz"},
		{"windows", "amd64", "pegg_windows_amd64.zip"},
		{"windows", "arm64", "pegg_windows_arm64.zip"},
	}
	for _, tc := range cases {
		if got := AgentAssetName(tc.goos, tc.goarch); got != tc.want {
			t.Errorf("AgentAssetName(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}
