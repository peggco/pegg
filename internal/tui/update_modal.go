package tui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/components"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

type updateState int

const (
	updateStateConfirm updateState = iota
	updateStateUpdating
	updateStateDone
	updateStateError
)

type UpdateModal struct {
	version       string
	state         updateState
	errMsg        string
	onUpdate      func() error
	onSkip        func()
	onQuit        func()
	requestRedraw func()
	selected      int
}

func NewUpdateModal(version string, onUpdate func() error, onSkip func()) *UpdateModal {
	return &UpdateModal{
		version:  version,
		state:    updateStateConfirm,
		onUpdate: onUpdate,
		onSkip:   onSkip,
		selected: 0,
	}
}

func (m *UpdateModal) SetRedraw(fn func()) {
	m.requestRedraw = fn
}

func (m *UpdateModal) HandleKey(ev *tcell.EventKey) bool {
	switch m.state {
	case updateStateConfirm:
		switch ev.Key() {
		case tcell.KeyLeft, tcell.KeyRight:
			m.selected = (m.selected + 1) % 2
			return true
		case tcell.KeyEnter:
			if m.selected == 0 && m.onUpdate != nil {
				m.state = updateStateUpdating
				go m.runUpdate()
			} else if m.selected == 1 && m.onSkip != nil {
				m.onSkip()
			}
			return true
		case tcell.KeyEsc:
			if m.onSkip != nil {
				m.onSkip()
			}
			return true
		}
	case updateStateDone, updateStateError:
		switch ev.Key() {
		case tcell.KeyEnter, tcell.KeyEsc:
			if m.onQuit != nil {
				m.onQuit()
			}
			return true
		}
	}
	return false
}

func (m *UpdateModal) runUpdate() {
	err := m.onUpdate()
	if err != nil {
		m.errMsg = err.Error()
		m.state = updateStateError
	} else {
		m.state = updateStateDone
	}
	if m.requestRedraw != nil {
		m.requestRedraw()
	}
}

func (m *UpdateModal) SetQuit(fn func()) {
	m.onQuit = fn
}

func (m *UpdateModal) Draw(s tcell.Screen, bounds layout.Region, focused bool) {
	w, h := 50, 10
	if bounds.Width-8 < w {
		w = bounds.Width - 8
	}

	th := styles.Current()
	modalBg := th.Base().Background(th.InputBg)

	switch m.state {
	case updateStateConfirm:
		m.drawConfirm(s, bounds, w, h, th, modalBg)
	case updateStateUpdating:
		m.drawUpdating(s, bounds, w, h, th, modalBg)
	case updateStateDone:
		m.drawDone(s, bounds, w, h, th, modalBg)
	case updateStateError:
		m.drawError(s, bounds, w, h, th, modalBg)
	}
}

func (m *UpdateModal) drawConfirm(s tcell.Screen, bounds layout.Region, w, h int, th styles.Theme, modalBg tcell.Style) {
	inner := components.DrawCenteredBox(s, bounds, w, h, i18n.T("update.titleConfirm"))

	y := inner.Top + 1
	msg := i18n.Tf("update.available", m.version)
	components.DrawText(s, inner.Left, y, msg, modalBg.Foreground(th.Foreground))

	y += 2
	components.DrawText(s, inner.Left, y, i18n.T("update.prompt"), modalBg.Foreground(th.Foreground))

	y += 2
	updateLabel := i18n.T("update.updateLabel")
	skipLabel := i18n.T("update.laterLabel")

	if m.selected == 0 {
		components.DrawText(s, inner.Left+5, y, updateLabel, modalBg.Foreground(th.Accent))
		components.DrawText(s, inner.Left+25, y, skipLabel, modalBg.Foreground(th.Foreground))
	} else {
		components.DrawText(s, inner.Left+5, y, updateLabel, modalBg.Foreground(th.Foreground))
		components.DrawText(s, inner.Left+25, y, skipLabel, modalBg.Foreground(th.Accent))
	}

	components.DrawFooter(s, inner, i18n.T("update.footerConfirm"))
}

func (m *UpdateModal) drawUpdating(s tcell.Screen, bounds layout.Region, w, h int, th styles.Theme, modalBg tcell.Style) {
	inner := components.DrawCenteredBox(s, bounds, w, h, i18n.T("update.titleUpdating"))

	y := inner.Top + 2
	components.DrawText(s, inner.Left, y, i18n.T("update.downloading"), modalBg.Foreground(th.Foreground))

	components.DrawFooter(s, inner, i18n.T("update.pleaseWait"))
}

func (m *UpdateModal) drawDone(s tcell.Screen, bounds layout.Region, w, h int, th styles.Theme, modalBg tcell.Style) {
	inner := components.DrawCenteredBox(s, bounds, w, h, i18n.T("update.titleDone"))

	y := inner.Top + 1
	components.DrawText(s, inner.Left, y, i18n.Tf("update.updated", m.version), modalBg.Foreground(th.Success))

	y += 2
	components.DrawText(s, inner.Left, y, i18n.T("update.restart"), modalBg.Foreground(th.Foreground))

	components.DrawFooter(s, inner, i18n.T("update.enterClose"))
}

func (m *UpdateModal) drawError(s tcell.Screen, bounds layout.Region, w, h int, th styles.Theme, modalBg tcell.Style) {
	inner := components.DrawCenteredBox(s, bounds, w, h, i18n.T("update.titleError"))

	y := inner.Top + 1
	components.DrawText(s, inner.Left, y, m.errMsg, modalBg.Foreground(th.Error))

	y += 2
	components.DrawText(s, inner.Left, y, i18n.T("update.errorRetry"), modalBg.Foreground(th.Foreground))

	components.DrawFooter(s, inner, i18n.T("update.enterClose"))
}
