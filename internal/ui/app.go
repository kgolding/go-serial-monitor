// Package ui assembles the Fyne widgets for the serial monitor application.
package ui

import (
	"errors"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/serialio"
	"github.com/kgolding/go-serial-monitor/internal/store"
)

// App holds the shared state for the whole window: the open serial
// connection (if any), the preset store, and the widgets that need to react
// to connect/disconnect and incoming data.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	store   *store.Store
	colors  *colorSettings

	conn    *connectionBar
	monitor *monitorView
	send    *sendPanel
	presets *presetsView
	display *displaySettingsView

	port        serialio.Port
	connected   bool
	mergeWindow time.Duration // RX chunks arriving within this of each other share a row
}

// NewApp builds the application state and its window content.
func NewApp(fyneApp fyne.App, win fyne.Window) (*App, error) {
	st, err := store.NewStore(fyneApp)
	if err != nil {
		return nil, fmt.Errorf("loading presets: %w", err)
	}

	a := &App{fyneApp: fyneApp, win: win, store: st}
	a.colors = newColorSettings(fyneApp)
	a.conn = newConnectionBar(a)
	a.monitor = newMonitorView(win, a.colors)
	a.send = newSendPanel(a)
	a.presets = newPresetsView(a)
	a.display = newDisplaySettingsView(a)
	return a, nil
}

// Content builds the root canvas object for the main window.
func (a *App) Content() fyne.CanvasObject {
	// Each section is its own single-item accordion so Fyne's Accordion
	// layout (which splits any extra height evenly across every open item)
	// doesn't stretch the fixed-height settings sections - they sit at their
	// natural height and the presets accordion starts right below them,
	// taking all remaining sidebar space.
	settingsAccordion := widget.NewAccordion(widget.NewAccordionItem("Serial Settings", a.conn.SettingsCanvasObject()))
	settingsAccordion.Open(0)

	// Closed by default: it's used far less often than the other two.
	displayAccordion := widget.NewAccordion(widget.NewAccordionItem("Display Settings", a.display.CanvasObject()))

	presetsAccordion := widget.NewAccordion(widget.NewAccordionItem("Presets", a.presets.CanvasObject()))
	presetsAccordion.Open(0)

	topStack := container.NewVBox(settingsAccordion, displayAccordion)
	sidebar := container.NewBorder(topStack, nil, nil, nil, presetsAccordion)

	main := container.NewBorder(
		a.conn.ControlsCanvasObject(),
		a.send.CanvasObject(),
		nil, nil,
		a.monitor.CanvasObject(),
	)

	split := container.NewHSplit(sidebar, main)
	split.Offset = 0.22
	return split
}

// Connected reports whether a serial port is currently open.
func (a *App) Connected() bool {
	return a.connected
}

// Connect opens the serial port described by cfg and wires up event
// handling. It returns an error if the port could not be opened.
func (a *App) Connect(cfg serialio.Config) error {
	if a.connected {
		return nil
	}
	p, err := serialio.Open(cfg, a.handleSerialEvent)
	if err != nil {
		return err
	}
	a.port = p
	a.connected = true
	a.mergeWindow = 2 * serialio.FrameDuration(cfg)
	a.conn.setConnected(true, cfg)
	a.monitor.SetPortName(cfg.Port)
	a.send.setEnabled(true)
	a.presets.setEnabled(true)
	return nil
}

// Disconnect closes the serial port, if one is open.
func (a *App) Disconnect() {
	if !a.connected {
		return
	}
	a.connected = false
	a.mergeWindow = 0
	if a.port != nil {
		a.port.Close()
		a.port = nil
	}
	a.conn.setConnected(false, serialio.Config{})
	a.send.setEnabled(false)
	a.presets.setEnabled(false)
}

// SendBytes writes data to the open port and logs it to the monitor as a TX
// entry. It fails if no port is currently open.
func (a *App) SendBytes(data []byte) error {
	if !a.connected || a.port == nil {
		return errors.New("not connected to a serial port")
	}
	if len(data) == 0 {
		return errors.New("nothing to send")
	}
	if _, err := a.port.Write(data); err != nil {
		return err
	}
	a.monitor.Append(true, data, time.Now(), 0)
	return nil
}

// handleSerialEvent is invoked by serialio from a background goroutine; it
// hops back onto the UI goroutine before touching any widgets.
func (a *App) handleSerialEvent(ev serialio.Event) {
	switch ev.Kind {
	case serialio.EventRX:
		fyne.Do(func() {
			a.monitor.Append(false, ev.Data, ev.Time, a.mergeWindow)
		})
	case serialio.EventError:
		fyne.Do(func() {
			wasConnected := a.connected
			a.connected = false
			a.port = nil
			a.conn.setConnected(false, serialio.Config{})
			a.send.setEnabled(false)
			a.presets.setEnabled(false)
			if wasConnected {
				dialog.ShowError(fmt.Errorf("serial port error: %w", ev.Err), a.win)
			}
		})
	case serialio.EventClosed:
		// Expected after an explicit Disconnect(); state is already up to date.
	}
}
