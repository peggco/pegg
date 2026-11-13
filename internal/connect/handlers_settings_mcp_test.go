package connect

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func newMCPTestConnector(t *testing.T, cfg *config.Config) *Connector {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c
}

func TestMCPUpsertPersistsCanonicalType(t *testing.T) {
	c := newMCPTestConnector(t, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"streamable", `{"name":"remote","type":"streamable","url":"https://example.com/mcp"}`, "streamable-http"},
		{"alias", `{"name":"legacy","type":"http","url":"https://example.com/sse"}`, "sse"},
		{"inferred", `{"name":"plain","url":"https://example.com/mcp"}`, "sse"},
		{"stdio", `{"name":"local","type":"local","command":"npx"}`, "stdio"},
	}
	for _, tc := range cases {
		if _, err := c.handleMCPUpsert(ctx, &Request{Params: []byte(tc.body)}); err != nil {
			t.Fatalf("%s: upsert: %v", tc.name, err)
		}
	}

	for name, want := range map[string]string{
		"remote": "streamable-http",
		"legacy": "sse",
		"plain":  "sse",
		"local":  "stdio",
	} {
		server, ok := c.deps.Config.MCPServers[name]
		if !ok {
			t.Fatalf("%s: server not stored", name)
		}
		if server.Type != want {
			t.Fatalf("%s: type = %q, want %q", name, server.Type, want)
		}
	}
}

func TestMCPUpsertRejectsInvalidTransport(t *testing.T) {
	c := newMCPTestConnector(t, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		body string
	}{
		{"unknown type", `{"name":"bad","type":"carrier-pigeon","command":"npx"}`},
		{"streamable without url", `{"name":"bad","type":"streamable-http","command":"npx"}`},
		{"stdio without command", `{"name":"bad","type":"stdio","url":"https://example.com/mcp"}`},
	}
	for _, tc := range cases {
		_, err := c.handleMCPUpsert(ctx, &Request{Params: []byte(tc.body)})
		if err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		var fe *FrameError
		if !asFrameErrorIs(err, &fe) || fe.Code != ErrCodeBadRequest {
			t.Fatalf("%s: expected bad request FrameError, got %v", tc.name, err)
		}
	}
}

func TestMCPListReportsResolvedType(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MCPServers = map[string]config.MCPServerConfig{
		"stdio":      {Command: "npx"},
		"inferred":   {URL: "https://example.com/sse"},
		"streamable": {Type: "streamable", URL: "https://example.com/mcp"},
		"invalid":    {Type: "carrier-pigeon", Command: "npx"},
	}
	c := newMCPTestConnector(t, cfg)

	got, err := c.handleMCPList(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	servers := got.(map[string]any)["servers"].([]map[string]any)
	types := make(map[string]any, len(servers))
	for _, server := range servers {
		types[server["name"].(string)] = server["type"]
	}

	for name, want := range map[string]string{
		"stdio":      "stdio",
		"inferred":   "sse",
		"streamable": "streamable-http",
		"invalid":    "carrier-pigeon",
	} {
		if types[name] != want {
			t.Fatalf("%s: type = %v, want %q", name, types[name], want)
		}
	}
}
