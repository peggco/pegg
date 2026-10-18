package acp

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

func init() {
	stt.RegisterProvider("acp-stt-fake", func(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
		return acpFakeSTT{}, nil
	})
}

type acpFakeSTT struct{}

func (acpFakeSTT) Name() string         { return "acp-stt-fake" }
func (acpFakeSTT) Models() []stt.Model  { return nil }
func (acpFakeSTT) SupportsStream() bool { return false }
func (acpFakeSTT) Stream(context.Context, *stt.StreamRequest, stt.StreamHandler) error {
	return stt.ErrStreamUnsupported
}
func (acpFakeSTT) Transcribe(_ context.Context, req *stt.Request) (*stt.Result, error) {
	return &stt.Result{Text: "hello from acp", Provider: "acp-stt-fake", Model: req.Model}, nil
}

func TestACPTranscribeAudioBlock(t *testing.T) {
	srv := &Server{cfg: &config.Config{STT: &config.STTConfig{
		Provider:  "acp-stt-fake",
		Model:     "fake",
		Providers: []config.STTProviderConfig{{Provider: "acp-stt-fake", APIKey: "key"}},
	}}}
	text, err := srv.transcribeAudioBlocks(context.Background(), []ContentBlock{{
		Type: "audio", Data: base64.StdEncoding.EncodeToString([]byte("audio")), MimeType: "audio/webm",
	}})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if !strings.Contains(text, "hello from acp") {
		t.Fatalf("text = %q", text)
	}
}
