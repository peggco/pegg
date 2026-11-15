package update

import (
	"regexp"
	"testing"
)

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

func TestAgentAssetsForPlatform(t *testing.T) {
	tests := []struct {
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
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			filters := agentAssetsFor(tt.goos, tt.goarch)
			if len(filters) != 1 {
				t.Fatalf("got %d filters, want 1", len(filters))
			}
			filter := regexp.MustCompile(filters[0])
			matches := 0
			for _, name := range releaseAssets {
				got := filter.MatchString(name)
				want := name == tt.want
				if got != want {
					t.Errorf("match(%q) = %v, want %v", name, got, want)
				}
				if got {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("filter matched %d assets, want exactly 1", matches)
			}
		})
	}
}

func TestAgentAssetsRejectDesktopArchives(t *testing.T) {
	desktop := []string{
		"pegg-desktop_0.2.4_darwin_amd64.zip",
		"pegg-desktop_0.2.4_darwin_arm64.zip",
		"pegg-desktop_0.2.4_linux_amd64.tar.gz",
		"pegg-desktop_0.2.4_windows_amd64.zip",
		"pegg-desktop_0.2.4_windows_arm64.zip",
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			filter := regexp.MustCompile(agentAssetsFor(goos, goarch)[0])
			for _, name := range desktop {
				if filter.MatchString(name) {
					t.Errorf("%s/%s filter matched desktop archive %q", goos, goarch, name)
				}
			}
		}
	}
}

func TestAgentAssetsUnknownPlatform(t *testing.T) {
	filter := regexp.MustCompile(agentAssetsFor("freebsd", "amd64")[0])
	for _, name := range releaseAssets {
		if filter.MatchString(name) {
			t.Errorf("freebsd filter matched %q", name)
		}
	}
}

func TestIsNewerThan(t *testing.T) {
	if !IsNewerThan("0.2.4", "0.2.5") {
		t.Fatal("0.2.5 should be newer than 0.2.4")
	}
	if IsNewerThan("0.2.5", "0.2.4") {
		t.Fatal("0.2.4 should not be newer than 0.2.5")
	}
	if IsNewerThan("0.2.5", "0.2.5") {
		t.Fatal("equal versions should not be newer")
	}
	if IsNewerThan("not-a-version", "0.2.5") {
		t.Fatal("invalid current version should not be newer")
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
		got := AgentAssetName(tc.goos, tc.goarch)
		if got != tc.want {
			t.Errorf("AgentAssetName(%q, %q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
		filter := regexp.MustCompile(agentAssetsFor(tc.goos, tc.goarch)[0])
		if !filter.MatchString(got) {
			t.Errorf("%s/%s filter does not match its own asset name %q", tc.goos, tc.goarch, got)
		}
	}
}
