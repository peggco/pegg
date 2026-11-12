package mcp

import (
	"errors"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestParseTransportKind(t *testing.T) {
	cases := []struct {
		input string
		want  TransportKind
	}{
		{"", ""},
		{"stdio", TransportStdio},
		{"LOCAL", TransportStdio},
		{"sse", TransportSSE},
		{"http", TransportSSE},
		{"streamable-http", TransportStreamableHTTP},
		{"streamable", TransportStreamableHTTP},
		{"HTTP-Streamable", TransportStreamableHTTP},
	}
	for _, tc := range cases {
		got, err := ParseTransportKind(tc.input)
		if err != nil {
			t.Fatalf("ParseTransportKind(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParseTransportKind(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	if _, err := ParseTransportKind("websocket"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unknown kind error = %v, want ErrUnsupported", err)
	}
}

func TestResolveTransportKind(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.MCPServerConfig
		want TransportKind
	}{
		{"explicit stdio", config.MCPServerConfig{Type: "stdio", Command: "npx"}, TransportStdio},
		{"explicit sse", config.MCPServerConfig{Type: "sse", URL: "https://example.com"}, TransportSSE},
		{"explicit streamable", config.MCPServerConfig{Type: "streamable-http", URL: "https://example.com"}, TransportStreamableHTTP},
		{"infer url first", config.MCPServerConfig{URL: "https://example.com", Command: "npx"}, TransportSSE},
		{"infer command", config.MCPServerConfig{Command: "npx"}, TransportStdio},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTransportKind(tc.cfg)
			if err != nil {
				t.Fatalf("ResolveTransportKind: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	invalid := []config.MCPServerConfig{
		{},
		{Type: "stdio", URL: "https://example.com"},
		{Type: "sse"},
		{Type: "streamable-http"},
		{Type: "websocket", URL: "https://example.com"},
	}
	for _, cfg := range invalid {
		if _, err := ResolveTransportKind(cfg); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("ResolveTransportKind(%+v) error = %v, want ErrUnsupported", cfg, err)
		}
	}
}

func TestTransportKindProtocolVersion(t *testing.T) {
	if got := TransportStreamableHTTP.ProtocolVersion(); got != ProtocolVersionStreamable {
		t.Fatalf("streamable protocol = %q, want %q", got, ProtocolVersionStreamable)
	}
	for _, kind := range []TransportKind{TransportStdio, TransportSSE} {
		if got := kind.ProtocolVersion(); got != ProtocolVersion {
			t.Fatalf("%s protocol = %q, want %q", kind, got, ProtocolVersion)
		}
	}
}
