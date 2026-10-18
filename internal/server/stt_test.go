package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

func init() {
	stt.RegisterProvider("stt-fake", func(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
		return fakeSTT{}, nil
	})
}

type fakeSTT struct{}

func (fakeSTT) Name() string         { return "stt-fake" }
func (fakeSTT) Models() []stt.Model  { return nil }
func (fakeSTT) SupportsStream() bool { return false }
func (fakeSTT) Stream(context.Context, *stt.StreamRequest, stt.StreamHandler) error {
	return stt.ErrStreamUnsupported
}
func (fakeSTT) Transcribe(_ context.Context, req *stt.Request) (*stt.Result, error) {
	return &stt.Result{Text: "hello from http", Provider: "stt-fake", Model: req.Model}, nil
}

func TestHandleSTT(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.STT = &config.STTConfig{
		Provider:  "stt-fake",
		Model:     "fake",
		Providers: []config.STTProviderConfig{{Provider: "stt-fake", APIKey: "key"}},
	}
	body := `{"audio":"` + base64.StdEncoding.EncodeToString([]byte("audio")) + `","media_type":"audio/webm"}`
	req := httptest.NewRequest(http.MethodPost, "/api/stt", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSTT(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello from http") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestHandleSTTMissingAudio(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/stt", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.handleSTT(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}
