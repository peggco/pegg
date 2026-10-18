package openai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/stt"
)

const maxResponseBytes = 1 << 20

type ServiceConfig struct {
	Name    string
	BaseURL string
	APIKey  string
	Headers map[string]string
	Timeout time.Duration
}

type Service struct {
	name   string
	cfg    ServiceConfig
	client *http.Client
}

func New(cfg ServiceConfig) *Service {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Service{name: cfg.Name, cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (s *Service) Name() string { return s.name }

func (s *Service) SupportsStream() bool { return false }

func (s *Service) Models() []stt.Model { return stt.ModelsFor(s.name) }

func (s *Service) Transcribe(ctx context.Context, req *stt.Request) (*stt.Result, error) {
	if req == nil || len(req.Audio) == 0 {
		return nil, fmt.Errorf("stt: audio is empty")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", req.Model)
	_ = writer.WriteField("response_format", "json")
	if req.Language != "" && req.Language != "auto" {
		_ = writer.WriteField("language", req.Language)
	}
	name := req.FileName
	if name == "" {
		name = "voice." + extension(req.MediaType)
	}
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(req.Audio); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/audio/transcriptions", &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	for key, value := range s.cfg.Headers {
		httpReq.Header.Set(key, value)
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("stt: request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(payload))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, fmt.Errorf("stt: provider error (%d): %s", resp.StatusCode, msg)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("stt: decode response: %w", err)
	}
	text := strings.TrimSpace(out.Text)
	if text == "" {
		return nil, fmt.Errorf("stt: provider returned an empty transcript")
	}
	return &stt.Result{Text: text, Provider: s.name, Model: req.Model}, nil
}

func (s *Service) Stream(context.Context, *stt.StreamRequest, stt.StreamHandler) error {
	return stt.ErrStreamUnsupported
}

func extension(mediaType string) string {
	mt := strings.ToLower(strings.TrimSpace(strings.Split(mediaType, ";")[0]))
	switch mt {
	case "audio/webm":
		return "webm"
	case "audio/ogg":
		return "ogg"
	case "audio/mp4", "audio/m4a", "audio/aac":
		return "m4a"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return "wav"
	default:
		return "webm"
	}
}
