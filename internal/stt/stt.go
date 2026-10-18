package stt

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/peggco/pegg/internal/core/config"
)

type Manager struct {
	provider Provider
	err      error
	name     string
	model    string
	language string
	stream   bool
}

func New(cfg *config.Config) *Manager {
	m := &Manager{}
	if cfg == nil || cfg.STT == nil {
		m.err = ErrNotConfigured
		return m
	}
	settings := cfg.STT
	m.name = settings.Provider
	m.model = settings.Model
	if m.name == "" || m.model == "" {
		m.err = ErrNotConfigured
		return m
	}
	entry, ok := settings.ProviderConfig(m.name)
	if !ok || entry.APIKey == "" {
		m.err = fmt.Errorf("stt: provider %q has no API key", m.name)
		return m
	}
	providerCfg := config.LLMConfig{Provider: m.name, APIKey: entry.APIKey, BaseURL: entry.BaseURL}
	provider, err := NewProvider(m.name, providerCfg, settings)
	if err != nil {
		m.err = err
		return m
	}
	m.provider = provider
	m.language = cfg.Language
	m.stream = provider.SupportsStream() && IsStreamModel(m.name, m.model)
	return m
}

func (m *Manager) Configured() bool { return m != nil && m.provider != nil }

func (m *Manager) Provider() string { return m.name }

func (m *Manager) Model() string { return m.model }

func (m *Manager) StreamModel() string {
	if m != nil && m.stream {
		return m.model
	}
	return ""
}

func (m *Manager) Language() string { return m.language }

func (m *Manager) SupportsStream() bool {
	return m != nil && m.Configured() && m.stream
}

func (m *Manager) Transcribe(ctx context.Context, audio []byte, mediaType, fileName, language string) (*Result, error) {
	if m == nil || !m.Configured() {
		if m != nil && m.err != nil {
			return nil, m.err
		}
		return nil, ErrNotConfigured
	}
	if language == "" {
		language = m.language
	}
	model := m.model
	if IsStreamModel(m.name, model) {
		model = DefaultModel(m.name)
	}
	return m.provider.Transcribe(ctx, &Request{
		Model:     model,
		Language:  language,
		Audio:     audio,
		MediaType: mediaType,
		FileName:  fileName,
	})
}

type StreamOptions struct {
	Model      string
	Language   string
	SampleRate int
	Encoding   string
}

func (m *Manager) Stream(ctx context.Context, r io.Reader, handler StreamHandler) error {
	return m.StreamWithOptions(ctx, r, StreamOptions{}, handler)
}

func (m *Manager) StreamWithOptions(ctx context.Context, r io.Reader, opts StreamOptions, handler StreamHandler) error {
	if m == nil || !m.Configured() {
		if m != nil && m.err != nil {
			return m.err
		}
		return ErrNotConfigured
	}
	if !m.SupportsStream() {
		return ErrStreamUnsupported
	}
	model := opts.Model
	if model == "" {
		model = m.model
	}
	language := opts.Language
	if language == "" {
		language = m.language
	}
	sampleRate := opts.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	encoding := opts.Encoding
	if encoding == "" {
		encoding = "pcm_s16le"
	}
	return m.provider.Stream(ctx, &StreamRequest{
		Model:      model,
		Language:   language,
		SampleRate: sampleRate,
		Encoding:   encoding,
		Audio:      r,
	}, handler)
}

func (m *Manager) ModelsFor() []Model {
	if m == nil || m.name == "" {
		return nil
	}
	return ModelsFor(m.name)
}

func WAVFromPCM(pcm []byte, sampleRate int) []byte {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	const channels = 1
	const bitsPerSample = 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	out := make([]byte, 44+len(pcm))
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+len(pcm)))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], channels)
	binary.LittleEndian.PutUint32(out[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(out[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}
