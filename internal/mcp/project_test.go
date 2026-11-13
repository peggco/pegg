package mcp

import (
	"reflect"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestProjectServerTypeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := map[string]config.MCPServerConfig{
		"remote": {
			Type:    "streamable-http",
			URL:     "https://example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer token"},
		},
		"local": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "server"},
			Env:     map[string]string{"DEBUG": "true"},
		},
	}
	if err := SaveProjectServers(dir, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadProjectServers(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}
