package connect

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/stt"
)

const maxSTTAudioBytes = 4 << 20

func (c *Connector) registerSTTMethods() {
	c.dispatch.register("stt.transcribe", ScopeChat, c.handleSTTTranscribe)
	c.dispatch.register("stt.status", ScopeChat, c.handleSTTStatus)
}

func (c *Connector) handleSTTStatus(_ context.Context, _ *Request) (any, error) {
	cfg := c.sttConfig()
	manager := stt.New(c.deps.Config)
	hasKey := false
	if entry, ok := cfg.ProviderConfig(cfg.Provider); ok {
		hasKey = entry.APIKey != ""
	}
	return map[string]any{
		"configured":        manager.Configured(),
		"provider":          cfg.Provider,
		"model":             cfg.Model,
		"stream":            manager.SupportsStream(),
		"has_key":           hasKey,
		"max_duration_secs": cfg.MaxDurationValue(),
	}, nil
}

type sttTranscribeParams struct {
	Audio     string `json:"audio"`
	MediaType string `json:"media_type"`
	FileName  string `json:"file_name"`
}

var sttMediaTypes = map[string]struct{}{
	"audio/webm":  {},
	"audio/ogg":   {},
	"audio/mp4":   {},
	"audio/m4a":   {},
	"audio/mpeg":  {},
	"audio/mp3":   {},
	"audio/wav":   {},
	"audio/x-wav": {},
	"audio/wave":  {},
}

func (c *Connector) handleSTTTranscribe(ctx context.Context, req *Request) (any, error) {
	var p sttTranscribeParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	if p.Audio == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio is required"}
	}
	if p.MediaType == "" {
		p.MediaType = "audio/webm"
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(p.MediaType, ";")[0]))
	if _, ok := sttMediaTypes[mediaType]; !ok {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "unsupported audio media_type: " + p.MediaType}
	}
	audio, err := base64.StdEncoding.DecodeString(p.Audio)
	if err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio is not valid base64"}
	}
	if len(audio) == 0 {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio is empty"}
	}
	if len(audio) > maxSTTAudioBytes {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio exceeds the 4 MiB limit"}
	}
	if c.deps.Config == nil {
		if err := c.reloadConfig(); err != nil {
			return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
		}
	}
	svc := stt.New(c.deps.Config)
	if !svc.Configured() {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "speech-to-text is not configured"}
	}

	start := time.Now()
	c.emit(eventSTTStarted, "", "", map[string]any{"source": "web"})
	res, err := svc.Transcribe(ctx, audio, mediaType, p.FileName, "")
	if err != nil {
		c.emit(eventSTTError, "", "", map[string]any{"source": "web", "message": err.Error()})
		return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
	}
	c.emit(eventSTTFinished, "", "", map[string]any{
		"source":      "web",
		"duration_ms": time.Since(start).Milliseconds(),
		"provider":    res.Provider,
		"model":       res.Model,
	})
	return map[string]any{
		"text":     res.Text,
		"provider": res.Provider,
		"model":    res.Model,
	}, nil
}
