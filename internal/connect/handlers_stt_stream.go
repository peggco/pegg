package connect

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/peggco/pegg/internal/stt"
)

type sttStreamSession struct {
	id     string
	writer *io.PipeWriter
	cancel context.CancelFunc
	done   chan struct{}

	mu    sync.Mutex
	text  string
	err   error
	bytes int
}

func (c *Connector) registerSTTStreamMethods() {
	c.dispatch.register("stt.stream.start", ScopeChat, c.handleSTTStreamStart)
	c.dispatch.register("stt.stream.audio", ScopeChat, c.handleSTTStreamAudio)
	c.dispatch.register("stt.stream.stop", ScopeChat, c.handleSTTStreamStop)
}

type sttStreamStartParams struct {
	Model      string `json:"model"`
	Language   string `json:"language"`
	SampleRate int    `json:"sample_rate"`
	Encoding   string `json:"encoding"`
	MediaType  string `json:"media_type"`
}

func (c *Connector) handleSTTStreamStart(_ context.Context, req *Request) (any, error) {
	var p sttStreamStartParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	if c.deps.Config == nil {
		if err := c.reloadConfig(); err != nil {
			return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
		}
	}
	manager := stt.New(c.deps.Config)
	if !manager.Configured() {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "speech-to-text is not configured"}
	}
	if !manager.SupportsStream() {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "configured STT provider does not support streaming"}
	}
	sampleRate := p.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	encoding := p.Encoding
	if encoding == "" {
		encoding = "pcm_s16le"
	}
	model := p.Model
	if model == "" {
		model = manager.StreamModel()
	}
	if stt.IsAsyncModel(manager.Provider(), model) {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "not a streaming model: " + model}
	}
	language := manager.Language()

	ctx := c.runCtx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	reader, writer := io.Pipe()
	session := &sttStreamSession{
		id:     randomHex(12),
		writer: writer,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	c.sttMu.Lock()
	c.sttStreams[session.id] = session
	c.sttMu.Unlock()

	c.emit(eventSTTStarted, "", "", map[string]any{"source": "web", "stream_id": session.id})
	go func() {
		defer close(session.done)
		defer cancel()
		var final strings.Builder
		err := manager.StreamWithOptions(ctx, reader, stt.StreamOptions{
			Model:      model,
			Language:   language,
			SampleRate: sampleRate,
			Encoding:   encoding,
		}, func(ev stt.StreamEvent) error {
			text := strings.TrimSpace(ev.Text)
			if ev.Final && text != "" {
				if final.Len() > 0 {
					final.WriteByte(' ')
				}
				final.WriteString(text)
			}
			c.emit(eventSTTPartial, "", "", map[string]any{
				"source":    "web",
				"stream_id": session.id,
				"text":      text,
				"final":     ev.Final,
			})
			return nil
		})
		session.mu.Lock()
		session.text = final.String()
		if err != nil && !errors.Is(err, context.Canceled) {
			session.err = err
		}
		session.mu.Unlock()
		if err != nil && !errors.Is(err, context.Canceled) {
			c.emit(eventSTTError, "", "", map[string]any{"source": "web", "stream_id": session.id, "message": err.Error()})
		} else {
			c.emit(eventSTTFinished, "", "", map[string]any{"source": "web", "stream_id": session.id})
		}
	}()

	return map[string]any{"stream_id": session.id, "model": model, "language": language, "sample_rate": sampleRate, "encoding": encoding}, nil
}

type sttStreamAudioParams struct {
	StreamID string `json:"stream_id"`
	Audio    string `json:"audio"`
}

func (c *Connector) handleSTTStreamAudio(_ context.Context, req *Request) (any, error) {
	var p sttStreamAudioParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	if p.StreamID == "" || p.Audio == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "stream_id and audio are required"}
	}
	c.sttMu.Lock()
	session := c.sttStreams[p.StreamID]
	c.sttMu.Unlock()
	if session == nil {
		return nil, &FrameError{Code: ErrCodeNotFound, Message: "unknown stream_id"}
	}
	audio, err := base64.StdEncoding.DecodeString(p.Audio)
	if err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio is not valid base64"}
	}
	if len(audio) == 0 {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio is empty"}
	}
	if len(audio) > 512<<10 {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "audio chunk exceeds 512 KiB"}
	}
	session.mu.Lock()
	if session.bytes+len(audio) > 8<<20 {
		session.mu.Unlock()
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "stream audio exceeds 8 MiB"}
	}
	session.bytes += len(audio)
	session.mu.Unlock()
	if _, err := session.writer.Write(audio); err != nil {
		return nil, &FrameError{Code: ErrCodeUnavailable, Message: "stream is closed"}
	}
	return map[string]any{"stream_id": p.StreamID, "bytes": len(audio)}, nil
}

type sttStreamStopParams struct {
	StreamID string `json:"stream_id"`
}

func (c *Connector) handleSTTStreamStop(_ context.Context, req *Request) (any, error) {
	var p sttStreamStopParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	c.sttMu.Lock()
	session := c.sttStreams[p.StreamID]
	delete(c.sttStreams, p.StreamID)
	c.sttMu.Unlock()
	if session == nil {
		return nil, &FrameError{Code: ErrCodeNotFound, Message: "unknown stream_id"}
	}
	_ = session.writer.Close()
	select {
	case <-session.done:
	case <-time.After(25 * time.Second):
		session.cancel()
		<-session.done
	}
	session.mu.Lock()
	text, err := session.text, session.err
	session.mu.Unlock()
	if err != nil && text == "" {
		return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
	}
	return map[string]any{"stream_id": session.id, "text": text}, nil
}

func (c *Connector) closeSTTStreams() {
	c.sttMu.Lock()
	sessions := make([]*sttStreamSession, 0, len(c.sttStreams))
	for id, session := range c.sttStreams {
		delete(c.sttStreams, id)
		sessions = append(sessions, session)
	}
	c.sttMu.Unlock()
	for _, session := range sessions {
		session.cancel()
		_ = session.writer.Close()
	}
}
