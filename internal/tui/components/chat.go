package components

import (
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

func NewChat() *Chat {
	return &Chat{
		autoScroll:      true,
		lastWidth:       -1,
		follow:          true,
		itemCursor:      -1,
		hscroll:         map[*ChatItem]int{},
		itemMaxWidth:    map[*ChatItem]int{},
		itemTableBounds: map[*ChatItem][]int{},
	}
}

func (c *Chat) SetItems(items []*ChatItem) {
	c.items = items
	c.cache = make([][]Line, len(items))
	c.sel()
	c.autoScroll = true
	c.follow = true
	c.lastWidth = -1
	c.flatRev = -1
	c.dirty = true
	c.itemCursor = -1
	c.selActive = false
	c.resetTableScroll()
}

func (c *Chat) sel() int {
	return len(c.items) - 1
}

func (c *Chat) AppendItem(it *ChatItem) {
	c.items = append(c.items, it)
	c.cache = append(c.cache, nil)
	if c.wasAtBottom {
		c.autoScroll = true
		c.follow = true
	}
	c.dirty = true
}

func (c *Chat) PrependItems(items []*ChatItem) {
	if len(items) == 0 {
		return
	}
	added := 0
	for _, it := range items {
		added += len(c.itemLines(it, c.lastWidth))
	}
	c.ensureCache()
	c.items = append(items, c.items...)
	c.cache = append(make([][]Line, len(items)), c.cache...)
	c.scroll += added
	c.autoScroll = false
	c.follow = false
	c.dirty = true
}

func (c *Chat) Clear() {
	c.items = nil
	c.cache = nil
	c.cacheWidth = 0
	c.scroll = 0
	c.autoScroll = true
	c.follow = true
	c.lastWidth = -1
	c.flatRev = -1
	c.dirty = true
	c.itemCursor = -1
	c.flat = nil
	c.flatItems = nil
	c.resetTableScroll()
}

func (c *Chat) HasItems() bool { return len(c.items) > 0 }

func (c *Chat) Items() []*ChatItem { return c.items }

func (c *Chat) SetBack(b bool) { c.back = b }

func (c *Chat) HasBack() bool { return c.back }

func (c *Chat) SetHasMore(b bool) { c.hasMore = b }

func (c *Chat) HasMore() bool { return c.hasMore }

func (c *Chat) SetOnActivate(fn func(*ChatItem)) { c.onActivate = fn }

func (c *Chat) SetOnOpenFile(fn func(path string)) { c.onOpenFile = fn }

func (c *Chat) SetOnBack(fn func()) { c.onBack = fn }

func (c *Chat) SetOnLoadMore(fn func()) { c.onLoadMore = fn }

func (c *Chat) SetOnSubagentHistory(fn func(string)) { c.onSubagentHistory = fn }

func (c *Chat) Invalidate() {
	c.cache = nil
	c.cacheWidth = 0
	c.dirty = true
	c.lastWidth = -1
	c.flatRev = -1
}

func (c *Chat) ensureCache() {
	if c.cache == nil || len(c.cache) != len(c.items) {
		c.cache = make([][]Line, len(c.items))
	}
}

func (c *Chat) MarkLastDirty() {
	if len(c.items) == 0 {
		return
	}
	c.ensureCache()
	c.cache[len(c.items)-1] = nil
	c.dirty = true
}

func (c *Chat) InvalidateItem(idx int) {
	if idx < 0 || idx >= len(c.items) {
		return
	}
	c.ensureCache()
	c.cache[idx] = nil
	c.dirty = true
}

func (c *Chat) InvalidateItemPtr(it *ChatItem) {
	for i, x := range c.items {
		if x == it {
			c.InvalidateItem(i)
			return
		}
	}
}

func (c *Chat) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		c.scrollBy(-1)
		return true
	case tcell.KeyDown:
		c.scrollBy(1)
		return true
	case tcell.KeyPgUp:
		c.scrollBy(-c.lastVisible)
		return true
	case tcell.KeyPgDn:
		c.scrollBy(c.lastVisible)
		return true
	case tcell.KeyHome:
		c.follow = false
		c.scroll = 0
		return true
	case tcell.KeyEnd:
		c.follow = true
		return true
	case tcell.KeyLeft:
		if ev.Modifiers()&tcell.ModShift != 0 {
			return c.ScrollTableHorizontal(-1)
		}
		return false
	case tcell.KeyRight:
		if ev.Modifiers()&tcell.ModShift != 0 {
			return c.ScrollTableHorizontal(1)
		}
		return false
	case tcell.KeyEnter:
		c.activateCursor()
		return true
	case tcell.KeyEsc:
		if c.back && c.onBack != nil {
			c.onBack()
			return true
		}
		return false
	case tcell.KeyRune:
		switch ev.Rune() {
		case ' ':
			c.activateCursor()
			return true
		case ']':
			c.nextItem()
			return true
		case '[':
			c.prevItem()
			return true
		}
	}
	return false
}

