package workspace

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	MaxTextFile   = 2 << 20
	MaxAttachment = 8 << 20
)

type Attachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Path      string `json:"path,omitempty"`
	Data      string `json:"data,omitempty"`
	Text      string `json:"text,omitempty"`
}

func Valid(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("workspace path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("workspace not found: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("workspace path is not a directory")
	}
	return abs, nil
}

func Name(path string) string {
	base := filepath.Base(filepath.Clean(path))
	if base == "." || base == string(filepath.Separator) {
		return path
	}
	return base
}

func ReadTextFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("path is a directory")
	}
	if info.Size() > MaxTextFile {
		return "", errors.New("file exceeds 2 MiB")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", errors.New("file is not valid UTF-8")
	}
	return string(raw), nil
}

func ReadAttachment(path string) (Attachment, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Attachment{}, err
	}
	if info.IsDir() {
		return Attachment{}, errors.New("path is a directory")
	}
	if info.Size() > MaxAttachment {
		return Attachment{}, errors.New("file exceeds 8 MiB")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, err
	}
	mediaType := byExtension(path)
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	att := Attachment{
		ID:        fmt.Sprintf("file-%d", len(raw)),
		Name:      filepath.Base(path),
		MediaType: mediaType,
		Size:      len(raw),
		Path:      path,
	}
	if isText(mediaType, raw) {
		att.Text = string(raw)
	} else {
		att.Data = base64.StdEncoding.EncodeToString(raw)
	}
	return att, nil
}

func byExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if t := mime.TypeByExtension(ext); t != "" {
		return strings.Split(t, ";")[0]
	}
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".c", ".h", ".cpp", ".hpp", ".rb", ".php", ".swift", ".cs", ".sh", ".bash", ".zsh", ".sql", ".css", ".scss", ".html", ".xml", ".yaml", ".yml", ".toml", ".ini", ".md", ".txt", ".json":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	}
	return ""
}

func isText(mediaType string, raw []byte) bool {
	if strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml" {
		return utf8.Valid(raw)
	}
	return false
}
