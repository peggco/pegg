package connect

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peggco/pegg/internal/agent/agents"
)

const mentionsDefaultLimit = 2000

func (c *Connector) registerMentionMethods() {
	c.dispatch.register("mentions.list", ScopeSessionsRead, c.handleMentionsList)
}

type mentionItem struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Kind   string `json:"kind"`
}

type mentionsListParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (c *Connector) handleMentionsList(_ context.Context, req *Request) (any, error) {
	var p mentionsListParams
	if req != nil {
		_ = req.Decode(&p)
	}

	limit := p.Limit
	if limit <= 0 || limit > mentionsDefaultLimit {
		limit = mentionsDefaultLimit
	}

	items := make([]mentionItem, 0, 64)

	for _, name := range agents.List() {
		desc := ""
		if a, err := agents.New(name); err == nil && a != nil {
			desc = a.Description
		}
		items = append(items, mentionItem{Label: name, Detail: desc, Kind: "agent"})
	}

	if fs := c.deps.VFS; fs != nil {
		if paths, err := fs.Glob("**", ""); err == nil {
			sort.Strings(paths)
			root := fs.Root()
			for _, rel := range paths {
				kind := "file"
				if fi, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr == nil && fi.IsDir() {
					kind = "folder"
				}
				items = append(items, mentionItem{Label: rel, Detail: kind, Kind: kind})
			}
		}
	}

	if q := strings.TrimSpace(p.Query); q != "" {
		items = filterMentions(items, q)
	}

	truncated := false
	if len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return map[string]any{"items": items, "truncated": truncated}, nil
}

func filterMentions(items []mentionItem, query string) []mentionItem {
	q := strings.ToLower(query)
	out := make([]mentionItem, 0, len(items))
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Label), q) {
			out = append(out, it)
		}
	}
	return out
}