func (c *Chat) scrollBy(delta int) {
	if c.flat == nil || len(c.items) == 0 {
		return
	}
	if c.follow {
		if delta >= 0 {
			return
		}
		off := c.maxScroll() + delta
		c.follow = false
		c.setScroll(off)
		return
	}
	off := c.scroll + delta
	if off >= c.maxScroll() {
		c.follow = true
		return
	}
	if off < 0 && c.hasMore && c.onLoadMore != nil {
		c.onLoadMore()
		return
	}
	c.setScroll(off)
}

func (c *Chat) ScrollBy(delta int) {
	c.scrollBy(delta)
}

func (c *Chat) ScrollUp(page int) {
	c.scrollBy(-page)
}

func (c *Chat) ScrollDown(page int) {
	c.scrollBy(page)
}

func (c *Chat) ScrollToBottom() {
	c.follow = true
	c.autoScroll = true
	c.wasAtBottom = true
}

func (c *Chat) itemAtFlat(idx int) *ChatItem {
	i := sort.Search(len(c.flatItems), func(i int) bool {
		return c.flatItems[i].End > idx
	})
	if i >= len(c.flatItems) {
		return nil
	}
	fi := c.flatItems[i]
	if idx < fi.Start || fi.ItemIdx < 0 || fi.ItemIdx >= len(c.items) {
		return nil
	}
	return c.items[fi.ItemIdx]
}

func (c *Chat) HandleWheel(x, y, top, left, delta int) bool {
	if len(c.flat) == 0 || delta == 0 {
		return false
	}
	idx := y - top + c.scroll
	if c.backVisible {
		idx--
	}
	if idx < 0 || idx >= len(c.flat) || !isTableLine(c.flat[idx]) {
		return false
	}
	return c.scrollTableForItem(c.itemAtFlat(idx), delta)
}

func (c *Chat) setScroll(off int) {
	c.scroll = ClampScroll(off, c.maxScroll())
}

func (c *Chat) maxScroll() int {
	total := len(c.flat)
	vis := c.lastVisible
	if vis <= 0 {
		return 0
	}
	m := total - vis
	if m < 0 {
		return 0
	}
	return m
}

func (c *Chat) HandleClick(x, y, top int) bool {
	if len(c.items) == 0 || y < top {
		return false
	}

	if c.tableLeftOn && y == c.tableLeftY && x >= c.tableLeftX0 && x <= c.tableLeftX1 {
		c.ScrollTableHorizontal(-1)
		return true
	}
	if c.tableRightOn && y == c.tableRightY && x >= c.tableRightX0 && x <= c.tableRightX1 {
		c.ScrollTableHorizontal(1)
		return true
	}
	if c.indicatorVisible && y == c.indicatorY && x >= c.indicatorX0 && x <= c.indicatorX1 {
		c.follow = true
		return true
	}
	if c.backVisible && y == c.backY && x >= c.backX0 && x <= c.backX1 {
		if c.onBack != nil {
			c.onBack()
		}
		return true
	}

	rel := y - top + c.scroll
	if rel < 0 || rel >= len(c.flat) {
		return false
	}
	if c.historyButtons != nil {
		if agentID, ok := c.historyButtons[rel]; ok {
			if c.onSubagentHistory != nil {
				c.onSubagentHistory(agentID)
			}
			return true
		}
	}
	if c.fileButtons != nil {
		if path, ok := c.fileButtons[rel]; ok && path != "" {
			if c.onOpenFile != nil {
				c.onOpenFile(path)
			}
			return true
		}
	}
	for _, fi := range c.flatItems {
		if rel >= fi.Start && rel < fi.End {
			c.itemCursor = fi.ItemIdx
			if c.onActivate != nil {
				c.onActivate(c.items[fi.ItemIdx])
			}
			return true
		}
	}
	return true
}

