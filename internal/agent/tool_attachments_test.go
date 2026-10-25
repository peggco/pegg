package agent

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
)

func TestToolAttachmentsReachNextRequest(t *testing.T) {
	img := llm.NewImageAttachmentFromBase64("image/png", "AAAA")
	img.FileName = "pixel.png"
	img.Path = "pixel.png"
	imageTool := tool.NewSpec("image-read", "reads images", nil, func(ctx context.Context, _ string) (string, error) {
		tool.AddAttachment(ctx, img)
		return "<image>pixel.png</image>", nil
	})

	a, prov := newTestAgent(t,
		WithTools(imageTool),
		WithModel(llm.Model{
			ID: "mock-model",
			Config: &config.ModelConfig{
				Modalities: &config.Modalities{Input: []string{"image"}},
			},
		}),
	)
	prov.responses = []mockResponse{
		{calls: []llm.ToolCall{toolCall("call-1", "image-read", "{}")}},
		{content: "done"},
	}

	if _, err := a.Run(context.Background(), "look at pixel.png"); err != nil {
		t.Fatal(err)
	}
	if prov.lastReq == nil {
		t.Fatal("provider received no request")
	}
	found := false
	for _, msg := range prov.lastReq.Messages {
		content, ok := msg.Content.(llm.Content)
		if !ok || len(content.Attachments) != 1 {
			continue
		}
		att := content.Attachments[0]
		if att.Type == llm.AttachmentTypeImage && att.MediaType == "image/png" && att.FileName == "pixel.png" {
			found = true
		}
	}
	if !found {
		t.Fatalf("image attachment missing from next request: %+v", prov.lastReq.Messages)
	}
}
