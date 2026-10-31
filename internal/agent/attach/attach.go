package attach

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
)

func Root() string { return filepath.Join(os.TempDir(), config.AppName) }

func IsStorePath(abs string) bool {
	root := filepath.Clean(Root())
	clean := filepath.Clean(abs)
	return clean == root || strings.HasPrefix(clean, root+string(filepath.Separator))
}

func Kind(mediaType string) llm.AttachmentType {
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return llm.AttachmentTypeImage
	case strings.HasPrefix(mediaType, "audio/"):
		return llm.AttachmentTypeAudio
	case strings.HasPrefix(mediaType, "video/"):
		return llm.AttachmentTypeVideo
	default:
		return llm.AttachmentTypeFile
	}
}

func MediaType(fileName string, _ []byte) string {
	if ext := strings.ToLower(filepath.Ext(fileName)); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			if idx := strings.IndexByte(mt, ';'); idx >= 0 {
				mt = strings.TrimSpace(mt[:idx])
			}
			return mt
		}
	}
	return "application/octet-stream"
}

func Save(fileName string, data []byte) (string, error) {
	if err := os.MkdirAll(Root(), 0o755); err != nil {
		return "", fmt.Errorf("attach: create store: %w", err)
	}
	name := sanitize(fileName)
	if name == "" {
		name = "file"
	}

	saveMu.Lock()
	defer saveMu.Unlock()
	path := uniquePath(name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("attach: write: %w", err)
	}
	return path, nil
}

var saveMu sync.Mutex

func uniquePath(name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	path := filepath.Join(Root(), name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(Root(), fmt.Sprintf("%s-%d%s", base, i, ext))
	}
}

func FromBytes(fileName string, data []byte) (llm.Attachment, error) {
	path, err := Save(fileName, data)
	if err != nil {
		return llm.Attachment{}, err
	}
	mediaType := MediaType(fileName, data)
	return llm.Attachment{
		Type:      Kind(mediaType),
		MediaType: mediaType,
		Data:      base64.StdEncoding.EncodeToString(data),
		Path:      path,
		FileName:  displayName(fileName),
	}, nil
}

func FromBase64(fileName, payload string) (llm.Attachment, error) {
	data, err := decodeBase64(payload)
	if err != nil {
		return llm.Attachment{}, err
	}
	return FromBytes(fileName, data)
}

func FromPath(src string) (llm.Attachment, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return llm.Attachment{}, fmt.Errorf("attach: read %q: %w", src, err)
	}
	return FromBytes(filepath.Base(src), data)
}

func MediaAttachments(atts []llm.Attachment, model llm.Model) []llm.Attachment {
	var allowed map[string]bool
	if model.Config != nil && model.Config.Modalities != nil {
		allowed = make(map[string]bool, len(model.Config.Modalities.Input))
		for _, modality := range model.Config.Modalities.Input {
			allowed[strings.ToLower(modality)] = true
		}
	}
	out := make([]llm.Attachment, 0, len(atts))
	for _, a := range atts {
		kind := string(a.Type)
		if allowed != nil && !allowed[kind] {
			continue
		}
		switch a.Type {
		case llm.AttachmentTypeImage, llm.AttachmentTypeAudio, llm.AttachmentTypeVideo:
			out = append(out, a)
		}
	}
	return out
}

const (
	BlockOpen  = "<attachments>"
	BlockClose = "</attachments>"
)

func Block(atts []llm.Attachment) string {
	lines := make([]string, 0, len(atts))
	for _, a := range atts {
		if a.Path == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			`<attachment name="%s" media_type="%s" size="%d" path="%s"/>`,
			xmlAttr(a.FileName), xmlAttr(a.MediaType), attachmentSize(a), xmlAttr(a.Path),
		))
	}
	if len(lines) == 0 {
		return ""
	}
	return BlockOpen + "\n" + strings.Join(lines, "\n") + "\n" + BlockClose
}

func StripBlock(text string) string {
	for {
		start := strings.Index(text, BlockOpen)
		if start < 0 {
			break
		}
		rest := text[start+len(BlockOpen):]
		end := strings.Index(rest, BlockClose)
		if end < 0 {
			text = text[:start]
			break
		}
		text = text[:start] + rest[end+len(BlockClose):]
	}
	return strings.TrimSpace(text)
}

func StripMessageContent(role llm.Role, content any) any {
	if role != llm.RoleUser {
		return content
	}
	switch c := content.(type) {
	case string:
		return StripBlock(c)
	case llm.Content:
		c.Text = StripBlock(c.Text)
		return c
	case []any:
		out := make([]any, 0, len(c))
		for _, item := range c {
			m, ok := item.(map[string]any)
			if !ok {
				out = append(out, item)
				continue
			}
			text, isText := m["text"].(string)
			if kind, _ := m["type"].(string); !isText || kind != "text" {
				out = append(out, item)
				continue
			}
			clone := make(map[string]any, len(m))
			for k, v := range m {
				clone[k] = v
			}
			clone["text"] = StripBlock(text)
			out = append(out, clone)
		}
		return out
	}
	return content
}

func attachmentSize(a llm.Attachment) int {
	if a.Data == "" {
		return 0
	}
	if n, err := base64.StdEncoding.DecodeString(a.Data); err == nil {
		return len(n)
	}
	return 0
}

func xmlAttr(value string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(value)
}

func decodeBase64(payload string) ([]byte, error) {
	if idx := strings.Index(payload, ";base64,"); idx >= 0 {
		payload = payload[idx+len(";base64,"):]
	}
	payload = strings.TrimSpace(payload)
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("attach: decode base64: %w", err)
	}
	return data, nil
}

func displayName(fileName string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "file"
	}
	return name
}

func sanitize(fileName string) string {
	name := displayName(fileName)
	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return -1
		case unicode.IsSpace(r):
			return '_'
		case unicode.IsControl(r):
			return -1
		default:
			return r
		}
	}, name)
	name = strings.Trim(name, "._")
	return name
}