func (c *Chat) posAt(x, y, top, left int) (selPos, bool) {
	contentTop := top
	if c.backVisible {
		contentTop++
	}
	line := y - contentTop + c.scroll
	if line < 0 || line >= len(c.flat) {
		return selPos{}, false
	}
	cell := x - (left + 1)
	if cell < 0 {
		cell = 0
	}
	if off := c.hscrollAtFlat(line); off > 0 && c.lastWidth > 0 && c.flat[line].Width() > c.lastWidth {
		cell += off
	}
	return selPos{line: line, cell: cell}, true
}

func (c *Chat) selectionBounds() (selPos, selPos, bool) {
	if !c.selActive {
		return selPos{}, selPos{}, false
	}
	a, b := c.selStart, c.selEnd
	if a.line > b.line || (a.line == b.line && a.cell > b.cell) {
		a, b = b, a
	}
	return a, b, true
}

func (c *Chat) HasSelection() bool { return c.selActive }

func (c *Chat) ClearSelection() {
	if c.selActive {
		c.selActive = false
		c.dirty = true
	}
}

func (c *Chat) setSelection(a, b selPos) {
	c.selStart, c.selEnd = a, b
	c.selActive = true
	c.dirty = true
}

func (c *Chat) HandleMouse(x, y, top, left int, buttons tcell.ButtonMask) bool {
	if buttons&tcell.WheelLeft != 0 {
		return c.ScrollTableHorizontal(-1)
	}
	if buttons&tcell.WheelRight != 0 {
		return c.ScrollTableHorizontal(1)
	}
	pos, ok := c.posAt(x, y, top, left)
	pressed := buttons&tcell.ButtonPrimary != 0

	if pressed {
		if !c.mouseDown {
			c.mouseDown = true
			c.dragged = false
			if ok {
				c.mouseStart = pos
			}
		} else if ok && c.isDrag(pos) {
			c.dragged = true
			c.setSelection(c.mouseStart, pos)
		}
		return true
	}

	if c.mouseDown {
		c.mouseDown = false
		if c.dragged && c.selActive {
			c.dragged = false
			return true
		}
		c.dragged = false
		c.ClearSelection()
		return c.HandleClick(x, y, top)
	}
	return false
}

func (c *Chat) isDrag(pos selPos) bool {
	if pos.line != c.mouseStart.line {
		return true
	}
	d := pos.cell - c.mouseStart.cell
	if d < 0 {
		d = -d
	}
	return d > 1
}

var frameRunes = map[rune]bool{
	'┌': true, '┐': true, '└': true, '┘': true, '─': true, '│': true, '▍': true,
}

func isFrameOnly(cells []Cell) bool {
	for _, c := range cells {
		if !frameRunes[c.R] && c.R != ' ' {
			return false
		}
	}
	return true
}

func sanitizeSelectedCells(cells []Cell) string {
	var b strings.Builder
	for _, c := range cells {
		if frameRunes[c.R] {
			continue
		}
		b.WriteRune(c.R)
	}
	return strings.TrimRight(b.String(), " \t")
}

func (c *Chat) SelectedText() string {
	a, b, ok := c.selectionBounds()
	if !ok || len(c.flat) == 0 {
		return ""
	}
	if a.line >= len(c.flat) {
		return ""
	}
	if b.line >= len(c.flat) {
		b.line = len(c.flat) - 1
		b.cell = len(c.flat[b.line])
	}
	var out []string
	for line := a.line; line <= b.line; line++ {
		cells := c.flat[line]
		from, to := 0, len(cells)
		if line == a.line {
			from = a.cell
		}
		if line == b.line {
			to = b.cell
		}
		if from > to {
			from = to
		}
		if from > len(cells) {
			from = len(cells)
		}
		if to > len(cells) {
			to = len(cells)
		}
		seg := cells[from:to]
		if isFrameOnly(seg) {
			continue
		}
		out = append(out, sanitizeSelectedCells(seg))
	}
	return strings.Join(out, "\n")
}

