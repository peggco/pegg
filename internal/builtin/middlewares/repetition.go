package middlewares

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	repetitionCheckInterval = 24
	maxWordUnitWidth        = 48
	maxCharPeriod           = 16
	minCharPeriodTail       = 64
	repetitionPreviewChars  = 120

	defaultLoopRepeatThreshold = 5
	defaultLoopMinUnitChars    = 12
	defaultLoopMaxUnitChars    = 400
	defaultLoopWindowChars     = 8192
	contentRepeatExtra         = 2
)

type repetitionConfig struct {
	MinUnitChars int
	MaxUnitChars int
	MinRepeats   int
	WindowChars  int
}

type repetitionHit struct {
	Source  string
	Kind    string
	Unit    string
	Repeats int
}

func normalizeRepetitionConfig(cfg repetitionConfig) repetitionConfig {
	if cfg.MinUnitChars <= 0 {
		cfg.MinUnitChars = defaultLoopMinUnitChars
	}
	if cfg.MaxUnitChars < cfg.MinUnitChars {
		cfg.MaxUnitChars = defaultLoopMaxUnitChars
	}
	if cfg.MinRepeats <= 0 {
		cfg.MinRepeats = defaultLoopRepeatThreshold
	}
	if cfg.WindowChars <= 0 {
		cfg.WindowChars = defaultLoopWindowChars
	}
	return cfg
}

type repetitionDetector struct {
	cfg        repetitionConfig
	source     string
	buf        []rune
	sinceCheck int
}

func newRepetitionDetector(source string, cfg repetitionConfig) *repetitionDetector {
	return &repetitionDetector{cfg: normalizeRepetitionConfig(cfg), source: source}
}

func (d *repetitionDetector) Append(s string) (repetitionHit, bool) {
	if s == "" {
		return repetitionHit{}, false
	}
	d.buf = append(d.buf, []rune(s)...)
	if len(d.buf) > d.cfg.WindowChars {
		d.buf = append(d.buf[:0], d.buf[len(d.buf)-d.cfg.WindowChars:]...)
	}
	d.sinceCheck += utf8.RuneCountInString(s)
	if d.sinceCheck < repetitionCheckInterval && !endsWithBoundary(s) {
		return repetitionHit{}, false
	}
	d.sinceCheck = 0
	return detectRepeatedText(d.source, string(d.buf), d.cfg)
}

func (d *repetitionDetector) Reset() {
	d.buf = d.buf[:0]
	d.sinceCheck = 0
}

func endsWithBoundary(s string) bool {
	s = strings.TrimRight(s, " \t\r")
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '\n', '.', '!', '?', ';':
		return true
	}
	return false
}

func detectRepeatedText(source, text string, cfg repetitionConfig) (repetitionHit, bool) {
	cfg = normalizeRepetitionConfig(cfg)
	if strings.TrimSpace(text) == "" {
		return repetitionHit{}, false
	}
	if hit, ok := detectRepeatedLines(source, text, cfg); ok {
		return hit, true
	}
	if hit, ok := detectRepeatedSentences(source, text, cfg); ok {
		return hit, true
	}
	if hit, ok := detectRepeatedWords(source, text, cfg); ok {
		return hit, true
	}
	if hit, ok := detectRepeatedChars(source, text, cfg); ok {
		return hit, true
	}
	return repetitionHit{}, false
}

func detectRepeatedLines(source, text string, cfg repetitionConfig) (repetitionHit, bool) {
	if !strings.Contains(text, "\n") {
		return repetitionHit{}, false
	}
	return repeatedUnitsRun(source, "line", strings.Split(text, "\n"), cfg)
}

func detectRepeatedSentences(source, text string, cfg repetitionConfig) (repetitionHit, bool) {
	return repeatedUnitsRun(source, "sentence", splitSentences(text), cfg)
}

