package server

import (
	"fmt"
	"net"
	"strings"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/logger"
)

// IsLoopbackHost reports whether host binds only to the local machine.
// An empty host is not considered loopback because net.Listen treats it as
// all interfaces.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ValidateHTTPServerConfig refuses an unauthenticated HTTP API bind on a
// non-loopback address unless the operator explicitly opts in.
func ValidateHTTPServerConfig(sc config.ServerConfig) error {
	if IsLoopbackHost(sc.Host) || len(sc.RequiredHeaders) > 0 || sc.AllowUnauthenticated {
		return nil
	}
	return fmt.Errorf(
		"refusing to bind HTTP API to non-loopback address %s without authentication: "+
			"configure server.required_headers, or pass --insecure (server.allow_unauthenticated = true)",
		bindLabel(sc.Host),
	)
}

// ValidateACPServerConfig refuses an unauthenticated ACP-over-HTTP bind on a
// non-loopback address. Unlike the HTTP API, ACP does not enforce
// required_headers, so only loopback or an explicit opt-in is accepted.
func ValidateACPServerConfig(sc config.ServerConfig) error {
	if IsLoopbackHost(sc.Host) || sc.AllowUnauthenticated {
		return nil
	}
	return fmt.Errorf(
		"refusing to bind ACP server to non-loopback address %s without authentication: "+
			"ACP-over-HTTP does not enforce server.required_headers; pass --insecure "+
			"(server.allow_unauthenticated = true) to accept the risk",
		bindLabel(sc.Host),
	)
}

// WarnInsecureBind logs a warning when a non-loopback bind was explicitly
// allowed without authentication.
func WarnInsecureBind(log *logger.Logger, sc config.ServerConfig, surface string) {
	if log == nil || IsLoopbackHost(sc.Host) || !sc.AllowUnauthenticated {
		return
	}
	log.Fwarn(
		"%s is listening on non-loopback address %s with unauthenticated access enabled "+
			"(--insecure / server.allow_unauthenticated); anyone who can reach it can run the agent",
		surface, bindLabel(sc.Host),
	)
}

func bindLabel(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "(all interfaces)"
	}
	return fmt.Sprintf("%q", host)
}
