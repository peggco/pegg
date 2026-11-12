package install

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRelease struct {
	tag      string
	asset    string
	data     []byte
	checksum string
	digest   string
}

func newReleaseServer(t *testing.T, state *fakeRelease) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"), strings.Contains(r.URL.Path, "/releases/tags/"):
			digest := ""
			if state.digest != "" {
				digest = `,"digest":"sha256:` + state.digest + `"`
			}
			assets := fmt.Sprintf(`{"name":%q,"browser_download_url":%q,"size":%d%s}`, state.asset, server.URL+"/asset", len(state.data), digest)
			if state.checksum != "" {
				assets += fmt.Sprintf(`,{"name":"checksums.txt","browser_download_url":%q,"size":100}`, server.URL+"/checksums.txt")
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, state.tag, assets)
		case r.URL.Path == "/asset":
			_, _ = w.Write(state.data)
		case r.URL.Path == "/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", state.checksum, state.asset)
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func testClient(server *httptest.Server) *Client {
	return &Client{APIBase: server.URL, HTTP: server.Client(), MaxAPIBytes: 1 << 20, MaxAssetBytes: 1 << 20, UserAgent: "test"}
}

func TestInstallUpdateRemove(t *testing.T) {
	state := &fakeRelease{tag: "v1.0.0", asset: "plugin_linux_amd64", data: []byte("v1")}
	state.checksum = SHA256Hex(state.data)
	server := newReleaseServer(t, state)
	defer server.Close()
	dir := t.TempDir()
	var out bytes.Buffer
	opts := Options{Source: Source{Owner: "owner", Repo: "repo"}, Dir: dir, GOOS: "linux", GOARCH: "amd64", Name: "plugin", Out: &out}
	result, err := Install(context.Background(), testClient(server), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "plugin" || result.Version != "1.0.0" || result.Source != "owner/repo" {
		t.Fatalf("result: %#v", result)
	}
	info, err := os.Stat(filepath.Join(dir, "plugin"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("installed file: %v %v", info, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "plugin"))
	if string(data) != "v1" {
		t.Fatalf("content: %q", data)
	}
	list, err := List(dir)
	if err != nil || len(list) != 1 || list[0].Manual || list[0].Version != "1.0.0" {
		t.Fatalf("list: %#v %v", list, err)
	}

	state.tag = "v1.1.0"
	state.data = []byte("v2")
	state.checksum = SHA256Hex(state.data)
	updated, err := Update(context.Background(), testClient(server), Options{Source: Source{Owner: "owner", Repo: "repo"}, Dir: dir, GOOS: "linux", GOARCH: "amd64", Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Updated || updated.PreviousVersion != "1.0.0" || updated.Version != "1.1.0" {
		t.Fatalf("updated: %#v", updated)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "plugin"))
	if string(data) != "v2" {
		t.Fatalf("updated content: %q", data)
	}

	state.tag = "v1.2.0"
	checked, err := Update(context.Background(), testClient(server), Options{Source: Source{Owner: "owner", Repo: "repo"}, Dir: dir, GOOS: "linux", GOARCH: "amd64", Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if !checked.UpdateAvailable || checked.Updated {
		t.Fatalf("check: %#v", checked)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "plugin"))
	if string(data) != "v2" {
		t.Fatalf("check changed binary: %q", data)
	}

	if err := Remove(dir, "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugin")); !os.IsNotExist(err) {
		t.Fatalf("binary still exists: %v", err)
	}
}

func TestInstallChecksumModes(t *testing.T) {
	state := &fakeRelease{tag: "v1.0.0", asset: "plugin_linux_amd64", data: []byte("binary"), checksum: strings.Repeat("0", 64)}
	server := newReleaseServer(t, state)
	defer server.Close()
	opts := Options{Source: Source{Owner: "o", Repo: "r"}, Dir: t.TempDir(), GOOS: "linux", GOARCH: "amd64", Name: "plugin"}
	if _, err := Install(context.Background(), testClient(server), opts); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	state.checksum = ""
	opts.RequireChecksum = true
	if _, err := Install(context.Background(), testClient(server), opts); err == nil {
		t.Fatal("expected required checksum error")
	}
	opts.RequireChecksum = false
	opts.SkipChecksum = true
	if _, err := Install(context.Background(), testClient(server), opts); err != nil {
		t.Fatalf("skip checksum: %v", err)
	}
	state.digest = SHA256Hex(state.data)
	opts.SkipChecksum = false
	opts.RequireChecksum = true
	opts.Force = true
	if _, err := Install(context.Background(), testClient(server), opts); err != nil {
		t.Fatalf("digest checksum: %v", err)
	}
}

func TestInstallArchiveAndSourceConflict(t *testing.T) {
	data := tarGz(t, map[string]struct {
		body []byte
		mode int64
	}{"plugin": {body: []byte("archived"), mode: 0o755}})
	state := &fakeRelease{tag: "v1.0.0", asset: "plugin_linux_amd64.tar.gz", data: data}
	state.checksum = SHA256Hex(data)
	server := newReleaseServer(t, state)
	defer server.Close()
	dir := t.TempDir()
	opts := Options{Source: Source{Owner: "owner", Repo: "repo"}, Dir: dir, GOOS: "linux", GOARCH: "amd64", Name: "plugin"}
	if _, err := Install(context.Background(), testClient(server), opts); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "plugin"))
	if string(content) != "archived" {
		t.Fatalf("archive content: %q", content)
	}
	opts.Source = Source{Owner: "other", Repo: "repo"}
	if _, err := Install(context.Background(), testClient(server), opts); err == nil {
		t.Fatal("expected source conflict")
	}
	opts.Force = true
	if _, err := Install(context.Background(), testClient(server), opts); err != nil {
		t.Fatalf("force replace: %v", err)
	}
}

func TestResolveRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	if _, err := Resolve(dir, "../escape"); err == nil {
		t.Fatal("expected traversal error")
	}
	if _, err := Resolve(dir, "plugin.sh"); err == nil {
		t.Fatal("expected extension error")
	}
}

func TestVersionNewer(t *testing.T) {
	if !VersionNewer("1.0.0", "v1.0.1") || VersionNewer("1.0.0", "v1.0.0") || !VersionNewer("", "v1") {
		t.Fatal("semver comparison failed")
	}
}