func (c *Chat) applySelection(idx int, line Line, s, e selPos, th styles.Theme) Line {
	if idx < s.line || idx > e.line {
		return line
	}
	from, to := 0, len(line)
	if idx == s.line {
		from = s.cell
	}
	if idx == e.line {
		to = e.cell
	}
	if from < 0 {
		from = 0
	}
	if from > len(line) {
		from = len(line)
	}
	if to > len(line) {
		to = len(line)
	}
	if from >= to {
		return line
	}
	selStyle := th.Base().Background(th.Selection)
	out := make(Line, len(line))
	copy(out, line)
	for i := from; i < to; i++ {
		out[i].S = selStyle
	}
	return out
}

func (c *Chat) Draw(s tcell.Screen, bounds layout.Region, focused bool) {
	th := styles.Current()
	bg := th.Base().Background(th.Background)
	FillRegion(s, bounds, ' ', bg)

	innerW := bounds.Width - 2
	if innerW < 4 {
		return
	}
	c.lastWidth = innerW

	if c.dirty || c.flatRev != c.lastWidth || c.flat == nil {
		c.rebuildFlat(innerW)
	}

	total := len(c.flat)
	if total == 0 {
		return
	}

	if c.back && bounds.Height > 2 {
		headerStyle := th.Base().Foreground(th.Accent).Bold(true).Background(th.Background)
		backText := i18n.T("common.back")
		DrawText(s, bounds.Left+1, bounds.Top, backText, headerStyle)
		c.backVisible = true
		c.backX0 = bounds.Left + 1
		c.backX1 = bounds.Left + 1 + len([]rune(backText)) - 1
		c.backY = bounds.Top
		bounds.Top++
		bounds.Height--
	} else {
		c.backVisible = false
	}

	c.lastVisible = bounds.Height
	visible := bounds.Height
	if c.follow {
		c.scroll = ClampScroll(total-visible, total)
	}
	if c.scroll < 0 {
		c.scroll = 0
	}
	maxS := c.maxScroll()
	if c.scroll > maxS {
		c.scroll = maxS
	}
	c.wasAtBottom = c.scroll >= maxS-2

	offset := c.scroll

	markerLine := -1
	if focused && c.itemCursor >= 0 && c.itemCursor < len(c.flatItems) {
		markerLine = c.flatItems[c.itemCursor].Start
	}

	selS, selE, hasSel := c.selectionBounds()
	c.tableHintOn = false
	tableOff, tableMax := 0, 0
	for row := 0; row < visible; row++ {
		idx := offset + row
		if idx >= total {
			break
		}
		x := bounds.Left
		y := bounds.Top + row
		if idx == markerLine {
			s.SetContent(x, y, '▍', nil, th.Base().Foreground(th.Accent).Background(th.Background))
		} else {
			s.SetContent(x, y, ' ', nil, bg)
		}
		line := c.flat[idx]
		if hasSel {
			line = c.applySelection(idx, line, selS, selE, th)
		}
		if rawWidth := line.Width(); rawWidth > innerW {
			if it := c.itemAtFlat(idx); it != nil {
				if maxOff := c.maxHScrollForItem(it); maxOff > 0 {
					if maxOff > tableMax {
						tableOff = c.hscrollFor(it)
						tableMax = maxOff
					}
					if off := c.hscrollFor(it); off > 0 {
						line = WindowLine(line, off, innerW)
					}
				}
			}
		}
		DrawLineBounded(s, x+1, y, bounds.Left+bounds.Width, line)
	}

	c.indicatorVisible = false
	below := total - (offset + visible)
	if below > 0 {
		label := i18n.Tf("chat.moreDown", below)
		labelCells := LineFromSegments([]Segment{
			{Text: label, Style: th.Base().Foreground(th.Accent).Background(th.Surface)},
		}, len(label))
		lx := bounds.Right() - len(label) - 1
		ly := bounds.Bottom() - 1
		DrawLine(s, lx, ly, labelCells)
		c.indicatorX0, c.indicatorX1, c.indicatorY = lx, lx+len(label)-1, ly
		c.indicatorVisible = true
	}

	c.tableLeftOn, c.tableRightOn = false, false
	if tableMax > 0 {
		style := th.Base().Foreground(th.Accent).Background(th.Surface)
		x := bounds.Left + 1
		y := bounds.Bottom() - 1
		drawSpan := func(text string) {
			w := DisplayWidth(text)
			if w <= 0 {
				return
			}
			cells := LineFromSegments([]Segment{{Text: text, Style: style}}, w)
			DrawLineBounded(s, x, y, bounds.Right(), cells)
			x += w
		}

		drawSpan(" ")
		if tableOff > 0 {
			c.tableLeftX0, c.tableLeftX1, c.tableLeftY = x, x, y
			c.tableLeftOn = true
			drawSpan("←")
			drawSpan(" ")
		}
		drawSpan(i18n.T("chat.table.scroll"))
		if tableOff < tableMax {
			drawSpan(" ")
			c.tableRightX0, c.tableRightX1, c.tableRightY = x, x, y
			c.tableRightOn = true
			drawSpan("→")
		}
		drawSpan(" ")
		c.tableHintOn = true
	}
}

