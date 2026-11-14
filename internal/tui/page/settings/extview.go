package settings

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/styles"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type extHitKind int

const (
	hitToolbar extHitKind = iota
	hitItem
	hitSetting
	hitAction
)

type extHit struct {
	kind    extHitKind
	section int
	item    int
	index   int
	action  string
	picker  string
	label   string
	detail  string
	fields  []pkgplugin.TUIField
	meta    map[string]string
}

type extSeg struct {
	text  string
	style tcell.Style
	hit   *extHit
}

type extRow struct {
	segs []extSeg
}

type extHitSpan struct {
	hit   *extHit
	start int
	end   int
}

type extDoc struct {
	rows  []extRow
	hits  []*extHit
	spans []extHitSpan
}

func (d *extDoc) spanFor(hit *extHit) (int, int, bool) {
	for _, s := range d.spans {
		if s.hit == hit {
			return s.start, s.end, true
		}
	}
	return 0, 0, false
}

var extToolbarActions = map[string]bool{
	"start":    true,
	"open":     true,
	"stop":     true,
	"sync":     true,
	"add":      true,
	"card.add": true,
	"enable":   true,
	"disable":  true,
	"reload":   true,
}

type extDocBuilder struct {
	width int
	doc   *extDoc
}

func newExtDocBuilder(width int) *extDocBuilder {
	if width < 20 {
		width = 20
	}
	return &extDocBuilder{width: width, doc: &extDoc{}}
}

func (b *extDocBuilder) addRow(segs []extSeg) {
	b.doc.rows = append(b.doc.rows, extRow{segs: segs})
	for _, s := range segs {
		if s.hit != nil {
			b.registerHit(s.hit)
		}
	}
}

func (b *extDocBuilder) registerHit(hit *extHit) {
	for _, h := range b.doc.hits {
		if h == hit {
			return
		}
	}
	b.doc.hits = append(b.doc.hits, hit)
}

func (b *extDocBuilder) blank() {
	b.addRow(nil)
}

func (b *extDocBuilder) finish() *extDoc {
	index := map[*extHit]int{}
	for i, row := range b.doc.rows {
		for _, s := range row.segs {
			if s.hit == nil {
				continue
			}
			if pos, ok := index[s.hit]; ok {
				b.doc.spans[pos].end = i
				continue
			}
			index[s.hit] = len(b.doc.spans)
			b.doc.spans = append(b.doc.spans, extHitSpan{hit: s.hit, start: i, end: i})
		}
	}
	return b.doc
}

func (b *extDocBuilder) addHeader(title, desc string) {
	th := styles.Current()
	if title != "" {
		b.addRow([]extSeg{{text: " " + components.TruncateTo(title, b.width-1), style: extTitleStyle(th)}})
	}
	if desc != "" {
		b.addRow([]extSeg{{text: " " + components.TruncateTo(desc, b.width-1), style: extDescStyle(th)}})
	}
	b.blank()
}

func (b *extDocBuilder) addToolbar(actions []pkgplugin.TUIAction) {
	th := styles.Current()
	var segs []extSeg
	x := 0
	for i := range actions {
		action := actions[i]
		label := " " + action.Label + " "
		w := components.DisplayWidth(label)
		if x > 0 && x+1+w > b.width {
			b.addRow(segs)
			segs = nil
			x = 0
		}
		if x > 0 {
			segs = append(segs, extSeg{text: " ", style: extBase(th)})
			x++
		}
		hit := &extHit{
			kind:   hitToolbar,
			index:  i,
			action: action.ID,
			picker: action.PickerField,
			label:  action.Label,
			detail: action.Description,
			fields: action.Fields,
		}
		segs = append(segs, extSeg{text: label, style: extChipStyle(th), hit: hit})
		x += w
	}
	if len(segs) > 0 {
		b.addRow(segs)
	}
	b.blank()
}

