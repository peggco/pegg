package tool

import (
	"context"
	"sync"

	"github.com/peggco/pegg/internal/llm"
)

type attachmentSinkKey struct{}

// AttachmentSink collects attachments produced by tool calls.
type AttachmentSink struct {
	mu   sync.Mutex
	atts []llm.Attachment
}

func NewAttachmentSink() *AttachmentSink {
	return &AttachmentSink{}
}

func WithAttachmentSink(ctx context.Context, sink *AttachmentSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, attachmentSinkKey{}, sink)
}

func AddAttachment(ctx context.Context, att llm.Attachment) {
	sink, _ := ctx.Value(attachmentSinkKey{}).(*AttachmentSink)
	if sink == nil {
		return
	}
	sink.mu.Lock()
	sink.atts = append(sink.atts, att)
	sink.mu.Unlock()
}

func (s *AttachmentSink) Attachments() []llm.Attachment {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]llm.Attachment(nil), s.atts...)
}
