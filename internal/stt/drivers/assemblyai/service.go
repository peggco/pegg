package assemblyai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/peggco/pegg/internal/stt"
)

const (
	ProviderName      = "assemblyai"
	DefaultBaseURL    = "https://api.assemblyai.com"
	DefaultStreamURL  = "wss://streaming.assemblyai.com/v3/ws"
	defaultSampleRate = 16000
)

type ServiceConfig struct {
	Name      string
	APIKey    string
	BaseURL   string
	StreamURL string
	Timeout   time.Duration
	Models    []stt.Model
}

type Service struct {
	cfg    ServiceConfig
	client *http.Client
}

func New(cfg ServiceConfig) *Service {
	if cfg.Name == "" {
		cfg.Name = ProviderName
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.StreamURL == "" {
		cfg.StreamURL = DefaultStreamURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Minute
	}
	return &Service{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (s *Service) Name() string { return s.cfg.Name }

func (s *Service) SupportsStream() bool { return true }

func (s *Service) Models() []stt.Model {
	if len(s.cfg.Models) > 0 {
		return s.cfg.Models
	}
	return stt.ModelsFor(s.cfg.Name)
}

func (s *Service) Transcribe(ctx context.Context, req *stt.Request) (*stt.Result, error) {
	if len(req.Audio) == 0 {
		return nil, fmt.Errorf("assemblyai: audio is empty")
	}
	uploadURL, err := s.upload(ctx, req.Audio)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"audio_url":   uploadURL,
		"punctuate":   true,
		"format_text": true,
	}
	if req.Model != "" {
		body["speech_model"] = req.Model
	}
	if code := languageCode(req.Language); code != "" {
		body["language_code"] = code
	}
	var created transcript
	if err := s.doJSON(ctx, http.MethodPost, "/v2/transcript", body, &created); err != nil {
		return nil, err
	}
	if created.ID == "" {
		return nil, fmt.Errorf("assemblyai: create transcript returned no id")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := s.doJSON(ctx, http.MethodGet, "/v2/transcript/"+url.PathEscape(created.ID), nil, &created); err != nil {
			return nil, err
		}
		switch created.Status {
		case "completed":
			if created.Text == "" {
				return nil, fmt.Errorf("assemblyai: transcript is empty")
			}
			return &stt.Result{
				Text:     strings.TrimSpace(created.Text),
				Provider: s.cfg.Name,
				Model:    req.Model,
				Duration: float64(created.AudioDuration),
			}, nil
		case "error":
			if created.Error != "" {
				return nil, fmt.Errorf("assemblyai: %s", created.Error)
			}
			return nil, fmt.Errorf("assemblyai: transcription failed")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) upload(ctx context.Context, audio []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/v2/upload", bytes.NewReader(audio))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("assemblyai: upload: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("assemblyai: upload failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var out struct {
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", fmt.Errorf("assemblyai: decode upload: %w", err)
	}
	if out.UploadURL == "" {
		return "", fmt.Errorf("assemblyai: upload returned no url")
	}
	return out.UploadURL, nil
}

func (s *Service) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.cfg.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", s.cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("assemblyai: request: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("assemblyai: request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("assemblyai: decode response: %w", err)
		}
	}
	return nil
}

func (s *Service) Stream(ctx context.Context, req *stt.StreamRequest, handler stt.StreamHandler) error {
	if req == nil || req.Audio == nil {
		return fmt.Errorf("assemblyai: audio stream is required")
	}
	sampleRate := req.SampleRate
	if sampleRate <= 0 {
		sampleRate = defaultSampleRate
	}
	encoding := req.Encoding
	if encoding == "" {
		encoding = "pcm_s16le"
	}
	query := url.Values{}
	query.Set("sample_rate", fmt.Sprintf("%d", sampleRate))
	query.Set("encoding", encoding)
	query.Set("speech_model", req.Model)
	if req.Language != "" && req.Language != "auto" {
		if data, err := json.Marshal([]string{req.Language}); err == nil {
			query.Set("language_codes", string(data))
		}
	}
	endpoint := s.cfg.StreamURL + "?" + query.Encode()

	dialCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(dialCtx, endpoint, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{s.cfg.APIKey}},
	})
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("assemblyai: connect stream (%d): %w", status, err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	conn.SetReadLimit(1 << 20)

	ctx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()

	done := make(chan struct{})
	var readErr error
	var handlerErr error
	go func() {
		defer close(done)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					readErr = err
				}
				return
			}
			var msg streamMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "Turn":
				event := stt.StreamEvent{Text: msg.Transcript, Final: msg.EndOfTurn, Confidence: msg.EndOfTurnConfidence}
				if handler != nil {
					if err := handler(event); err != nil {
						handlerErr = err
						cancelStream()
						return
					}
				}
			case "Error":
				readErr = fmt.Errorf("assemblyai: %s", msg.Error)
				cancelStream()
				return
			case "Termination":
				return
			}
		}
	}()

	buf := make([]byte, 3200)
	for {
		n, readErrAudio := req.Audio.Read(buf)
		if n > 0 {
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				cancelStream()
				<-done
				if handlerErr != nil {
					return handlerErr
				}
				return fmt.Errorf("assemblyai: write audio: %w", err)
			}
		}
		if readErrAudio == io.EOF {
			break
		}
		if readErrAudio != nil {
			cancelStream()
			<-done
			return fmt.Errorf("assemblyai: read audio: %w", readErrAudio)
		}
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Terminate"}`)); err != nil {
		cancelStream()
		<-done
		return fmt.Errorf("assemblyai: terminate: %w", err)
	}

	finalTimeout := 10 * time.Second
	if req.FinalTimeout > 0 {
		finalTimeout = time.Duration(req.FinalTimeout) * time.Second
	}
	select {
	case <-done:
	case <-time.After(finalTimeout):
		cancelStream()
		<-done
	case <-ctx.Done():
		<-done
	}
	if handlerErr != nil {
		return handlerErr
	}
	if readErr != nil && ctx.Err() == nil {
		return readErr
	}
	return ctx.Err()
}

type transcript struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Text          string `json:"text"`
	Error         string `json:"error"`
	AudioDuration int    `json:"audio_duration"`
}

type streamMessage struct {
	Type                string  `json:"type"`
	Transcript          string  `json:"transcript"`
	EndOfTurn           bool    `json:"end_of_turn"`
	EndOfTurnConfidence float64 `json:"end_of_turn_confidence"`
	Error               string  `json:"error"`
}

func languageCode(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "auto":
		return ""
	case "en":
		return "en_us"
	case "en-us", "en_us":
		return "en_us"
	case "en-gb", "en_uk":
		return "en_uk"
	case "en-au", "en_au":
		return "en_au"
	case "tr":
		return "tr"
	default:
		return strings.ToLower(strings.TrimSpace(language))
	}
}