func (b *extDocBuilder) addTiles(metrics []pkgplugin.TUIMetric) {
	if len(metrics) == 0 {
		return
	}
	th := styles.Current()
	gap := 1
	cols := (b.width + gap) / 15
	if cols < 1 {
		cols = 1
	}
	if cols > len(metrics) {
		cols = len(metrics)
	}
	tileW := (b.width - (cols-1)*gap) / cols
	if tileW < 8 {
		tileW = 8
	}
	hasDetail := false
	for _, m := range metrics {
		if m.Detail != "" || m.Badge != "" {
			hasDetail = true
			break
		}
	}
	tileH := 3
	if hasDetail {
		tileH = 4
	}
	for start := 0; start < len(metrics); start += cols {
		end := start + cols
		if end > len(metrics) {
			end = len(metrics)
		}
		for r := 0; r < tileH; r++ {
			var segs []extSeg
			for i := start; i < end; i++ {
				if i > start {
					segs = append(segs, extSeg{text: strings.Repeat(" ", gap), style: extBase(th)})
				}
				segs = append(segs, tileRow(metrics[i], r, tileW, tileH, th)...)
			}
			b.addRow(segs)
		}
		b.blank()
	}
}

func tileRow(m pkgplugin.TUIMetric, r, w, h int, th styles.Theme) []extSeg {
	border := extBorderStyle(th)
	switch {
	case r == 0:
		label := strings.ToUpper(m.Label)
		inner := w - 5
		if inner < 0 {
			inner = 0
		}
		label = components.TruncateTo(label, inner)
		fill := inner - components.DisplayWidth(label)
		if fill < 0 {
			fill = 0
		}
		return []extSeg{{text: "┌─ " + label + " " + strings.Repeat("─", fill) + "┐", style: border}}
	case r == h-1:
		return []extSeg{{text: "└" + strings.Repeat("─", w-2) + "┘", style: border}}
	case r == 1:
		return tileContent(m.Value, extValueStyle(th), w, border)
	default:
		detail := m.Detail
		if detail == "" {
			detail = m.Badge
		}
		return tileContent(detail, extDescStyle(th), w, border)
	}
}

func tileContent(text string, style tcell.Style, w int, border tcell.Style) []extSeg {
	inner := w - 4
	if inner < 0 {
		inner = 0
	}
	text = components.TruncateTo(text, inner)
	pad := inner - components.DisplayWidth(text)
	if pad < 0 {
		pad = 0
	}
	return []extSeg{
		{text: "│ ", style: border},
		{text: text, style: style},
		{text: strings.Repeat(" ", pad) + " │", style: border},
	}
}

func (b *extDocBuilder) addSectionTitle(title, note string) {
	th := styles.Current()
	label := strings.ToUpper(title)
	segs := []extSeg{{text: label, style: extSectionStyle(th)}}
	if note != "" {
		labelW := components.DisplayWidth(label)
		noteW := components.DisplayWidth(note)
		pad := b.width - labelW - noteW - 1
		if pad > 0 {
			segs = append(segs, extSeg{text: strings.Repeat(" ", pad), style: extBase(th)})
			segs = append(segs, extSeg{text: note, style: extDescStyle(th)})
		}
	}
	b.addRow(segs)
}

func (b *extDocBuilder) addEmpty(text string) {
	th := styles.Current()
	b.addRow([]extSeg{{text: "  " + components.TruncateTo(text, b.width-2), style: extDescStyle(th)}})
}

func (b *extDocBuilder) addChannels(sectionIndex int, section pkgplugin.TUISection) {
	th := styles.Current()
	if section.Title != "" {
		b.addSectionTitle(section.Title, section.Note)
	}
	items := section.Items
	if len(items) == 0 {
		b.addEmpty("No channels configured.")
		b.blank()
		return
	}
	gap := 1
	cols := (b.width + gap) / 24
	if cols < 1 {
		cols = 1
	}
	if cols > len(items) {
		cols = len(items)
	}
	cardW := (b.width - (cols-1)*gap) / cols
	if cardW < 16 {
		cardW = 16
	}
	for start := 0; start < len(items); start += cols {
		end := start + cols
		if end > len(items) {
			end = len(items)
		}
		cards := make([][][]extSeg, 0, end-start)
		for i := start; i < end; i++ {
			item := items[i]
			hit := &extHit{
				kind:    hitItem,
				section: sectionIndex,
				item:    i,
				action:  item.Action,
				label:   item.Title,
				detail:  item.Detail,
				fields:  item.Fields,
				meta:    item.Meta,
			}
			content := [][]extSeg{}
			if item.Detail != "" {
				content = append(content, []extSeg{{text: item.Detail, style: extDescStyle(th)}})
			}
			if item.Badge != "" {
				content = append(content, []extSeg{{text: item.Badge, style: extBadgeStyle(th)}})
			}
			cards = append(cards, boxRows(item.Title, content, cardW, th, hit))
		}
		b.addComposed(cards, gap, th)
		b.blank()
	}
}

