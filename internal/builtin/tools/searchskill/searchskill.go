package searchskill

import (
	"context"
	"fmt"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/catalog"
)

const (
	defaultLimit  = 10
	maxLimit      = 20
	searchTimeout = 20 * time.Second
)

func generateSearchSkillToolPrompt() (string, error) {
	sys, err := searchSkillPromptBuilder().
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}

func SearchSkillTool() {
	tools.Register(searchSkillTool(catalog.NewClient()))
}

func searchSkillTool(client *catalog.Client) tool.Tool {
	prompt, err := generateSearchSkillToolPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to generate searchskill tool prompt: %v", err))
	}

	return tool.NewSpec(
		"searchskill",
		prompt,
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query describing the capability you need. Examples: 'docx word document', 'xlsx spreadsheet', 'pptx slides', 'pdf forms', 'frontend design'.",
				},
				"maxResults": map[string]any{
					"type":        "integer",
					"description": "Maximum number of skills to return (default 10, max 20).",
					"minimum":     1,
					"maximum":     maxLimit,
				},
			},
			"required": []string{"query"},
		},
		func(ctx context.Context, args string) (string, error) {
			var params struct {
				Query      string `json:"query"`
				MaxResults int    `json:"maxResults"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("searchskill: invalid arguments: %w", err)
			}
			params.Query = strings.TrimSpace(params.Query)
			if params.Query == "" {
				return "", fmt.Errorf("searchskill: query is required")
			}
			if params.MaxResults <= 0 {
				params.MaxResults = defaultLimit
			}
			if params.MaxResults > maxLimit {
				params.MaxResults = maxLimit
			}
			if client == nil {
				return "", fmt.Errorf("searchskill: hub client is unavailable")
			}

			ctx, cancel := context.WithTimeout(ctx, searchTimeout)
			defer cancel()

			res, err := client.Search(ctx, catalog.KindSkill, catalog.SearchOptions{Query: params.Query, Limit: params.MaxResults})
			if err != nil {
				return "", fmt.Errorf("searchskill: %w", err)
			}
			return renderResults(params.Query, res.Items), nil
		},
	)
}

func renderResults(query string, items []catalog.RemoteSkill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<skill_results query=\"%s\" count=\"%d\">\n", escapeXML(query), len(items))
	for _, item := range items {
		fmt.Fprintf(&b, "  <result>\n")
		fmt.Fprintf(&b, "    <name>%s</name>\n", escapeXML(item.Name))
		fmt.Fprintf(&b, "    <skill_id>%s</skill_id>\n", escapeXML(item.SkillID))
		fmt.Fprintf(&b, "    <source>%s</source>\n", escapeXML(item.Source))
		fmt.Fprintf(&b, "    <ref>%s</ref>\n", escapeXML(item.Source+"/"+item.SkillID))
		if item.URL != "" {
			fmt.Fprintf(&b, "    <url>%s</url>\n", escapeXML(item.URL))
		}
		if item.License != "" {
			fmt.Fprintf(&b, "    <license>%s</license>\n", escapeXML(item.License))
		}
		fmt.Fprintf(&b, "    <official>%t</official>\n", item.Official)
		fmt.Fprintf(&b, "  </result>\n")
	}
	fmt.Fprintf(&b, "</skill_results>\n")
	return b.String()
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

func escapeXML(s string) string {
	return xmlEscaper.Replace(s)
}
