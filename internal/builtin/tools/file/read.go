package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/vfs"
)

const maxReadImageBytes = 5 << 20

var readImageMediaTypes = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

func generateReadToolPrompt() (string, error) {
	sys, err := readToolPromptBuilder().
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}

func readTool(fs *vfs.VFS) tool.Tool {
	prompt, err := generateReadToolPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to generate read tool prompt: %v", err))
	}

	return tool.NewSpec(
		"read",
		prompt,
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filePath": map[string]any{
					"type":        "string",
					"description": "The path to the file to read.",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "The line number to start reading from (1-based)",
					"minimum":     1,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "The number of lines to read (defaults to 2000)",
					"minimum":     0,
				},
			},
			"required": []string{"filePath"},
		},
		func(ctx context.Context, args string) (string, error) {
			var params struct {
				FilePath string `json:"filePath"`
				Offset   int    `json:"offset"`
				Limit    int    `json:"limit"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("read: invalid arguments: %w", err)
			}
			if params.FilePath == "" {
				return "", fmt.Errorf("read: filePath is required")
			}

			fi, err := fs.StatCtx(ctx, params.FilePath)
			if err != nil {
				return "", fmt.Errorf("read: %w", err)
			}

			if fi.IsDir {
				result, err := fs.ListRecursiveIgnoreCtx(ctx, params.FilePath, nil)
				if err != nil {
					return "", fmt.Errorf("read: %w", err)
				}
				return formatListResult(result), nil
			}

			if att, out, ok, err := readImageAttachment(ctx, fs, params.FilePath, fi.Size); err != nil {
				return "", err
			} else if ok {
				tool.AddAttachment(ctx, att)
				return out, nil
			}

			offset := params.Offset
			if offset < 1 {
				offset = 1
			}
			limit := params.Limit
			if limit <= 0 {
				limit = 2000
			}

			result, err := fs.ReadRangeCtx(ctx, params.FilePath, offset, limit)
			if err != nil {
				return "", fmt.Errorf("read: %w", err)
			}

			return formatReadOutput(result, offset, limit), nil
		},
	).SetPermissionError(isScopeError)
}

func formatReadOutput(vfsOutput string, offset, limit int) string {
	lines := strings.Split(vfsOutput, "\n")

	totalLines := 0
	contentStart := 0

	for i, line := range lines {
		if idx := strings.Index(line, "Lines: "); idx >= 0 {
			fmt.Sscanf(line[idx:], "Lines: %d", &totalLines)
		}
		if line == "---" {
			contentStart = i + 1
			break
		}
	}

	contentLines := lines[contentStart:]
	for len(contentLines) > 0 && contentLines[len(contentLines)-1] == "" {
		contentLines = contentLines[:len(contentLines)-1]
	}

	var diagnostics []string
	for i, line := range contentLines {
		if strings.TrimSpace(line) == "<file_diagnostics>" {
			diagnostics = contentLines[i:]
			contentLines = contentLines[:i]
			for len(contentLines) > 0 && contentLines[len(contentLines)-1] == "" {
				contentLines = contentLines[:len(contentLines)-1]
			}
			break
		}
	}

	var b strings.Builder
	b.WriteString("<file>\n")

	for _, line := range contentLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	shown := len(contentLines)
	endLine := offset + shown - 1
	if totalLines > 0 && endLine < totalLines {
		b.WriteString(fmt.Sprintf("\n(File has more lines. Use 'offset' parameter to read beyond line %d)\n", endLine))
	}

	b.WriteString("</file>")

	if len(diagnostics) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Join(diagnostics, "\n"))
	}

	return b.String()
}

func readImageAttachment(ctx context.Context, fs *vfs.VFS, path string, size int64) (llm.Attachment, string, bool, error) {
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if idx := strings.IndexByte(mediaType, ';'); idx >= 0 {
		mediaType = strings.TrimSpace(mediaType[:idx])
	}
	if _, ok := readImageMediaTypes[mediaType]; !ok {
		return llm.Attachment{}, "", false, nil
	}
	if size > maxReadImageBytes {
		return llm.Attachment{}, "", false, fmt.Errorf("read: image %q is too large to attach (%d bytes, max %d)", path, size, maxReadImageBytes)
	}
	data, rel, err := fs.ReadBytesCtx(ctx, path)
	if err != nil {
		return llm.Attachment{}, "", false, fmt.Errorf("read: %w", err)
	}
	detected := detectImageMediaType(data)
	if _, ok := readImageMediaTypes[detected]; !ok {
		return llm.Attachment{}, "", false, nil
	}
	out := fmt.Sprintf("<image>\nPath: %s\nMedia type: %s\nSize: %d bytes\nImage content is attached for visual inspection when the model supports image input.\n</image>", rel, detected, len(data))
	att := llm.Attachment{
		Type:      llm.AttachmentTypeImage,
		MediaType: detected,
		Data:      base64.StdEncoding.EncodeToString(data),
		FileName:  filepath.Base(rel),
		Path:      rel,
	}
	return att, out, true, nil
}

func detectImageMediaType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}
