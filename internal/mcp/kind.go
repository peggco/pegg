package mcp

import (
	"fmt"
	"strings"

	"github.com/peggco/pegg/internal/core/config"
)

type TransportKind string

const (
	TransportStdio          TransportKind = "stdio"
	TransportSSE            TransportKind = "sse"
	TransportStreamableHTTP TransportKind = "streamable-http"
)

func ParseTransportKind(value string) (TransportKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "stdio", "local":
		return TransportStdio, nil
	case "sse", "http":
		return TransportSSE, nil
	case "streamable-http", "streamable", "http-streamable":
		return TransportStreamableHTTP, nil
	default:
		return "", fmt.Errorf("%w: unknown transport type %q", ErrUnsupported, value)
	}
}

func ResolveTransportKind(cfg config.MCPServerConfig) (TransportKind, error) {
	kind, err := ParseTransportKind(cfg.Type)
	if err != nil {
		return "", err
	}
	if kind == "" {
		switch {
		case strings.TrimSpace(cfg.URL) != "":
			kind = TransportSSE
		case strings.TrimSpace(cfg.Command) != "":
			kind = TransportStdio
		default:
			return "", fmt.Errorf("%w: missing command or url", ErrUnsupported)
		}
	}
	switch kind {
	case TransportStdio:
		if strings.TrimSpace(cfg.Command) == "" {
			return "", fmt.Errorf("%w: stdio transport requires command", ErrUnsupported)
		}
	case TransportSSE, TransportStreamableHTTP:
		if strings.TrimSpace(cfg.URL) == "" {
			return "", fmt.Errorf("%w: %s transport requires url", ErrUnsupported, kind)
		}
	default:
		return "", fmt.Errorf("%w: unknown transport type %q", ErrUnsupported, kind)
	}
	return kind, nil
}

func (k TransportKind) ProtocolVersion() string {
	if k == TransportStreamableHTTP {
		return ProtocolVersionStreamable
	}
	return ProtocolVersion
}
