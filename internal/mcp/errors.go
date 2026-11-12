package mcp

import "errors"

var (
	ErrNotInitialized = errors.New("mcp: client not initialized")
	ErrTimeout        = errors.New("mcp: request timed out")
	ErrServerError    = errors.New("mcp: server returned error")
	ErrUnsupported    = errors.New("mcp: unsupported transport")
	ErrClosed         = errors.New("mcp: transport closed")
	ErrProtocol       = errors.New("mcp: protocol error")
	ErrSessionExpired = errors.New("mcp: session expired")
)
