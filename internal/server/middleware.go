package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		if s.log != nil {
			s.log.Fdebug("http: %s %s %d %s", r.Method, r.URL.Path, rw.statusCode, time.Since(start))
		}
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	origins := normalizeOrigins(s.cfg.Server.CORSOrigins)
	wildcard := false
	for _, origin := range origins {
		if origin == "*" {
			wildcard = true
			break
		}
	}
	corsEnabled := len(origins) > 0

	userHeaders := make(map[string]string, len(s.cfg.Server.Headers))
	for k, v := range s.cfg.Server.Headers {
		if isCORSManagedHeader(k) {
			continue
		}
		userHeaders[k] = v
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if corsEnabled && !wildcard {
			w.Header().Add("Vary", "Origin")
		}

		allowed := false
		switch {
		case wildcard:
			w.Header().Set("Access-Control-Allow-Origin", "*")
			allowed = true
		case corsEnabled:
			if origin := r.Header.Get("Origin"); origin != "" && originAllowed(origin, origins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				allowed = true
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", s.allowedRequestHeaders())
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		for k, v := range userHeaders {
			w.Header().Set(k, v)
		}

		if allowed && isPreflight(r) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func normalizeOrigins(origins []string) []string {
	normalized := make([]string, 0, len(origins))
	seen := make(map[string]bool, len(origins))
	for _, origin := range origins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" || seen[origin] {
			continue
		}
		seen[origin] = true
		normalized = append(normalized, origin)
	}
	return normalized
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}

func isCORSManagedHeader(name string) bool {
	lower := strings.ToLower(name)
	return lower == "vary" || strings.HasPrefix(lower, "access-control-")
}

func (s *Server) allowedRequestHeaders() string {
	headers := []string{"Content-Type"}
	seen := map[string]bool{"content-type": true}

	names := make([]string, 0, len(s.cfg.Server.RequiredHeaders))
	for name := range s.cfg.Server.RequiredHeaders {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		headers = append(headers, http.CanonicalHeaderKey(name))
	}

	return strings.Join(headers, ", ")
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == "*" || strings.EqualFold(origin, candidate) {
			return true
		}
	}
	return false
}

func headerValueMatches(key, got, expected string) bool {
	if expected == "" {
		return true
	}
	if strings.EqualFold(key, "Authorization") {
		gotScheme, gotToken, gotOK := strings.Cut(got, " ")
		expectedScheme, expectedToken, expectedOK := strings.Cut(expected, " ")
		if gotOK && expectedOK {
			return strings.EqualFold(gotScheme, expectedScheme) && secureEqual(gotToken, expectedToken)
		}
	}
	return secureEqual(got, expected)
}

func secureEqual(a, b string) bool {
	aHash := sha256.Sum256([]byte(a))
	bHash := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aHash[:], bHash[:]) == 1
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	required := s.cfg.Server.RequiredHeaders
	if len(required) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, expected := range required {
			got := r.Header.Get(key)
			if got == "" {
				writeError(w, http.StatusUnauthorized, "missing required header: "+key)
				return
			}
			if !headerValueMatches(key, got, expected) {
				writeError(w, http.StatusUnauthorized, "invalid header: "+key)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
