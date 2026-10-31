package binary

import "testing"

func TestParseVersion(t *testing.T) {
	cases := map[string]string{
		"pegg version 0.2.2\n": "0.2.2",
		"0.2.2\n":              "0.2.2",
		"":                     "",
	}
	for input, want := range cases {
		if got := parseVersion(input); got != want {
			t.Errorf("parseVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAssetName(t *testing.T) {
	d := &Downloader{GOOS: "linux", GOARCH: "arm64"}
	name, ext := d.assetName()
	if name != "pegg_linux_arm64.tar.gz" || ext != "tar.gz" {
		t.Fatalf("asset = %q %q", name, ext)
	}
	d = &Downloader{GOOS: "windows", GOARCH: "amd64"}
	name, ext = d.assetName()
	if name != "pegg_windows_amd64.zip" || ext != "zip" {
		t.Fatalf("asset = %q %q", name, ext)
	}
}
