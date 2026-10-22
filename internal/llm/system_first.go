package llm

import (
	"context"
	"strings"
)

type systemFirstProvider struct {
	inner Provider
}

func (p *systemFirstProvider) Name() string {
	return p.inner.Name()
}

func (p *systemFirstProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	return p.inner.Chat(ctx, withSystemFirstOnly(req))
}

func (p *systemFirstProvider) ChatStream(ctx context.Context, req *Request, handler StreamHandler) error {
	return p.inner.ChatStream(ctx, withSystemFirstOnly(req), handler)
}

func (p *systemFirstProvider) ListModels(ctx context.Context) ([]Model, error) {
	return p.inner.ListModels(ctx)
}

func (p *systemFirstProvider) SkipModelCache() bool {
	if s, ok := p.inner.(interface{ SkipModelCache() bool }); ok {
		return s.SkipModelCache()
	}
	return false
}

func WithSystemFirstOnly(p Provider) Provider {
	if p == nil {
		return nil
	}
	if _, ok := p.(*systemFirstProvider); ok {
		return p
	}
	return &systemFirstProvider{inner: p}
}

func withSystemFirstOnly(req *Request) *Request {
	if req == nil {
		return req
	}
	messages := FoldLateSystemMessages(req.Messages)
	if messages == nil {
		return req
	}
	clone := *req
	clone.Messages = messages
	return &clone
}

func FoldLateSystemMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	first := -1
	for i, msg := range messages {
		if msg.Role == RoleSystem {
			first = i
			break
		}
	}
	if first < 0 {
		return nil
	}
	if first == 0 {
		late := false
		for i := 1; i < len(messages); i++ {
			if messages[i].Role == RoleSystem {
				late = true
				break
			}
		}
		if !late {
			return nil
		}
	}

	source := messages
	if first > 0 {
		source = make([]Message, 0, len(messages))
		source = append(source, messages[first])
		source = append(source, messages[:first]...)
		source = append(source, messages[first+1:]...)
	}

	out := make([]Message, 0, len(source))
	pending := ""
	for i, msg := range source {
		if pending != "" {
			if msg.Role == RoleUser {
				msg = AppendMessageText(msg, pending)
			} else {
				out = append(out, UserMessage(pending))
			}
			pending = ""
		}
		if msg.Role != RoleSystem || i == 0 {
			out = append(out, msg)
			continue
		}
		text := MessageText(msg)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if n := len(out) - 1; n >= 0 && (out[n].Role == RoleUser || out[n].Role == RoleTool) {
			out[n] = AppendMessageText(out[n], text)
			continue
		}
		if i+1 < len(source) && source[i+1].Role == RoleUser {
			pending = text
			continue
		}
		msg.Role = RoleUser
		out = append(out, msg)
	}
	if pending != "" {
		out = append(out, UserMessage(pending))
	}
	return out
}

func AppendMessageText(msg Message, text string) Message {
	if strings.TrimSpace(text) == "" {
		return msg
	}
	switch content := msg.Content.(type) {
	case string:
		msg.Content = content + "\n\n" + text
	case Content:
		msg.Content = Content{Text: content.Text + "\n\n" + text, Attachments: content.Attachments}
	case []any:
		parts := append([]any(nil), content...)
		parts = append(parts, map[string]any{"type": "text", "text": text})
		msg.Content = parts
	default:
		msg.Content = MessageText(msg) + "\n\n" + text
	}
	return msg
}
