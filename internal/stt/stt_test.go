package stt_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
	_ "github.com/peggco/pegg/internal/stt/drivers"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

func testConfig(baseURL string) *config.Config {
	return &config.Config{
		STT: &config.STTConfig{
			Provider:  "openai",
			Model:     "whisper-1",
			Providers: []config.STTProviderConfig{{Provider: "openai", APIKey: "test-key", BaseURL: baseURL}},
		},
	}
}

func TestTranscribe(t *testing.T) {
	var gotModel, gotAuth, gotFile string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotModel = r.FormValue("model")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer file.Close()
		gotFile = header.Filename
		_ = json.NewEncoder(w).Encode(map[string]string{"text": " hello world "})
	}))
	defer srv.Close()

	m := stt.New(testConfig(srv.URL))
	if !m.Configured() {
		t.Fatal("expected configured manager")
	}
	res, err := m.Transcribe(context.Background(), []byte("audio"), "audio/webm", "", "en")
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if res.Text != "hello world" {
		t.Fatalf("text = %q", res.Text)
	}
	if gotModel != "whisper-1" {
		t.Fatalf("model = %q", gotModel)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotFile != "voice.webm" {
		t.Fatalf("filename = %q", gotFile)
	}
}

func TestTranscribeProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "invalid key", http.StatusUnauthorized)
	}))
	defer srv.Close()

	m := stt.New(testConfig(srv.URL))
	_, err := m.Transcribe(context.Background(), []byte("audio"), "audio/wav", "", "")
	if err == nil || !strings.Contains(err.Error(), "provider error (401)") {
		t.Fatalf("err = %v", err)
	}
}

func TestTranscribeNotConfigured(t *testing.T) {
	m := stt.New(&config.Config{})
	if _, err := m.Transcribe(context.Background(), []byte("x"), "audio/wav", "", ""); err != stt.ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamUnsupported(t *testing.T) {
	m := stt.New(testConfig("https://example.invalid"))
	if m.SupportsStream() {
		t.Fatal("openai should not support stream")
	}
	err := m.Stream(context.Background(), strings.NewReader("audio"), nil)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestModelsForConfigured(t *testing.T) {
	models := stt.ModelsForConfigured([]string{"assemblyai", "openai"})
	if len(models) == 0 {
		t.Fatal("expected models")
	}
	for _, m := range models {
		if m.Provider != "assemblyai" && m.Provider != "openai" {
			t.Fatalf("unexpected provider %q", m.Provider)
		}
	}
}

func TestAssemblyAIManagerStreaming(t *testing.T) {
	cfg := &config.Config{
		STT: &config.STTConfig{
			Provider:  "assemblyai",
			Model:     "universal-3-6-pro",
			Providers: []config.STTProviderConfig{{Provider: "assemblyai", APIKey: "test-key"}},
		},
	}
	m := stt.New(cfg)
	if !m.Configured() {
		t.Fatalf("manager not configured: %v", m)
	}
	if !m.SupportsStream() {
		t.Fatal("assemblyai manager should support streaming")
	}
	if m.StreamModel() != "universal-3-6-pro" {
		t.Fatalf("stream model = %q", m.StreamModel())
	}
}

func TestModelsForConfigUsesConfiguredModels(t *testing.T) {
	cfg := &config.STTConfig{
		Providers: []config.STTProviderConfig{{
			Provider: "pegg-team",
			APIKey:   "test-key",
			BaseURL:  "https://team.example.com/v1",
			Models: []config.STTModel{
				{ID: "whisper-large-v3", Name: "Whisper Large v3"},
				{ID: "gpt-4o-transcribe", Name: "GPT-4o Transcribe"},
			},
		}},
	}
	models := stt.ModelsForConfig("pegg-team", cfg)
	if len(models) != 2 {
		t.Fatalf("models = %+v", models)
	}
	if models[0].Provider != "pegg-team" || models[0].ID != "whisper-large-v3" {
		t.Fatalf("model = %+v", models[0])
	}
	if got := stt.DefaultModelForConfig("pegg-team", cfg); got != "whisper-large-v3" {
		t.Fatalf("default model = %q", got)
	}
	providers := stt.ProvidersForConfig(cfg)
	found := false
	for _, name := range providers {
		if name == "pegg-team" {
			found = true
		}
	}
	if !found {
		t.Fatalf("providers = %+v", providers)
	}
}

func TestModelsForConfigFallsBackToBuiltin(t *testing.T) {
	models := stt.ModelsForConfig("openai", &config.STTConfig{})
	if len(models) == 0 || models[0].Provider != "openai" {
		t.Fatalf("models = %+v", models)
	}
}

func TestManagerModelsForConfiguredProvider(t *testing.T) {
	cfg := &config.Config{
		STT: &config.STTConfig{
			Provider: "pegg-team",
			Model:    "whisper-large-v3",
			Providers: []config.STTProviderConfig{{
				Provider: "pegg-team",
				Driver:   "openai",
				APIKey:   "test-key",
				BaseURL:  "https://team.example.com/v1",
				Models:   []config.STTModel{{ID: "whisper-large-v3", Name: "Whisper Large v3"}},
			}},
		},
	}
	m := stt.New(cfg)
	if !m.Configured() {
		t.Fatalf("manager not configured: %v", m)
	}
	models := m.ModelsFor()
	if len(models) != 1 || models[0].ID != "whisper-large-v3" {
		t.Fatalf("models = %+v", models)
	}
}

func TestManagerUnknownDriver(t *testing.T) {
	cfg := &config.Config{
		STT: &config.STTConfig{
			Provider: "custom",
			Model:    "whisper-1",
			Providers: []config.STTProviderConfig{{
				Provider: "custom",
				Driver:   "missing",
				APIKey:   "test-key",
				BaseURL:  "https://example.invalid/v1",
			}},
		},
	}
	m := stt.New(cfg)
	if m.Configured() {
		t.Fatal("manager should not be configured with an unknown driver")
	}
	_, err := m.Transcribe(context.Background(), []byte("audio"), "audio/wav", "", "")
	if err == nil || !strings.Contains(err.Error(), "no driver registered") {
		t.Fatalf("err = %v", err)
	}
}

func TestManagerProviderTakesPrecedenceOverDriver(t *testing.T) {
	cfg := &config.Config{
		STT: &config.STTConfig{
			Provider: "openai",
			Model:    "whisper-1",
			Providers: []config.STTProviderConfig{{
				Provider: "openai",
				Driver:   "missing",
				APIKey:   "test-key",
				BaseURL:  "https://example.invalid/v1",
			}},
		},
	}
	m := stt.New(cfg)
	if !m.Configured() {
		t.Fatalf("registered provider should win over driver: %v", m)
	}
}
