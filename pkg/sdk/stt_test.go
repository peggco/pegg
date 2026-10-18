package sdk

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

func init() {
	stt.RegisterProvider("sdk-stt-fake", func(cfg config.LLMConfig, settings *config.STTConfig) (stt.Provider, error) {
		return sdkFakeSTT{}, nil
	})
}

type sdkFakeSTT struct{}

func (sdkFakeSTT) Name() string         { return "sdk-stt-fake" }
func (sdkFakeSTT) Models() []stt.Model  { return nil }
func (sdkFakeSTT) SupportsStream() bool { return false }
func (sdkFakeSTT) Stream(context.Context, *stt.StreamRequest, stt.StreamHandler) error {
	return stt.ErrStreamUnsupported
}
func (sdkFakeSTT) Transcribe(_ context.Context, req *stt.Request) (*stt.Result, error) {
	return &stt.Result{Text: "hello from sdk", Provider: "sdk-stt-fake", Model: req.Model}, nil
}

func TestSDKTranscribe(t *testing.T) {
	e := &Engine{cfg: &config.Config{STT: &config.STTConfig{
		Provider:  "sdk-stt-fake",
		Model:     "fake",
		Providers: []config.STTProviderConfig{{Provider: "sdk-stt-fake", APIKey: "key"}},
	}}}
	res, err := e.Transcribe(context.Background(), STTRequest{Audio: []byte("audio"), MediaType: "audio/webm"})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if res.Text != "hello from sdk" {
		t.Fatalf("text = %q", res.Text)
	}
	if !e.STTConfigured() {
		t.Fatal("expected configured")
	}
}
