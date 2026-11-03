package sendfile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/attach"
	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/vfs"
)

const (
	MaxFiles            = 20
	MaxFileBytes  int64 = 200 << 20
	MaxTotalBytes int64 = 500 << 20
)

type params struct {
	Files   []string `json:"files"`
	Message string   `json:"message"`
}

type delivered struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}

func generateSendFileToolPrompt() (string, error) {
	sys, err := sendFileToolPromptBuilder().
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}

func SendFileTools(fs *vfs.VFS) {
	if sent := sendFileTool(fs); sent != nil {
		tools.Register(sent)
	}
}

func sendFileTool(fs *vfs.VFS) tool.Tool {
	sys, err := generateSendFileToolPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to generate sendfile tool prompt: %v", err))
	}

	return tool.NewSpec(
		"sendfile",
		sys,
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"files": map[string]any{
					"type":        "array",
					"description": "Paths of the files to send to the user.",
					"items":       map[string]any{"type": "string"},
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Optional short note shown to the user together with the files.",
				},
			},
			"required": []string{"files"},
		},
		func(ctx context.Context, args string) (string, error) {
			var p params
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return "", fmt.Errorf("sendfile: invalid arguments: %w", err)
			}
			return execute(ctx, fs, p)
		},
	).SetPermissionError(isScopeError)
}

func isScopeError(err error) bool {
	return errors.Is(err, vfs.ErrOutOfBounds)
}

func execute(ctx context.Context, fs *vfs.VFS, p params) (string, error) {
	paths := normalizePaths(p.Files)
	if len(paths) == 0 {
		return "", fmt.Errorf("sendfile: files is required")
	}
	if len(paths) > MaxFiles {
		return "", fmt.Errorf("sendfile: too many files (%d), maximum is %d", len(paths), MaxFiles)
	}

	seen := make(map[string]bool, len(paths))
	out := make([]delivered, 0, len(paths))
	var total int64

	for _, path := range paths {
		info, err := fs.StatCtx(ctx, path)
		if err != nil {
			return "", fmt.Errorf("sendfile: %s: %w", path, err)
		}
		if info.IsDir {
			return "", fmt.Errorf("sendfile: %s is a directory; archive it first", path)
		}
		if info.Size <= 0 {
			return "", fmt.Errorf("sendfile: %s is empty", path)
		}
		if info.Size > MaxFileBytes {
			return "", fmt.Errorf("sendfile: %s is too large (%s, max %s)", path, formatBytes(info.Size), formatBytes(MaxFileBytes))
		}
		abs, err := fs.ResolveCtx(ctx, path)
		if err != nil {
			return "", fmt.Errorf("sendfile: %s: %w", path, err)
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true

		total += info.Size
		if total > MaxTotalBytes {
			return "", fmt.Errorf("sendfile: total size exceeds %s", formatBytes(MaxTotalBytes))
		}

		name := info.Name
		if name == "" {
			name = path
		}
		mediaType := attach.MediaType(name, nil)
		tool.AddAttachment(ctx, llm.Attachment{
			Type:      llm.AttachmentTypeFile,
			MediaType: mediaType,
			FileName:  name,
			Path:      abs,
			Size:      info.Size,
		})
		out = append(out, delivered{
			Name:      name,
			Path:      info.Path,
			Size:      info.Size,
			MediaType: mediaType,
		})
	}

	if len(out) == 0 {
		return "", fmt.Errorf("sendfile: no files to send")
	}
	return formatResult(out, p.Message, total), nil
}

func normalizePaths(input []string) []string {
	out := make([]string, 0, len(input))
	for _, raw := range input {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if strings.ContainsRune(path, 0) {
			continue
		}
		out = append(out, path)
	}
	return out
}

func formatResult(files []delivered, message string, total int64) string {
	var b strings.Builder
	if note := strings.TrimSpace(message); note != "" {
		b.WriteString(note)
		b.WriteString("\n\n")
	}
	fmt.Fprintf(&b, "Sent %d file(s) to the user (%s total):\n", len(files), formatBytes(total))
	for _, f := range files {
		fmt.Fprintf(&b, "- %s (%s, %s)\n", f.Path, f.MediaType, formatBytes(f.Size))
	}
	b.WriteString("\nThe user can open or download these files from the chat. Do not repeat their contents in your reply.")
	return b.String()
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTP"[exp])
}
