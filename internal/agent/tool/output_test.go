package tool

import (
	"strings"
	"testing"
)

func TestOutputSinkCoalescesAndFlushes(t *testing.T) {
	var chunks []string
	sink := NewOutputSink(func(stream, chunk string) {
		chunks = append(chunks, stream+":"+chunk)
	})

	sink.Write("stdout", "a\n")
	sink.Write("stdout", "b\n")
	sink.Close()

	if got := strings.Join(chunks, ""); got != "stdout:a\nb\n" {
		t.Fatalf("streamed %q, want %q", got, "stdout:a\nb\n")
	}
}

func TestOutputSinkFlushesOnSize(t *testing.T) {
	var chunks []string
	sink := NewOutputSink(func(stream, chunk string) {
		chunks = append(chunks, chunk)
	})

	sink.Write("stdout", strings.Repeat("x", outputFlushBytes))
	if len(chunks) != 1 || len(chunks[0]) != outputFlushBytes {
		t.Fatalf("got %d chunks, want one %d-byte chunk", len(chunks), outputFlushBytes)
	}
	sink.Close()
}

func TestOutputWriterWithoutSinkDiscards(t *testing.T) {
	w := OutputWriter(t.Context(), "stdout")
	if _, err := w.Write([]byte("ignored")); err != nil {
		t.Fatalf("write: %v", err)
	}
}
