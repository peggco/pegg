package update

import (
	"regexp"
	"testing"
)

func TestAgentAssetsIgnoreDesktop(t *testing.T) {
	if len(agentAssets) == 0 {
		t.Fatal("no agent asset filters")
	}
	filter := regexp.MustCompile(agentAssets[0])
	for _, name := range []string{
		"pegg_linux_amd64.tar.gz",
		"pegg_darwin_arm64.tar.gz",
		"pegg_windows_amd64.zip",
		"pegg_windows_arm64.zip",
	} {
		if !filter.MatchString(name) {
			t.Errorf("agent filter did not match %q", name)
		}
	}
	for _, name := range []string{
		"pegg-desktop_0.2.5_linux_amd64.tar.gz",
		"pegg-desktop_0.2.5_darwin_arm64.zip",
		"pegg-desktop_0.2.5_windows_amd64.zip",
		"checksums.txt",
	} {
		if filter.MatchString(name) {
			t.Errorf("agent filter matched %q", name)
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
