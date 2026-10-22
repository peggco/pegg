package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/peggco/pegg/internal/llm"
	openaidriver "github.com/peggco/pegg/internal/llm/drivers/openai"
)

type strictProviderServer struct {
	mu       sync.Mutex
	requests [][]map[string]any
}

func (s *strictProviderServer) handler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []map[string]any `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	s.mu.Lock()
	s.requests = append(s.requests, body.Messages)
	s.mu.Unlock()

	for i, msg := range body.Messages {
		if i > 0 && msg["role"] == "system" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"System message must be at the beginning"}}`))
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","model":"mock","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
}

func (s *strictProviderServer) seen() [][]map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

func TestOpenAIDriverRejectsLateSystemMessage(t *testing.T) {
	strict := &strictProviderServer{}
	server := httptest.NewServer(http.HandlerFunc(strict.handler))
	defer server.Close()

	driver := openaidriver.NewService("strict", openaidriver.ServiceConfig{
		BaseURL: server.URL,
		APIKey:  "test",
	})

	req := llm.NewRequest("mock", []llm.Message{
		llm.SystemMessage("you are pegg"),
		llm.UserMessage("hello"),
		llm.SystemMessage("late reminder"),
	})

	if _, err := driver.Chat(context.Background(), req); err == nil {
		t.Fatal("expected the strict provider to reject the late system message")
	} else if !strings.Contains(err.Error(), "System message must be at the beginning") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSystemFirstOnlySatisfiesStrictProvider(t *testing.T) {
	strict := &strictProviderServer{}
	server := httptest.NewServer(http.HandlerFunc(strict.handler))
	defer server.Close()

	driver := openaidriver.NewService("strict", openaidriver.ServiceConfig{
		BaseURL: server.URL,
		APIKey:  "test",
	})
	provider := llm.WithSystemFirstOnly(driver)

	req := llm.NewRequest("mock", []llm.Message{
		llm.SystemMessage("you are pegg"),
		llm.UserMessage("hello"),
		llm.SystemMessage("late reminder"),
	})

	resp, err := provider.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("the folded request must be accepted: %v", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	seen := strict.seen()
	if len(seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(seen))
	}
	for i, msg := range seen[0] {
		if i > 0 && msg["role"] == "system" {
			t.Fatalf("message %d is a late system message: %+v", i, msg)
		}
	}
	if !strings.Contains(seen[0][0]["content"].(string), "you are pegg") {
		t.Fatalf("system prompt = %+v", seen[0][0])
	}
}
