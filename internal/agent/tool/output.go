package tool

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	outputFlushInterval = 100 * time.Millisecond
	outputFlushBytes    = 8 * 1024
)

type OutputFunc func(stream, chunk string)

type outputStream struct {
	buf   strings.Builder
	timer *time.Timer
}

type OutputSink struct {
	mu      sync.Mutex
	emit    OutputFunc
	streams map[string]*outputStream
	closed  bool
}

func NewOutputSink(emit OutputFunc) *OutputSink {
	return &OutputSink{emit: emit, streams: make(map[string]*outputStream)}
}

func (s *OutputSink) Write(stream, chunk string) {
	if s == nil || chunk == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	st := s.streams[stream]
	if st == nil {
		st = &outputStream{}
		s.streams[stream] = st
	}
	st.buf.WriteString(chunk)
	if st.buf.Len() >= outputFlushBytes {
		s.flushLocked(stream, st)
		return
	}
	if st.timer == nil {
		st.timer = time.AfterFunc(outputFlushInterval, func() {
			s.flush(stream)
		})
	}
}

func (s *OutputSink) flush(stream string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.streams[stream]; st != nil {
		s.flushLocked(stream, st)
	}
}

func (s *OutputSink) flushLocked(stream string, st *outputStream) {
	if st.timer != nil {
		st.timer.Stop()
		st.timer = nil
	}
	if st.buf.Len() == 0 {
		return
	}
	chunk := st.buf.String()
	st.buf.Reset()
	if s.emit != nil {
		s.emit(stream, chunk)
	}
}

func (s *OutputSink) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for stream, st := range s.streams {
		s.flushLocked(stream, st)
	}
}

type outputSinkKey struct{}

func WithOutputSink(ctx context.Context, sink *OutputSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, outputSinkKey{}, sink)
}

func OutputSinkFromContext(ctx context.Context) *OutputSink {
	sink, _ := ctx.Value(outputSinkKey{}).(*OutputSink)
	return sink
}

func OutputWriter(ctx context.Context, stream string) io.Writer {
	sink := OutputSinkFromContext(ctx)
	if sink == nil {
		return io.Discard
	}
	return &outputWriter{sink: sink, stream: stream}
}

type outputWriter struct {
	sink   *OutputSink
	stream string
}

func (w *outputWriter) Write(p []byte) (int, error) {
	w.sink.Write(w.stream, string(p))
	return len(p), nil
}
