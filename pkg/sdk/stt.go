package sdk

import (
	"context"
	"io"

	"github.com/peggco/pegg/internal/stt"
)

type STTRequest struct {
	Audio     []byte
	MediaType string
	FileName  string
}

type STTResponse struct {
	Text     string
	Provider string
	Model    string
}

type STTModel = stt.Model
type STTStreamEvent = stt.StreamEvent

var (
	ErrSTTNotConfigured     = stt.ErrNotConfigured
	ErrSTTStreamUnsupported = stt.ErrStreamUnsupported
)

func (e *Engine) Transcribe(ctx context.Context, req STTRequest) (STTResponse, error) {
	if err := e.checkOpen(); err != nil {
		return STTResponse{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager := stt.New(e.cfg)
	if !manager.Configured() {
		return STTResponse{}, stt.ErrNotConfigured
	}
	res, err := manager.Transcribe(ctx, req.Audio, req.MediaType, req.FileName, "")
	if err != nil {
		return STTResponse{}, err
	}
	return STTResponse{Text: res.Text, Provider: res.Provider, Model: res.Model}, nil
}

func (e *Engine) TranscribeStream(ctx context.Context, r io.Reader, handler func(STTStreamEvent) error) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager := stt.New(e.cfg)
	if !manager.Configured() {
		return stt.ErrNotConfigured
	}
	var h stt.StreamHandler
	if handler != nil {
		h = func(ev stt.StreamEvent) error { return handler(ev) }
	}
	return manager.Stream(ctx, r, h)
}

func (e *Engine) STTModels() []STTModel {
	manager := stt.New(e.cfg)
	return manager.ModelsFor()
}

func (e *Engine) STTConfigured() bool {
	return stt.New(e.cfg).Configured()
}

func (e *Engine) STTProviders() []string { return stt.ListProviders() }