func (c *Chat) setFollow(f bool) {
	if c.follow != f {
		c.follow = f
		if !f {
			c.scroll = c.maxScroll()
		}
	}
}

func (c *Chat) nextItem() {
	if len(c.flatItems) == 0 {
		return
	}
	c.itemCursor++
	if c.itemCursor >= len(c.flatItems) {
		c.itemCursor = 0
	}
	c.revealCursor()
}

func (c *Chat) prevItem() {
	if len(c.flatItems) == 0 {
		return
	}
	c.itemCursor--
	if c.itemCursor < 0 {
		c.itemCursor = len(c.flatItems) - 1
	}
	c.revealCursor()
}

func (c *Chat) revealCursor() {
	if c.flat == nil || c.itemCursor < 0 || c.itemCursor >= len(c.flatItems) {
		return
	}
	item := c.flatItems[c.itemCursor]
	if c.follow {
		c.follow = false
	}
	off := c.scroll
	if item.Start < off {
		c.setScroll(item.Start)
	} else if item.Start >= off+c.lastVisible {
		c.setScroll(item.Start - c.lastVisible + 1)
	}
}

func (c *Chat) activateCursor() {
	if c.itemCursor < 0 || c.itemCursor >= len(c.flatItems) {
		return
	}
	fi := c.flatItems[c.itemCursor]
	if fi.ItemIdx >= 0 && fi.ItemIdx < len(c.items) {
		if c.onActivate != nil {
			c.onActivate(c.items[fi.ItemIdx])
		}
	}
}

func (c *Chat) rebuildFlat(width int) {
	if c.cache == nil || len(c.cache) != len(c.items) || c.cacheWidth != width {
		c.cache = make([][]Line, len(c.items))
		c.cacheWidth = width
	}
	c.flat = nil
	c.flatItems = nil
	c.historyButtons = nil
	c.fileButtons = nil
	total := 0
	for i, it := range c.items {
		live := c.isLiveItem(it)
		var lines []Line
		if cached := c.cache[i]; cached != nil && !live {
			lines = cached
		} else {
			lines = c.itemLines(it, width)
			if !live {
				c.cache[i] = lines
			}
		}
		if it.Kind == ItemAssistant {
			maxW := 0
			for _, ln := range lines {
				if w := ln.Width(); w > maxW {
					maxW = w
				}
			}
			if c.itemMaxWidth == nil {
				c.itemMaxWidth = map[*ChatItem]int{}
			}
			if c.itemTableBounds == nil {
				c.itemTableBounds = map[*ChatItem][]int{}
			}
			c.itemMaxWidth[it] = maxW
			c.itemTableBounds[it] = tableColumnBounds(lines)
		}
		if len(lines) > 0 {
			c.flatItems = append(c.flatItems, flatItem{
				ItemIdx: i,
				Start:   total,
				End:     total + len(lines),
			})
			if id := c.historyAgentID(it); id != "" {
				if c.historyButtons == nil {
					c.historyButtons = make(map[int]string)
				}
				c.historyButtons[total] = id
			}
			if boxes := c.fileBoxesFor(it, width); len(boxes) > 0 {
				if c.fileButtons == nil {
					c.fileButtons = make(map[int]string)
				}
				for _, box := range boxes {
					for line := total + box.start; line < total+box.end; line++ {
						c.fileButtons[line] = box.path
					}
				}
			}
		}
		c.flat = append(c.flat, lines...)
		c.flat = append(c.flat, nil)
		total += len(lines) + 1
	}
	c.flatRev = width
	c.dirty = false
	if c.itemCursor >= len(c.flatItems) {
		c.itemCursor = -1
	}
}

