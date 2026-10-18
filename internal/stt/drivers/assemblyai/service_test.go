package assemblyai

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/peggco/pegg/internal/stt"
)

func TestTranscribe(t *testing.T) {
	var uploaded bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/upload":
			uploaded = true
			_, _ = w.Write([]byte(`{"upload_url":"https://example.test/audio.wav"}`))
		case r.URL.Path == "/v2/transcript" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"tr_1","status":"queued"}`))
		case strings.HasPrefix(r.URL.Path, "/v2/transcript/"):
			_, _ = w.Write([]byte(`{"id":"tr_1","status":"completed","text":" hello assembly ","audio_duration":3}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := New(ServiceConfig{APIKey: "test-key", BaseURL: srv.URL})
	res, err := svc.Transcribe(context.Background(), &stt.Request{
		Model: "best", Audio: []byte("audio"), Language: "en",
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if !uploaded {
		t.Fatal("expected upload call")
	}
	if res.Text != "hello assembly" {
		t.Fatalf("text = %q", res.Text)
	}
	if res.Duration != 3 {
		t.Fatalf("duration = %v", res.Duration)
	}
}

func TestStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		ctx := r.Context()
		for {
			typ, _, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageText {
				break
			}
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Turn","transcript":"hello","end_of_turn":false}`))
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Turn","transcript":"hello world","end_of_turn":true}`))
		}
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Turn","transcript":"hello world","end_of_turn":true}`))
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Termination","audio_duration_seconds":1}`))
	}))
	defer srv.Close()

	streamURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v3/ws"
	svc := New(ServiceConfig{APIKey: "test-key", StreamURL: streamURL})
	var events []stt.StreamEvent
	err := svc.Stream(context.Background(), &stt.StreamRequest{
		Model: "universal-3-6-pro", Encoding: "pcm_s16le", SampleRate: 16000,
		Audio: bytes.NewReader([]byte("pcm-audio")),
	}, func(ev stt.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	if !events[len(events)-1].Final || events[len(events)-1].Text != "hello world" {
		t.Fatalf("events = %+v", events)
	}
}

func TestLanguageCode(t *testing.T) {
	if got := languageCode("en"); got != "en_us" {
		t.Fatalf("languageCode(en) = %q", got)
	}
	if got := languageCode("auto"); got != "" {
		t.Fatalf("languageCode(auto) = %q", got)
	}
}
