package install

import "testing"

func TestSelectAsset(t *testing.T) {
	release := &Release{TagName: "v1.0.0", Assets: []Asset{
		{Name: "plugin_linux_amd64", URL: "u1"},
		{Name: "plugin_darwin_arm64", URL: "u2"},
		{Name: "checksums.txt", URL: "u3"},
		{Name: "plugin_linux_amd64.tar.gz", URL: "u4"},
	}}
	asset, err := SelectAsset(release, SelectOptions{GOOS: "linux", GOARCH: "amd64", Repo: "plugin"})
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "plugin_linux_amd64" {
		t.Fatalf("unexpected asset %q", asset.Name)
	}
	asset, err = SelectAsset(release, SelectOptions{Pattern: "*.tar.gz", GOOS: "linux", GOARCH: "amd64", Repo: "plugin"})
	if err != nil || asset.Name != "plugin_linux_amd64.tar.gz" {
		t.Fatalf("pattern: %#v %v", asset, err)
	}
	asset, err = SelectAsset(&Release{Assets: []Asset{{Name: "only-binary", URL: "u"}}}, SelectOptions{GOOS: "linux", GOARCH: "amd64"})
	if err != nil || asset.Name != "only-binary" {
		t.Fatalf("single fallback: %#v %v", asset, err)
	}
	if _, err := SelectAsset(&Release{Assets: []Asset{{Name: "alpha", URL: "a"}, {Name: "beta", URL: "b"}}}, SelectOptions{GOOS: "linux", GOARCH: "arm64"}); err == nil {
		t.Fatal("expected ambiguity error")
	}
}
