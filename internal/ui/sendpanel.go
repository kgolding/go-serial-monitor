package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/store"
)

// sendPanel is the bottom bar used to compose and send one-off ASCII/hex data.
type sendPanel struct {
	app *App

	entry            *widget.Entry
	formatRadio      *widget.RadioGroup
	lineEndingSelect *widget.Select
	sendBtn          *widget.Button
}

func newSendPanel(app *App) *sendPanel {
	s := &sendPanel{app: app}

	s.entry = widget.NewEntry()
	s.entry.OnSubmitted = func(string) { s.doSend() }

	s.formatRadio = widget.NewRadioGroup([]string{"ASCII", "Hex"}, func(string) {
		setDataPlaceholder(s.entry, s.formatRadio)
	})
	s.formatRadio.Horizontal = true
	s.formatRadio.SetSelected("ASCII")
	setDataPlaceholder(s.entry, s.formatRadio)

	s.lineEndingSelect = widget.NewSelect(lineEndingLabels(), nil)
	s.lineEndingSelect.SetSelected(lineEndingLabel(store.LineEndingCRLF))

	s.sendBtn = widget.NewButtonWithIcon("Send", theme.MailSendIcon(), s.doSend)
	s.sendBtn.Disable()

	return s
}

func (s *sendPanel) CanvasObject() fyne.CanvasObject {
	options := container.NewHBox(
		s.formatRadio,
		widget.NewLabel("Line ending:"),
		s.lineEndingSelect,
	)
	return container.NewBorder(options, nil, nil, s.sendBtn, s.entry)
}

func (s *sendPanel) setEnabled(enabled bool) {
	if enabled {
		s.sendBtn.Enable()
	} else {
		s.sendBtn.Disable()
	}
}

func (s *sendPanel) doSend() {
	format := formatFromLabel(s.formatRadio.Selected)
	le := lineEndingFromLabel(s.lineEndingSelect.Selected)

	data, err := buildSendBytes(format, s.entry.Text, le)
	if err != nil {
		dialog.ShowError(err, s.app.win)
		return
	}
	if err := s.app.SendBytes(data); err != nil {
		dialog.ShowError(err, s.app.win)
	}
}
