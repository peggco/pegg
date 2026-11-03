package connect

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

const (
	fileChunkDefault = 256 << 10
	fileChunkMax     = 4 << 20
)

func trimSources(att llm.Attachment) llm.Attachment {
	att.Data = ""
	att.URL = ""
	return att
}

func (c *Connector) registerFileMethods() {
	c.dispatch.register("file.download", ScopeRead, c.handleFileDownload)
}

type fileDownloadParams struct {
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	Length    int    `json:"length"`
}

type fileChunk struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type,omitempty"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	Length    int    `json:"length"`
	Data      string `json:"data"`
	EOF       bool   `json:"eof"`
}

func (c *Connector) handleFileDownload(_ context.Context, req *Request) (any, error) {
	var p fileDownloadParams
	if err := req.Decode(&p); err != nil {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "invalid params: " + err.Error()}
	}
	if p.SessionID == "" {
		p.SessionID = req.SessionID
	}
	if p.SessionID == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "session_id is required"}
	}
	if strings.TrimSpace(p.Path) == "" {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "path is required"}
	}

	att, ok := c.deliveredFile(p.SessionID, p.Path)
	if !ok {
		Audit("file.download.denied", map[string]any{"session_id": p.SessionID, "path": p.Path})
		return nil, &FrameError{Code: ErrCodeForbidden, Message: "file was not delivered in this session"}
	}

	f, err := os.Open(att.Path)
	if err != nil {
		return nil, &FrameError{Code: ErrCodeNotFound, Message: "file not found: " + err.Error()}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, &FrameError{Code: ErrCodeInternal, Message: err.Error()}
	}
	if info.IsDir() {
		return nil, &FrameError{Code: ErrCodeBadRequest, Message: "path is a directory"}
	}

	size := info.Size()
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > size {
		offset = size
	}
	length := p.Length
	if length <= 0 {
		length = fileChunkDefault
	}
	if length > fileChunkMax {
		length = fileChunkMax
	}
	if remaining := size - offset; int64(length) > remaining {
		length = int(remaining)
	}

	buf := make([]byte, length)
	n, readErr := f.ReadAt(buf, offset)
	if readErr != nil && readErr != io.EOF {
		return nil, &FrameError{Code: ErrCodeInternal, Message: readErr.Error()}
	}
	name := att.FileName
	if name == "" {
		name = filepath.Base(att.Path)
	}
	chunk := fileChunk{
		Name:      name,
		MediaType: att.MediaType,
		Size:      size,
		Offset:    offset,
		Length:    n,
		Data:      base64.StdEncoding.EncodeToString(buf[:n]),
		EOF:       offset+int64(n) >= size,
	}
	Audit("file.download", map[string]any{
		"session_id": p.SessionID,
		"path":       att.Path,
		"offset":     offset,
		"length":     n,
		"size":       size,
	})
	return chunk, nil
}

func (c *Connector) rememberDelivered(sessionID string, atts []llm.Attachment) {
	if sessionID == "" || len(atts) == 0 {
		return
	}
	c.deliveredMu.Lock()
	defer c.deliveredMu.Unlock()
	if c.delivered == nil {
		c.delivered = make(map[string]map[string]llm.Attachment)
	}
	idx := c.delivered[sessionID]
	if idx == nil {
		idx = make(map[string]llm.Attachment)
		c.delivered[sessionID] = idx
	}
	for _, a := range atts {
		if a.Type != llm.AttachmentTypeFile || a.Path == "" {
			continue
		}
		a = trimSources(a)
		idx[a.Path] = a
		if virtual := c.virtualPath(a.Path); virtual != "" {
			idx[virtual] = a
		}
	}
}

func (c *Connector) deliveredFile(sessionID, path string) (llm.Attachment, bool) {
	c.deliveredMu.Lock()
	if idx := c.delivered[sessionID]; idx != nil {
		if att, ok := idx[path]; ok {
			c.deliveredMu.Unlock()
			return att, true
		}
		if abs := c.resolveDeliveredPath(path); abs != "" {
			if att, ok := idx[abs]; ok {
				c.deliveredMu.Unlock()
				return att, true
			}
		}
	}
	c.deliveredMu.Unlock()

	if c.deps.Sessions == nil {
		return llm.Attachment{}, false
	}
	msgs, err := c.deps.Sessions.Messages(sessionID)
	if err != nil {
		return llm.Attachment{}, false
	}
	var found llm.Attachment
	for _, m := range msgs {
		if m.Role != llm.RoleTool {
			continue
		}
		for _, a := range session.AttachmentsOf(m) {
			if a.Type != llm.AttachmentTypeFile || a.Path == "" {
				continue
			}
			if a.Path == path || c.virtualPath(a.Path) == path {
				found = a
			}
		}
	}
	if found.Path == "" {
		return llm.Attachment{}, false
	}
	c.rememberDelivered(sessionID, []llm.Attachment{found})
	return trimSources(found), true
}

func (c *Connector) resolveDeliveredPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if c.deps.VFS == nil {
		return ""
	}
	abs, err := c.deps.VFS.Resolve(path)
	if err != nil {
		return ""
	}
	return abs
}

func (c *Connector) virtualPath(abs string) string {
	if c.deps.VFS == nil || abs == "" {
		return ""
	}
	return c.deps.VFS.Virtual(abs)
}

func sanitizeMessageContent(content any) any {
	switch c := content.(type) {
	case llm.Content:
		if len(c.Attachments) == 0 {
			return content
		}
		trimmed := make([]llm.Attachment, len(c.Attachments))
		for i, att := range c.Attachments {
			trimmed[i] = trimSources(att)
		}
		c.Attachments = trimmed
		return c
	case map[string]any:
		raw, ok := c["attachments"]
		if !ok {
			return content
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			return content
		}
		clone := make(map[string]any, len(c))
		for k, v := range c {
			clone[k] = v
		}
		trimmed := make([]any, len(list))
		for i, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				trimmed[i] = item
				continue
			}
			cp := make(map[string]any, len(m))
			for k, v := range m {
				cp[k] = v
			}
			cp["data"] = ""
			cp["url"] = ""
			trimmed[i] = cp
		}
		clone["attachments"] = trimmed
		return clone
	default:
		return content
	}
}
