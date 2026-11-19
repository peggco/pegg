package components

import (
	"sort"
)

func (c *Chat) resetTableScroll() {
	c.hscroll = map[*ChatItem]int{}
	c.itemMaxWidth = map[*ChatItem]int{}
	c.itemTableBounds = map[*ChatItem][]int{}
	c.tableHintOn = false
	c.tableLeftOn = false
	c.tableRightOn = false
}

func isTableLine(ln Line) bool {
	return len(ln) >= 2 && ln[0].R == '│' && ln[1].R == ' ' && ln[len(ln)-1].R == '│'
}

func tableColumnBounds(lines []Line) []int {
	seen := map[int]bool{}
	var bounds []int
	for _, ln := range lines {
		if !isTableLine(ln) {
			continue
		}
		col := 0
		for _, cell := range ln {
			if cell.R == '│' && !seen[col] {
				seen[col] = true
				bounds = append(bounds, col)
			}
			col += cellWidth(cell.R)
		}
	}
	sort.Ints(bounds)
	return bounds
}

func (c *Chat) maxHScrollForItem(it *ChatItem) int {
	if it == nil || c.lastWidth <= 0 {
		return 0
	}
	maxW := c.itemMaxWidth[it]
	if maxW <= c.lastWidth {
		return 0
	}
	return maxW - c.lastWidth
}

func (c *Chat) hscrollFor(it *ChatItem) int {
	if it == nil || c.hscroll == nil {
		return 0
	}
	return ClampScroll(c.hscroll[it], c.maxHScrollForItem(it))
}

func (c *Chat) hscrollAtFlat(idx int) int {
	return c.hscrollFor(c.itemAtFlat(idx))
}

func (c *Chat) activeWideItem() *ChatItem {
	if c.itemCursor >= 0 && c.itemCursor < len(c.flatItems) {
		it := c.items[c.flatItems[c.itemCursor].ItemIdx]
		if c.maxHScrollForItem(it) > 0 {
			return it
		}
	}
	start := c.scroll
	end := start + c.lastVisible
	if c.lastVisible <= 0 {
		end = len(c.flat)
	}
	for _, fi := range c.flatItems {
		if fi.End <= start || fi.Start >= end {
			continue
		}
		it := c.items[fi.ItemIdx]
		if c.maxHScrollForItem(it) > 0 {
			return it
		}
	}
	return nil
}

func (c *Chat) ScrollTableHorizontal(delta int) bool {
	return c.scrollTableForItem(c.activeWideItem(), delta)
}

func (c *Chat) scrollTableForItem(it *ChatItem, delta int) bool {
	if it == nil || delta == 0 {
		return false
	}
	maxOff := c.maxHScrollForItem(it)
	if maxOff <= 0 {
		return false
	}
	cur := c.hscrollFor(it)
	next := c.snapTableOffset(it, cur, delta, maxOff)
	if next == cur {
		return false
	}
	if c.hscroll == nil {
		c.hscroll = map[*ChatItem]int{}
	}
	c.hscroll[it] = next
	c.dirty = true
	return true
}

func (c *Chat) snapTableOffset(it *ChatItem, cur, delta, maxOff int) int {
	bounds := c.itemTableBounds[it]
	if len(bounds) == 0 {
		step := c.lastWidth / 4
		if step < 4 {
			step = 4
		}
		return ClampScroll(cur+delta*step, maxOff)
	}
	if delta > 0 {
		for _, b := range bounds {
			if b > cur {
				return ClampScroll(b, maxOff)
			}
		}
		return maxOff
	}
	for i := len(bounds) - 1; i >= 0; i-- {
		if bounds[i] < cur {
			return ClampScroll(bounds[i], maxOff)
		}
	}
	return 0
}

func (c *Chat) TableScrollState() (offset, max int, ok bool) {
	it := c.activeWideItem()
	if it == nil {
		return 0, 0, false
	}
	return c.hscrollFor(it), c.maxHScrollForItem(it), true
}

func (c *Chat) TableHintVisible() bool { return c.tableHintOn }
