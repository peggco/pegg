package connect

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

func TestSTTTranscribe(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"text": "hello from voice"})
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.STT = &config.STTConfig{
		Provider:  "openai",
		Model:     "whisper-1",
		Providers: []config.STTProviderConfig{{Provider: "openai", APIKey: "test-key", BaseURL: srv.URL}},
	}
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	params, _ := json.Marshal(map[string]string{
		"audio":      base64.StdEncoding.EncodeToString([]byte("audio")),
		"media_type": "audio/webm",
	})
	res, err := c.handleSTTTranscribe(context.Background(), &Request{Params: params})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	out, _ := res.(map[string]any)
	if out["text"] != "hello from voice" {
		t.Fatalf("text = %v", out["text"])
	}
}

func TestSTTTranscribeRejects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	checks := []string{
		`{}`,
		`{"audio":"not-base64!"}`,
		`{"audio":"eA==","media_type":"text/plain"}`,
	}
	for _, body := range checks {
		_, err := c.handleSTTTranscribe(context.Background(), &Request{Params: []byte(body)})
		if err == nil {
			t.Fatalf("expected error for %s", body)
		}
		fe, ok := err.(*FrameError)
		if !ok || fe.Code != ErrCodeBadRequest {
			t.Fatalf("expected bad_request for %s, got %v", body, err)
		}
	}
}

func TestSTTTranscribeNotConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	_, err = c.handleSTTTranscribe(context.Background(), &Request{Params: []byte(`{"audio":"eA=="}`)})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v", err)
	}
}

func TestSTTStatusAssemblyAI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.STT = &config.STTConfig{
		Provider:  "assemblyai",
		Model:     "universal-3-6-pro",
		Providers: []config.STTProviderConfig{{Provider: "assemblyai", APIKey: "test-key"}},
	}
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	res, err := c.handleSTTStatus(context.Background(), &Request{})
	if err != nil {
		t.Fatal(err)
	}
	out := res.(map[string]any)
	if out["configured"] != true || out["stream"] != true {
		t.Fatalf("status = %+v", out)
	}
}