func splitSentences(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		s := strings.TrimSpace(b.String())
		if s != "" {
			out = append(out, s)
		}
		b.Reset()
	}
	for _, r := range text {
		switch r {
		case '\n':
			flush()
		case '.', '!', '?', ';':
			b.WriteRune(r)
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return out
}

func repeatedUnitsRun(source, kind string, units []string, cfg repetitionConfig) (repetitionHit, bool) {
	for len(units) > 0 && normalizeUnit(units[0]) == "" {
		units = units[1:]
	}
	for len(units) > 0 && normalizeUnit(units[len(units)-1]) == "" {
		units = units[:len(units)-1]
	}
	if len(units) < cfg.MinRepeats {
		return repetitionHit{}, false
	}

	bestCount := 0
	bestIndex := -1
	for i := 0; i < len(units); {
		norm := normalizeUnit(units[i])
		j := i
		for j+1 < len(units) && normalizeUnit(units[j+1]) == norm {
			j++
		}
		count := j - i + 1
		if count > bestCount && meaningfulUnit(norm, cfg) && utf8.RuneCountInString(norm) <= cfg.MaxUnitChars {
			bestCount = count
			bestIndex = i
		}
		i = j + 1
	}
	if bestCount < cfg.MinRepeats {
		return repetitionHit{}, false
	}
	return repetitionHit{Source: source, Kind: kind, Unit: previewUnit(units[bestIndex]), Repeats: bestCount}, true
}

func detectRepeatedWords(source, text string, cfg repetitionConfig) (repetitionHit, bool) {
	words := strings.Fields(strings.ToLower(text))
	n := len(words)
	if n < cfg.MinRepeats {
		return repetitionHit{}, false
	}
	maxWidth := n / cfg.MinRepeats
	if maxWidth > maxWordUnitWidth {
		maxWidth = maxWordUnitWidth
	}
	for width := 1; width <= maxWidth; width++ {
		unit := words[n-width:]
		unitText := strings.Join(unit, " ")
		if !meaningfulWordUnit(unit, unitText, cfg) {
			continue
		}
		if len(unitText) > cfg.MaxUnitChars {
			continue
		}
		if n < 2*width || words[n-1] != words[n-1-width] {
			continue
		}
		count := 1
		for start := n - 2*width; start >= 0; start -= width {
			if !equalWords(words[start:start+width], unit) {
				break
			}
			count++
		}
		if count >= cfg.MinRepeats {
			return repetitionHit{Source: source, Kind: "words", Unit: previewUnit(unitText), Repeats: count}, true
		}
	}
	return repetitionHit{}, false
}

func detectRepeatedChars(source, text string, cfg repetitionConfig) (repetitionHit, bool) {
	runes := []rune(strings.ToLower(text))
	n := len(runes)
	if n < minCharPeriodTail || n < cfg.MinRepeats {
		return repetitionHit{}, false
	}
	for period := 1; period <= maxCharPeriod; period++ {
		if n < period*cfg.MinRepeats {
			break
		}
		unit := runes[n-period:]
		unitText := string(unit)
		if strings.ContainsAny(unitText, " \t\n\r") {
			continue
		}
		if !meaningfulUnit(unitText, cfg) {
			continue
		}
		if len(unitText) > cfg.MaxUnitChars {
			continue
		}
		count := 1
		for start := n - 2*period; start >= 0; start -= period {
			if !equalRunes(runes[start:start+period], unit) {
				break
			}
			count++
		}
		if count >= cfg.MinRepeats {
			return repetitionHit{Source: source, Kind: "chars", Unit: previewUnit(unitText), Repeats: count}, true
		}
	}
	return repetitionHit{}, false
}

func meaningfulUnit(s string, cfg repetitionConfig) bool {
	if utf8.RuneCountInString(s) < cfg.MinUnitChars {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func meaningfulWordUnit(words []string, text string, cfg repetitionConfig) bool {
	if !meaningfulUnit(text, cfg) {
		return false
	}
	if len(words) == 1 {
		return true
	}
	seen := make(map[string]struct{}, len(words))
	for _, w := range words {
		seen[w] = struct{}{}
	}
	return len(seen) > 1
}

func normalizeUnit(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func previewUnit(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > repetitionPreviewChars {
		return string(runes[:repetitionPreviewChars]) + "…"
	}
	return s
}

func equalWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatRepetitionHit(hit repetitionHit) string {
	return fmt.Sprintf("repetitive %s output detected: the same %s repeated %d times (%q)",
		hit.Source, repetitionUnitLabel(hit), hit.Repeats, hit.Unit)
}

func repetitionUnitLabel(hit repetitionHit) string {
	switch hit.Kind {
	case "words":
		return fmt.Sprintf("%d-word unit", len(strings.Fields(hit.Unit)))
	case "line":
		return "line"
	case "sentence":
		return "sentence"
	case "chars":
		return "character block"
	default:
		return "unit"
	}
}
