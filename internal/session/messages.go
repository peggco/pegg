package session

import (
	"encoding/json"
	"strings"

	"github.com/peggco/pegg/internal/llm"
)

func MessagesToLLM(msgs []Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	var pending []llm.Attachment
	flush := func() {
		if len(pending) == 0 {
			return
		}
		names := make([]string, 0, len(pending))
		for _, att := range pending {
			if att.FileName != "" {
				names = append(names, att.FileName)
			}
		}
		text := "Images loaded by the read tool"
		if len(names) > 0 {
			text += ": " + strings.Join(names, ", ")
		}
		out = append(out, llm.UserMessage(llm.ContentWithAttachments(text, pending)))
		pending = nil
	}
	for _, m := range msgs {
		msg := llm.Message{
			Role:       m.Role,
			Content:    normalizeContent(m.Content),
			Reasoning:  m.Reasoning,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
			ToolCalls:  m.ToolCalls,
		}
		if msg.Role == llm.RoleTool {
			if content, ok := msg.Content.(llm.Content); ok && len(content.Attachments) > 0 {
				atts := mediaOnly(content.Attachments)
				msg.Content = content.Text
				pending = append(pending, atts...)
			}
			out = append(out, msg)
			continue
		}
		flush()
		out = append(out, msg)
	}
	flush()
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
