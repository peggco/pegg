package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

func TestDrawFooterRightAlignsToRightEdge(t *testing.T) {
	styles.RegisterDefaults()
	styles.Set("dark")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 6)

	bounds := layout.Region{Left: 0, Top: 0, Width: 40, Height: 6}
	text := "r refresh · esc close"
	DrawFooterRight(screen, bounds, text)
	screen.Show()

	cells, width, _ := screen.GetContents()
	row := (bounds.Bottom() - 1) * width
	var line strings.Builder
	for x := 0; x < width; x++ {
		cell := cells[row+x]
		if len(cell.Runes) > 0 {
			line.WriteRune(cell.Runes[0])
		} else {
			line.WriteByte(' ')
		}
	}
	got := strings.TrimRight(line.String(), " ")
	wantStart := width - len([]rune(text))
	if !strings.HasSuffix(got, text) {
		t.Fatalf("footer not right aligned: %q", got)
	}
	if strings.TrimSpace(got[:wantStart]) != "" {
		t.Fatalf("footer should start at column %d: %q", wantStart, got)
	}
}
