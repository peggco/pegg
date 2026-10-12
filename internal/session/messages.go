package session

import (
	"encoding/json"

	"github.com/peggco/pegg/internal/llm"
)

func MessagesToLLM(msgs []Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, llm.Message{
			Role:       m.Role,
			Content:    normalizeContent(m.Content),
			Reasoning:  m.Reasoning,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
			ToolCalls:  m.ToolCalls,
		})
	}
	return out
}

func normalizeContent(content any) any {
	switch v := content.(type) {
	case llm.Content:
		return llm.Content{Text: v.Text, Attachments: mediaOnly(v.Attachments)}
	case map[string]any:
		text, _ := v["text"].(string)
		atts := mediaOnly(decodeAttachments(v["attachments"]))
		if len(atts) == 0 {
			return text
		}
		return llm.Content{Text: text, Attachments: atts}
	default:
		return content
	}
}

func mediaOnly(atts []llm.Attachment) []llm.Attachment {
	if len(atts) == 0 {
		return nil
	}
	out := make([]llm.Attachment, 0, len(atts))
	for _, a := range atts {
		switch a.Type {
		case llm.AttachmentTypeImage, llm.AttachmentTypeAudio, llm.AttachmentTypeVideo:
			out = append(out, a)
		}
	}
	return out
}

func decodeAttachments(raw any) []llm.Attachment {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var atts []llm.Attachment
	if err := json.Unmarshal(data, &atts); err != nil {
		return nil
	}
	return atts
}
