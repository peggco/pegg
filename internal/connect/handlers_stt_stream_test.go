package connect

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/peggco/pegg/internal/core/config"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

func TestSTTStream(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
		}
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Turn","transcript":"hello stream","end_of_turn":true}`))
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Termination","audio_duration_seconds":1}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.STT = &config.STTConfig{
		Provider: "assemblyai",
		Model:    "universal-3-6-pro",
		Providers: []config.STTProviderConfig{{
			Provider:  "assemblyai",
			APIKey:    "test-key",
			StreamURL: "ws" + strings.TrimPrefix(srv.URL, "http"),
		}},
	}
	c, err := New(Deps{Config: cfg, Log: testLogger(), Version: "test"}, cfg.Connect)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop()

	start, err := c.handleSTTStreamStart(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	id, _ := start.(map[string]any)["stream_id"].(string)
	if id == "" {
		t.Fatal("missing stream_id")
	}
	audio := base64.StdEncoding.EncodeToString([]byte("pcm-audio"))
	if _, err := c.handleSTTStreamAudio(context.Background(), &Request{Params: []byte(`{"stream_id":"` + id + `","audio":"` + audio + `"}`)}); err != nil {
		t.Fatalf("audio: %v", err)
	}
	res, err := c.handleSTTStreamStop(context.Background(), &Request{Params: []byte(`{"stream_id":"` + id + `"}`)})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got, _ := res.(map[string]any)["text"].(string); got != "hello stream" {
		t.Fatalf("text = %q", got)
	}
}
