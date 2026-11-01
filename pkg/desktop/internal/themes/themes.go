package themes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/peggco/pegg/internal/tui/styles"
)

type Theme struct {
	Name   string            `json:"name"`
	Label  string            `json:"label"`
	Dark   bool              `json:"dark"`
	Tokens map[string]string `json:"tokens"`
}

func All() []Theme {
	styles.RegisterDefaults()
	out := make([]Theme, 0, len(styles.Names()))
	for _, name := range styles.Names() {
		t, ok := styles.Get(name)
		if !ok {
			continue
		}
		out = append(out, fromTUI(name, t))
	}
	return out
}

func Find(name string) (Theme, bool) {
	for _, t := range All() {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

func fromTUI(name string, t styles.Theme) Theme {
	tokens := map[string]string{
		"--pg-bg":            hex(t.Background),
		"--pg-surface":       hex(t.Surface),
		"--pg-surface-2":     mix(t.Background, t.Foreground, 0.07),
		"--pg-text":          hex(t.Foreground),
		"--pg-text-2":        hex(t.TextDim),
		"--pg-muted":         hex(t.Muted),
		"--pg-accent":        hex(t.Accent),
		"--pg-accent-soft":   hex(t.AccentDim),
		"--pg-border":        hex(t.Border),
		"--pg-border-strong": mix(t.Border, t.Foreground, 0.18),
		"--pg-hover":         mix(t.Background, t.Foreground, 0.055),
		"--pg-active":        mix(t.Background, t.Accent, 0.16),
		"--pg-input-bg":      hex(t.InputBg),
		"--pg-input-text":    hex(t.InputText),
		"--pg-placeholder":   hex(t.Placeholder),
		"--pg-selection":     hex(t.Selection),
		"--pg-code-bg":       hex(t.CodeBg),
		"--pg-code-border":   hex(t.CodeBorder),
		"--pg-code-text":     hex(t.CodeText),
		"--pg-success":       hex(t.Success),
		"--pg-warning":       hex(t.Warning),
		"--pg-danger":        hex(t.Error),
		"--pg-running":       hex(t.Running),
		"--pg-chip":          hex(t.Chip),
		"--pg-mention":       hex(t.Mention),
		"--pg-user-bg":       mix(t.Background, t.Accent, 0.10),
		"--pg-user-label":    hex(t.UserLabel),
		"--pg-assistant":     hex(t.AssistantLabel),
		"--pg-reasoning":     hex(t.Reasoning),
		"--pg-subagent":      hex(t.Subagent),
		"--pg-thinking":      hex(t.ThinkingGlow),
		"--pg-syn-keyword":   hex(t.TokenKeyword),
		"--pg-syn-name":      hex(t.TokenName),
		"--pg-syn-function":  hex(t.TokenFunction),
		"--pg-syn-string":    hex(t.TokenString),
		"--pg-syn-number":    hex(t.TokenNumber),
		"--pg-syn-comment":   hex(t.TokenComment),
		"--pg-syn-operator":  hex(t.TokenOperator),
		"--pg-syn-punct":     hex(t.TokenPunctuation),
		"--pg-syn-type":      hex(t.TokenType),
		"--pg-syn-constant":  hex(t.TokenConstant),
		"--pg-syn-generic":   hex(t.TokenGeneric),
	}
	tokens["--pg-sidebar"] = hex(t.Background)
	tokens["--pg-shadow"] = "rgba(0,0,0,0.08)"
	tokens["--pg-accent-fg"] = "#ffffff"
	if isDark(t.Background) {
		tokens["--pg-shadow"] = "rgba(0,0,0,0.42)"
		tokens["--pg-accent-fg"] = "#0a0a0a"
	}
	applyDesktopPalette(name, tokens)
	return Theme{
		Name:   name,
		Label:  label(name),
		Dark:   isDark(t.Background),
		Tokens: tokens,
	}
}

func applyDesktopPalette(name string, tokens map[string]string) {
	switch name {
	case "dark":
		for key, value := range map[string]string{
			"--pg-bg":            "#0d0d0d",
			"--pg-sidebar":       "#0d0d0d",
			"--pg-surface":       "#141414",
			"--pg-surface-2":     "#1a1a1a",
			"--pg-text":          "#fafafa",
			"--pg-text-2":        "#d4d4d4",
			"--pg-muted":         "#a3a3a3",
			"--pg-accent":        "#fafafa",
			"--pg-accent-soft":   "#a3a3a3",
			"--pg-accent-fg":     "#0a0a0a",
			"--pg-border":        "#262626",
			"--pg-border-strong": "#3f3f3f",
			"--pg-hover":         "#1a1a1a",
			"--pg-active":        "#222222",
			"--pg-input-bg":      "#141414",
			"--pg-input-text":    "#fafafa",
			"--pg-placeholder":   "#a3a3a3",
			"--pg-selection":     "#404040",
			"--pg-code-bg":       "#141414",
			"--pg-code-border":   "#262626",
			"--pg-code-text":     "#e5e5e5",
			"--pg-user-bg":       "#1a1a1a",
			"--pg-shadow":        "rgba(0,0,0,0.42)",
		} {
			tokens[key] = value
		}
	case "light":
		for key, value := range map[string]string{
			"--pg-bg":            "#ffffff",
			"--pg-sidebar":       "#ffffff",
			"--pg-surface":       "#fafafa",
			"--pg-surface-2":     "#f5f5f5",
			"--pg-text":          "#0d0d0d",
			"--pg-text-2":        "#404040",
			"--pg-muted":         "#737373",
			"--pg-accent":        "#171717",
			"--pg-accent-soft":   "#404040",
			"--pg-accent-fg":     "#ffffff",
			"--pg-border":        "#e5e5e5",
			"--pg-border-strong": "#d4d4d4",
			"--pg-hover":         "#f5f5f5",
			"--pg-active":        "#ededed",
			"--pg-input-bg":      "#ffffff",
			"--pg-input-text":    "#0d0d0d",
			"--pg-placeholder":   "#737373",
			"--pg-selection":     "#d4d4d4",
			"--pg-code-bg":       "#fafafa",
			"--pg-code-border":   "#e5e5e5",
			"--pg-code-text":     "#0d0d0d",
			"--pg-user-bg":       "#f5f5f5",
			"--pg-shadow":        "rgba(0,0,0,0.08)",
		} {
			tokens[key] = value
		}
	}
}

func hex(c interface{ Hex() int32 }) string {
	v := c.Hex()
	return fmt.Sprintf("#%06x", v&0xFFFFFF)
}

func isDark(c interface{ Hex() int32 }) bool {
	v := c.Hex()
	r := float64((v>>16)&0xFF) / 255
	g := float64((v>>8)&0xFF) / 255
	b := float64(v&0xFF) / 255
	return 0.2126*r+0.7152*g+0.0722*b < 0.45
}

func mix(a, b interface{ Hex() int32 }, t float64) string {
	ar, ag, ab := rgb(a.Hex())
	br, bg, bb := rgb(b.Hex())
	return fmt.Sprintf("#%02x%02x%02x",
		int(ar+(br-ar)*t),
		int(ag+(bg-ag)*t),
		int(ab+(bb-ab)*t),
	)
}

func rgb(v int32) (float64, float64, float64) {
	return float64((v >> 16) & 0xFF), float64((v >> 8) & 0xFF), float64(v & 0xFF)
}

func label(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func Names() []string {
	styles.RegisterDefaults()
	out := append([]string(nil), styles.Names()...)
	sort.Strings(out)
	return out
}