func (b *extDocBuilder) addList(sectionIndex int, section pkgplugin.TUISection) {
	th := styles.Current()
	if len(section.Items) == 0 {
		if section.Title != "" {
			b.addSectionTitle(section.Title, section.Note)
		}
		b.addEmpty("Nothing to show.")
		b.blank()
		return
	}
	content := make([][]extSeg, 0, len(section.Items)+1)
	if section.Note != "" {
		content = append(content, []extSeg{{text: section.Note, style: extDescStyle(th)}})
	}
	for i := range section.Items {
		item := section.Items[i]
		hit := &extHit{
			kind:    hitItem,
			section: sectionIndex,
			item:    i,
			action:  item.Action,
			label:   item.Title,
			detail:  item.Detail,
			fields:  item.Fields,
			meta:    item.Meta,
		}
		content = append(content, listRow(item.Title, item.Detail, item.Badge, b.width-4, th, hit))
	}
	b.addBox(strings.ToUpper(section.Title), content, b.width, th)
	b.blank()
}

func (b *extDocBuilder) addSettings(fields []pkgplugin.TUIField) {
	if len(fields) == 0 {
		return
	}
	th := styles.Current()
	content := make([][]extSeg, 0, len(fields))
	for i := range fields {
		field := fields[i]
		hit := &extHit{
			kind:   hitSetting,
			index:  i,
			label:  field.Label,
			detail: field.Value,
			fields: []pkgplugin.TUIField{field},
		}
		value := field.Value
		if len(field.Options) > 0 {
			value = "‹ " + value + " ›"
		}
		content = append(content, listRow(field.Label, value, "", b.width-4, th, hit))
	}
	b.addBox("SETTINGS", content, b.width, th)
	b.blank()
}

func (b *extDocBuilder) addActions(actions []pkgplugin.TUIAction) {
	if len(actions) == 0 {
		return
	}
	th := styles.Current()
	content := make([][]extSeg, 0, len(actions))
	for i := range actions {
		action := actions[i]
		hit := &extHit{
			kind:   hitAction,
			index:  i,
			action: action.ID,
			picker: action.PickerField,
			label:  action.Label,
			detail: action.Description,
			fields: action.Fields,
		}
		content = append(content, listRow(action.Label, action.Description, "", b.width-4, th, hit))
	}
	b.addBox("ACTIONS", content, b.width, th)
	b.blank()
}

func (b *extDocBuilder) addBox(title string, content [][]extSeg, w int, th styles.Theme) {
	rows := boxRows(title, content, w, th, nil)
	for _, row := range rows {
		b.addRow(row)
	}
}

func (b *extDocBuilder) addComposed(cards [][][]extSeg, gap int, th styles.Theme) {
	height := 0
	for _, card := range cards {
		if len(card) > height {
			height = len(card)
		}
	}
	for r := 0; r < height; r++ {
		var segs []extSeg
		for i, card := range cards {
			if i > 0 {
				segs = append(segs, extSeg{text: strings.Repeat(" ", gap), style: extBase(th)})
			}
			if r < len(card) {
				segs = append(segs, card[r]...)
			}
		}
		b.addRow(segs)
	}
}

func boxRows(title string, content [][]extSeg, w int, th styles.Theme, hit *extHit) [][]extSeg {
	border := extBorderStyle(th)
	titleStyle := extSectionStyle(th)
	if hit != nil {
		titleStyle = border
	}
	rows := [][]extSeg{boxTop(title, w, border, titleStyle, hit)}
	for _, row := range content {
		rows = append(rows, boxContent(row, w, border, hit))
	}
	rows = append(rows, boxBottom(w, border, hit))
	return rows
}

