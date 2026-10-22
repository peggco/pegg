package llm

import (
	"fmt"
	"math"
	"strconv"
)

func (u *Usage) ContextPercent(maxInputTokens int) float64 {
	if u == nil || maxInputTokens <= 0 {
		return 0
	}
	return float64(u.TotalTokens) / float64(maxInputTokens) * 100
}

func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}

func FormatContextUsage(u Usage, maxInputTokens int) string {
	pct := u.ContextPercent(maxInputTokens)
	if pct <= 0 {
		return FormatTokens(u.TotalTokens)
	}
	return fmt.Sprintf("%s (%d%%)", FormatTokens(u.TotalTokens), int(math.Round(pct)))
}
