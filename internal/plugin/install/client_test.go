package install

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientLatestAndAuth(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[{"name":"plugin_linux_amd64","browser_download_url":"`+serverURL(r)+`/asset","size":4}]}`)
	}))
	defer server.Close()
	client := &Client{APIBase: server.URL, HTTP: server.Client(), Token: "secret", UserAgent: "test", MaxAPIBytes: 1 << 20, MaxAssetBytes: 1 << 20}
	release, err := client.Latest(context.Background(), Source{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.2.3" || len(release.Assets) != 1 || release.Assets[0].Name != "plugin_linux_amd64" {
		t.Fatalf("release: %#v", release)
	}
	if seenAuth != "Bearer secret" {
		t.Fatalf("auth header: %q", seenAuth)
	}
}

func TestClientErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"rate limited"}`)
	}))
	defer server.Close()
	client := &Client{APIBase: server.URL, HTTP: server.Client()}
	if _, err := client.Latest(context.Background(), Source{Owner: "owner", Repo: "missing"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("404 error: %v", err)
	}
	if _, err := client.Latest(context.Background(), Source{Owner: "owner", Repo: "repo"}); err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("403 error: %v", err)
	}
}

func TestDownloadLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0123456789")
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client()}
	if _, err := client.Download(context.Background(), server.URL, 5); err == nil {
		t.Fatal("expected size limit error")
	}
	data, err := client.Download(context.Background(), server.URL, 20)
	if err != nil || string(data) != "0123456789" {
		t.Fatalf("download: %q %v", data, err)
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