func boxTop(title string, w int, style, titleStyle tcell.Style, hit *extHit) []extSeg {
	if w < 2 {
		w = 2
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return []extSeg{{text: "┌" + strings.Repeat("─", w-2) + "┐", style: style, hit: hit}}
	}
	maxLabel := w - 5
	if maxLabel < 0 {
		maxLabel = 0
	}
	label := components.TruncateTo(title, maxLabel)
	fill := w - 5 - components.DisplayWidth(label)
	if fill < 0 {
		fill = 0
	}
	return []extSeg{
		{text: "┌─ ", style: style, hit: hit},
		{text: label, style: titleStyle, hit: hit},
		{text: " " + strings.Repeat("─", fill) + "┐", style: style, hit: hit},
	}
}

func boxBottom(w int, style tcell.Style, hit *extHit) []extSeg {
	if w < 2 {
		w = 2
	}
	return []extSeg{{text: "└" + strings.Repeat("─", w-2) + "┘", style: style, hit: hit}}
}

func boxContent(row []extSeg, w int, style tcell.Style, hit *extHit) []extSeg {
	if w < 4 {
		w = 4
	}
	inner := w - 4
	segs := []extSeg{{text: "│ ", style: style, hit: hit}}
	used := 0
	for _, s := range row {
		if used >= inner {
			break
		}
		text := s.text
		if used+components.DisplayWidth(text) > inner {
			text = components.TruncateTo(text, inner-used)
		}
		segHit := s.hit
		if segHit == nil {
			segHit = hit
		}
		segs = append(segs, extSeg{text: text, style: s.style, hit: segHit})
		used += components.DisplayWidth(text)
	}
	pad := inner - used
	if pad < 0 {
		pad = 0
	}
	segs = append(segs, extSeg{text: strings.Repeat(" ", pad) + " │", style: style, hit: hit})
	return segs
}

func listRow(label, detail, badge string, w int, th styles.Theme, hit *extHit) []extSeg {
	label = components.TruncateTo(label, w)
	labelW := components.DisplayWidth(label)
	segs := []extSeg{{text: label, style: extValueStyle(th), hit: hit}}
	right := detail
	if badge != "" {
		if right != "" {
			right = badge + " · " + right
		} else {
			right = badge
		}
	}
	if right == "" {
		return segs
	}
	rightW := components.DisplayWidth(right)
	pad := w - labelW - rightW
	if pad < 1 {
		right = components.TruncateTo(right, w-labelW-1)
		rightW = components.DisplayWidth(right)
		pad = w - labelW - rightW
	}
	if pad < 1 {
		return segs
	}
	segs = append(segs, extSeg{text: strings.Repeat(" ", pad), style: extBase(th), hit: hit})
	segs = append(segs, extSeg{text: right, style: extDescStyle(th), hit: hit})
	return segs
}

func extBase(th styles.Theme) tcell.Style {
	return th.Base().Background(th.InputBg)
}

func extTitleStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Foreground).Bold(true)
}

func extDescStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Muted)
}

func extValueStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Foreground)
}

func extBorderStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Border)
}

func extSectionStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Muted).Bold(true)
}

func extBadgeStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Hint)
}

func extChipStyle(th styles.Theme) tcell.Style {
	return extBase(th).Foreground(th.Foreground)
}

func buildExtDoc(page pkgplugin.TUIPage, width int) *extDoc {
	b := newExtDocBuilder(width)
	b.addHeader(page.Title, page.Description)

	var toolbar []pkgplugin.TUIAction
	var rest []pkgplugin.TUIAction
	for _, action := range page.Actions {
		if action.ID == page.SaveAction {
			continue
		}
		if extToolbarActions[action.ID] {
			toolbar = append(toolbar, action)
		} else {
			rest = append(rest, action)
		}
	}
	b.addToolbar(toolbar)
	b.addTiles(page.Metrics)

	for i, section := range page.Sections {
		switch section.Kind {
		case "board":
			continue
		case "channels":
			b.addChannels(i, section)
		default:
			b.addList(i, section)
		}
	}

	b.addSettings(page.Settings)
	b.addActions(rest)
	return b.finish()
}
