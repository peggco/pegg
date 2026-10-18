package stt

import (
	"context"
	"io"
)

type Model struct {
	Provider  string `json:"provider"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Streaming bool   `json:"streaming,omitempty"`
}

type Request struct {
	Model     string
	Language  string
	Audio     []byte
	MediaType string
	FileName  string
}

type Result struct {
	Text     string  `json:"text"`
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Duration float64 `json:"duration,omitempty"`
}

type StreamRequest struct {
	Model      string
	Language   string
	SampleRate int
	Encoding   string
	Audio      io.Reader

	FinalTimeout int
}

type StreamEvent struct {
	Text       string  `json:"text"`
	Final      bool    `json:"final"`
	Confidence float64 `json:"confidence,omitempty"`
	Error      error   `json:"-"`
}

type StreamHandler func(event StreamEvent) error

type Provider interface {
	Name() string
	Models() []Model
	Transcribe(ctx context.Context, req *Request) (*Result, error)
	Stream(ctx context.Context, req *StreamRequest, handler StreamHandler) error
	SupportsStream() bool
}
