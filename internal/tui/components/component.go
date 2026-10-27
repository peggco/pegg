package components

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/layout"
)

type Component interface {
	HandleKey(ev *tcell.EventKey) bool
	Draw(s tcell.Screen, bounds layout.Region, focused bool)
}

type Ticker interface {
	OnTick(blinkOn bool)
}

type Animator interface {
	Animate(now time.Time) bool
}

type Focusable interface {
	Component
	Focus()
	Blur()
	Focused() bool
}

type MouseComponent interface {
	Component
	HandleMouse(x, y int, buttons tcell.ButtonMask) bool
}
