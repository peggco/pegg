package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/vfs"
)

const maxPluginAttachmentBytes = 25 * 1024 * 1024

type pluginTool struct {
	m          *Manager
	pluginName string
	spec       pkgplugin.ToolSpec
	fs         *vfs.VFS
}

func (t *pluginTool) Name() string {
	return t.spec.Name
}

func (t *pluginTool) Description() string {
	return t.spec.Description
}

func (t *pluginTool) Parameters() any {
	var params any
	if err := json.Unmarshal([]byte(t.spec.Parameters), &params); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if params == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return params
}

func (t *pluginTool) Execute(ctx context.Context, args string) (string, error) {
	res, err := t.m.ExecuteTool(ctx, t.pluginName, t.spec.Name, args)
	if err != nil {
		return "", err
	}
	return t.persist(ctx, res)
}

func (t *pluginTool) persist(ctx context.Context, res *pkgplugin.ToolResult) (string, error) {
	output := res.Output
	if len(res.Attachments) == 0 {
		return output, nil
	}
	if t.fs == nil {
		return output, fmt.Errorf("plugin tool %s: no workspace filesystem for attachments", t.spec.Name)
	}

	dir := filepath.ToSlash(filepath.Join(".pegg", "artifacts", "plugins", sanitizeName(t.pluginName)))
	for i, att := range res.Attachments {
		data, err := t.attachmentBytes(att)
		if err != nil {
			output = appendLine(output, fmt.Sprintf("attachment error: %v", err))
			continue
		}
		if len(data) == 0 {
			continue
		}

		name := sanitizeFileName(att.FileName)
		if name == "" {
			name = fmt.Sprintf("%s-%d%s", sanitizeFileName(t.spec.Name), time.Now().UnixNano()+int64(i), extensionForMediaType(att.MediaType))
		}
		saveName := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405.000000000"), name)
		rel := filepath.ToSlash(filepath.Join(dir, saveName))
		saved, err := t.fs.WriteCtx(ctx, rel, data)
		if err != nil {
			output = appendLine(output, fmt.Sprintf("save attachment: %v", err))
			continue
		}
		rel = saved

		mediaType := att.MediaType
		attType := strings.ToLower(att.Type)
		if mediaType == "" {
			mediaType = mediaTypeForExtension(filepath.Ext(name))
		}
		if attType == "" {
			if strings.HasPrefix(mediaType, "image/") {
				attType = "image"
			} else {
				attType = "file"
			}
		}

		if attType == "image" && strings.HasPrefix(mediaType, "image/") && len(data) <= 5*1024*1024 {
			tool.AddAttachment(ctx, llm.Attachment{
				Type:      llm.AttachmentTypeImage,
				MediaType: mediaType,
				Data:      base64.StdEncoding.EncodeToString(data),
				FileName:  name,
				Path:      rel,
			})
		} else {
			tool.AddAttachment(ctx, llm.Attachment{
				Type:      llm.AttachmentTypeFile,
				MediaType: mediaType,
				FileName:  name,
				Path:      rel,
				Size:      int64(len(data)),
			})
		}
		output = appendLine(output, fmt.Sprintf("Saved: %s (%d bytes)", rel, len(data)))
	}
	return output, nil
}

func (t *pluginTool) attachmentBytes(att pkgplugin.Attachment) ([]byte, error) {
	if att.Data != "" {
		data, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			return nil, fmt.Errorf("decode base64: %w", err)
		}
		if len(data) > maxPluginAttachmentBytes {
			return nil, fmt.Errorf("attachment exceeds %d bytes", maxPluginAttachmentBytes)
		}
		return data, nil
	}
	if att.Path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(att.Path)
	if err != nil {
		return nil, err
	}
	if len(data) > maxPluginAttachmentBytes {
		return nil, fmt.Errorf("attachment exceeds %d bytes", maxPluginAttachmentBytes)
	}
	if isTempPath(att.Path) {
		_ = os.Remove(att.Path)
	}
	return data, nil
}

func isTempPath(path string) bool {
	temp := filepath.Clean(os.TempDir())
	clean := filepath.Clean(path)
	return clean == temp || strings.HasPrefix(clean, temp+string(filepath.Separator))
}

func appendLine(out, line string) string {
	if out == "" {
		return line
	}
	return out + "\n" + line
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "plugin"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." || name == "/" || name == "" {
		return ""
	}
	return sanitizeName(strings.TrimSuffix(name, filepath.Ext(name))) + filepath.Ext(name)
}

func extensionForMediaType(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mediaType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "text/html":
		return ".html"
	case "text/plain":
		return ".txt"
	case "application/json":
		return ".json"
	case "text/csv":
		return ".csv"
	case "application/zip":
		return ".zip"
	default:
		return ".bin"
	}
}

func mediaTypeForExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".html":
		return "text/html"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