func (c *Chat) isLiveItem(it *ChatItem) bool {
	switch it.Kind {
	case ItemTool:
		return !it.ToolDone && it.ToolErr == "" && it.ToolOutput == ""
	case ItemSubagent:
		return it.SubagentStatus == "" || it.SubagentStatus == "running"
	case ItemThinking:
		return c.isActiveThinking(it)
	}
	return false
}

func (c *Chat) historyAgentID(it *ChatItem) string {
	if it.Kind == ItemSubagent && it.AgentID != "" {
		return it.AgentID
	}
	if it.Kind == ItemTool && it.ToolTaskAgentID != "" && it.ToolTaskStatus == "running" {
		return it.ToolTaskAgentID
	}
	return ""
}

func (c *Chat) itemLines(it *ChatItem, width int) []Line {
	if width < 1 {
		width = 80
	}
	var lines []Line
	switch it.Kind {
	case ItemUser:
		lines = c.userCardLines(it, width)
	case ItemAssistant:
		lines = MdToLines(it.Text, width, styles.Current())
	case ItemThinking:
		lines = c.thinkingLines(it, width)
	case ItemTool:
		lines = c.toolLines(it, width)
	case ItemSubagent:
		lines = c.subagentLines(it, width)
	case ItemFinished:
		return nil
	case ItemError:
		th := styles.Current()
		lines = WrapSegments([]Segment{
			{Text: i18n.Tf("chat.errorPrefix", it.Text), Style: th.Base().Foreground(th.Error).Bold(true).Background(th.Background)},
		}, width)
	case ItemCompaction:
		th := styles.Current()
		lines = []Line{LineFromSegments([]Segment{
			{Text: "  " + it.Text, Style: th.Base().Foreground(th.Muted).Background(th.Background)},
		}, width)}
	case ItemCompactionDivider:
		th := styles.Current()
		lines = []Line{LineFromSegments([]Segment{
			{Text: "─ " + it.Text + " ─", Style: th.Base().Foreground(th.Muted).Background(th.Background)},
		}, width)}
	case ItemNotice:
		th := styles.Current()
		lines = WrapSegments([]Segment{
			{Text: "  " + it.Text, Style: th.Base().Foreground(th.Warning).Background(th.Background)},
		}, width)
	}
	if it.Kind == ItemAssistant {
		return lines
	}
	return clipLinesToWidth(lines, width)
}

func clipLinesToWidth(lines []Line, width int) []Line {
	for i, l := range lines {
		if l.Width() > width {
			lines[i] = clipLineToWidth(l, width)
		}
	}
	return lines
}

func clipLineToWidth(l Line, width int) Line {
	w := 0
	for i, c := range l {
		cw := cellWidth(c.R)
		if w+cw > width {
			return l[:i]
		}
		w += cw
	}
	return l
}

func (c *Chat) OnTick(blinkOn bool) {
	c.now += 450 * time.Millisecond
	if c.hasLiveItems() {
		c.dirty = true
	}
}

func (c *Chat) hasLiveItems() bool {
	for _, it := range c.items {
		if c.isLiveItem(it) {
			return true
		}
	}
	return false
}
