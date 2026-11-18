package server

import (
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.5.5.5", true},
		{"::1", true},
		{"[::1]", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{" 127.0.0.1 ", true},
		{"0.0.0.0", false},
		{"::", false},
		{"192.168.1.10", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := IsLoopbackHost(tc.host); got != tc.want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestValidateHTTPServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.ServerConfig
		wantErr bool
	}{
		{
			name: "loopback without auth",
			cfg:  config.ServerConfig{Host: "127.0.0.1"},
		},
		{
			name: "localhost without auth",
			cfg:  config.ServerConfig{Host: "localhost"},
		},
		{
			name:    "all interfaces without auth",
			cfg:     config.ServerConfig{Host: "0.0.0.0"},
			wantErr: true,
		},
		{
			name:    "empty host without auth",
			cfg:     config.ServerConfig{Host: ""},
			wantErr: true,
		},
		{
			name:    "public host without auth",
			cfg:     config.ServerConfig{Host: "203.0.113.5"},
			wantErr: true,
		},
		{
			name: "non-loopback with required headers",
			cfg: config.ServerConfig{
				Host:            "0.0.0.0",
				RequiredHeaders: map[string]string{"Authorization": "Bearer secret"},
			},
		},
		{
			name: "non-loopback with explicit opt-in",
			cfg: config.ServerConfig{
				Host:                 "0.0.0.0",
				AllowUnauthenticated: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHTTPServerConfig(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateHTTPServerConfig() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "non-loopback") {
				t.Errorf("error should explain the non-loopback refusal, got %v", err)
			}
		})
	}
}

func TestValidateACPServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.ServerConfig
		wantErr bool
	}{
		{
			name: "loopback",
			cfg:  config.ServerConfig{Host: "127.0.0.1"},
		},
		{
			name:    "all interfaces without opt-in",
			cfg:     config.ServerConfig{Host: "0.0.0.0"},
			wantErr: true,
		},
		{
			name: "required headers do not protect ACP",
			cfg: config.ServerConfig{
				Host:            "0.0.0.0",
				RequiredHeaders: map[string]string{"Authorization": "Bearer secret"},
			},
			wantErr: true,
		},
		{
			name: "explicit opt-in",
			cfg: config.ServerConfig{
				Host:                 "0.0.0.0",
				AllowUnauthenticated: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateACPServerConfig(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateACPServerConfig() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
