package acp

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/stt"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

const maxSTTBytes = 8 << 20

type STTTranscribeParams struct {
	Audio     string `json:"audio"`
	MediaType string `json:"mediaType,omitempty"`
	FileName  string `json:"fileName,omitempty"`
}

type STTTranscribeResult struct {
	Text     string `json:"text"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

func (s *Server) handleSTTTranscribe(ctx context.Context, params json.RawMessage) (any, error) {
	var req STTTranscribeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid stt/transcribe params"}
	}
	res, err := s.transcribeBase64(ctx, req.Audio, req.MediaType, req.FileName)
	if err != nil {
		return nil, err
	}
	return STTTranscribeResult{Text: res.Text, Provider: res.Provider, Model: res.Model}, nil
}

func (s *Server) transcribeBase64(ctx context.Context, encoded, mediaType, fileName string) (*stt.Result, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "audio is required"}
	}
	audio, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "audio is not valid base64"}
	}
	if len(audio) == 0 {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "audio is empty"}
	}
	if len(audio) > maxSTTBytes {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "audio exceeds 8 MiB"}
	}
	if mediaType == "" {
		mediaType = "audio/webm"
	}
	manager := stt.New(s.cfg)
	if !manager.Configured() {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "speech-to-text is not configured"}
	}
	transcribeCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := manager.Transcribe(transcribeCtx, audio, mediaType, fileName, "")
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: err.Error()}
	}
	return res, nil
}

func (s *Server) transcribeAudioBlocks(ctx context.Context, blocks []ContentBlock) (string, error) {
	for _, block := range blocks {
		if block.Type != "audio" || block.Data == "" {
			continue
		}
		res, err := s.transcribeBase64(ctx, block.Data, block.MimeType, "")
		if err != nil {
			return "", err
		}
		return res.Text, nil
	}
	return "", nil
}
