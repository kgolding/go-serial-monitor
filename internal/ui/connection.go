package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/serialio"
)

// Preferences keys used to remember the serial settings between runs.
const (
	prefPort     = "serial.port"
	prefBaud     = "serial.baud"
	prefDataBits = "serial.dataBits"
	prefParity   = "serial.parity"
	prefStopBits = "serial.stopBits"
)

var (
	dataBitsOptions = []string{"5", "6", "7", "8"}
	parityOptions   = []struct {
		Value serialio.Parity
		Label string
	}{
		{serialio.ParityNone, "None"},
		{serialio.ParityOdd, "Odd"},
		{serialio.ParityEven, "Even"},
		{serialio.ParityMark, "Mark"},
		{serialio.ParitySpace, "Space"},
	}
	stopBitsOptions = []struct {
		Value serialio.StopBits
		Label string
	}{
		{serialio.StopBits1, "1"},
		{serialio.StopBits1Half, "1.5"},
		{serialio.StopBits2, "2"},
	}
)

// connectionBar lets the user pick a port and its settings and connect.
type connectionBar struct {
	app *App

	portSelect     *widget.SelectEntry
	baudSelect     *widget.SelectEntry
	dataBitsSelect *widget.Select
	paritySelect   *widget.Select
	stopBitsSelect *widget.Select
	connectBtn     *widget.Button
	statusLabel    *widget.Label
}

func newConnectionBar(app *App) *connectionBar {
	c := &connectionBar{app: app}

	c.portSelect = widget.NewSelectEntry(nil)
	c.portSelect.PlaceHolder = "(select or type a port)"

	baudLabels := make([]string, len(serialio.CommonBaudRates))
	for i, b := range serialio.CommonBaudRates {
		baudLabels[i] = strconv.Itoa(b)
	}
	c.baudSelect = widget.NewSelectEntry(baudLabels)
	c.baudSelect.SetText("115200")

	c.dataBitsSelect = widget.NewSelect(dataBitsOptions, nil)
	c.dataBitsSelect.SetSelected("8")

	parityLabels := make([]string, len(parityOptions))
	for i, p := range parityOptions {
		parityLabels[i] = p.Label
	}
	c.paritySelect = widget.NewSelect(parityLabels, nil)
	c.paritySelect.SetSelected("None")

	stopLabels := make([]string, len(stopBitsOptions))
	for i, s := range stopBitsOptions {
		stopLabels[i] = s.Label
	}
	c.stopBitsSelect = widget.NewSelect(stopLabels, nil)
	c.stopBitsSelect.SetSelected("1")

	c.connectBtn = widget.NewButtonWithIcon("Connect", theme.MediaPlayIcon(), c.onConnectClicked)
	c.statusLabel = widget.NewLabel("Disconnected")

	c.loadSettings()
	c.refreshPorts()

	// Wired up only after loading, so restoring saved values above doesn't
	// immediately re-save them.
	c.portSelect.OnChanged = func(string) { c.saveSettings() }
	c.baudSelect.OnChanged = func(string) { c.saveSettings() }
	c.dataBitsSelect.OnChanged = func(string) { c.saveSettings() }
	c.paritySelect.OnChanged = func(string) { c.saveSettings() }
	c.stopBitsSelect.OnChanged = func(string) { c.saveSettings() }

	return c
}

// loadSettings restores the serial settings saved by a previous run, if any.
func (c *connectionBar) loadSettings() {
	prefs := c.app.fyneApp.Preferences()
	c.portSelect.SetText(prefs.StringWithFallback(prefPort, c.portSelect.Text))
	c.baudSelect.SetText(prefs.StringWithFallback(prefBaud, c.baudSelect.Text))
	c.dataBitsSelect.SetSelected(prefs.StringWithFallback(prefDataBits, c.dataBitsSelect.Selected))
	c.paritySelect.SetSelected(prefs.StringWithFallback(prefParity, c.paritySelect.Selected))
	c.stopBitsSelect.SetSelected(prefs.StringWithFallback(prefStopBits, c.stopBitsSelect.Selected))
}

// saveSettings persists the current serial settings so they can be restored
// the next time the app starts.
func (c *connectionBar) saveSettings() {
	prefs := c.app.fyneApp.Preferences()
	prefs.SetString(prefPort, c.portSelect.Text)
	prefs.SetString(prefBaud, c.baudSelect.Text)
	prefs.SetString(prefDataBits, c.dataBitsSelect.Selected)
	prefs.SetString(prefParity, c.paritySelect.Selected)
	prefs.SetString(prefStopBits, c.stopBitsSelect.Selected)
}

// SettingsCanvasObject builds the port/baud/data bits/parity/stop bits form,
// intended for the sidebar's "Serial Settings" accordion.
func (c *connectionBar) SettingsCanvasObject() fyne.CanvasObject {
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), c.refreshPorts)
	portRow := container.NewBorder(nil, nil, nil, refreshBtn, c.portSelect)

	form := widget.NewForm(
		widget.NewFormItem("Baud", c.baudSelect),
		widget.NewFormItem("Data bits", c.dataBitsSelect),
		widget.NewFormItem("Parity", c.paritySelect),
		widget.NewFormItem("Stop bits", c.stopBitsSelect),
	)

	return container.NewVBox(portRow, form)
}

// ControlsCanvasObject builds the Connect/Disconnect button and status
// label, intended for the top of the main content area.
func (c *connectionBar) ControlsCanvasObject() fyne.CanvasObject {
	return container.NewHBox(c.connectBtn, c.statusLabel)
}

func (c *connectionBar) refreshPorts() {
	ports, err := serialio.ListPorts()
	c.portSelect.SetOptions(ports)
	if err != nil {
		c.statusLabel.SetText(fmt.Sprintf("Error listing ports: %v", err))
	}
	if c.portSelect.Text == "" && len(ports) > 0 {
		c.portSelect.SetText(ports[0])
	}
}

func (c *connectionBar) onConnectClicked() {
	if c.app.Connected() {
		c.app.Disconnect()
		return
	}

	if strings.TrimSpace(c.portSelect.Text) == "" {
		dialog.ShowError(fmt.Errorf("select or type a serial port first"), c.app.win)
		return
	}
	baud, err := strconv.Atoi(c.baudSelect.Text)
	if err != nil || baud <= 0 {
		dialog.ShowError(fmt.Errorf("invalid baud rate %q", c.baudSelect.Text), c.app.win)
		return
	}
	dataBits, _ := strconv.Atoi(c.dataBitsSelect.Selected)

	cfg := serialio.Config{
		Port:     c.portSelect.Text,
		BaudRate: baud,
		DataBits: dataBits,
		Parity:   parityFromLabel(c.paritySelect.Selected),
		StopBits: stopBitsFromLabel(c.stopBitsSelect.Selected),
	}

	if err := c.app.Connect(cfg); err != nil {
		dialog.ShowError(fmt.Errorf("could not open %s: %w", cfg.Port, err), c.app.win)
		return
	}
}

// setConnected updates the toolbar to reflect the current connection state.
func (c *connectionBar) setConnected(connected bool, cfg serialio.Config) {
	if connected {
		c.connectBtn.SetText("Disconnect")
		c.connectBtn.SetIcon(theme.MediaStopIcon())
		c.statusLabel.SetText(fmt.Sprintf("Connected to %s at %d,%d%s%s",
			cfg.Port, cfg.BaudRate, cfg.DataBits, parityCode(cfg.Parity), stopBitsCode(cfg.StopBits)))
		c.portSelect.Disable()
		c.baudSelect.Disable()
		c.dataBitsSelect.Disable()
		c.paritySelect.Disable()
		c.stopBitsSelect.Disable()
	} else {
		c.connectBtn.SetText("Connect")
		c.connectBtn.SetIcon(theme.MediaPlayIcon())
		c.statusLabel.SetText("Disconnected")
		c.portSelect.Enable()
		c.baudSelect.Enable()
		c.dataBitsSelect.Enable()
		c.paritySelect.Enable()
		c.stopBitsSelect.Enable()
	}
}

func parityFromLabel(label string) serialio.Parity {
	for _, p := range parityOptions {
		if p.Label == label {
			return p.Value
		}
	}
	return serialio.ParityNone
}

func stopBitsFromLabel(label string) serialio.StopBits {
	for _, s := range stopBitsOptions {
		if s.Label == label {
			return s.Value
		}
	}
	return serialio.StopBits1
}

// parityCode returns the single-letter code conventionally used in compact
// serial notation (e.g. the "N" in "9600,8N1").
func parityCode(p serialio.Parity) string {
	switch p {
	case serialio.ParityOdd:
		return "O"
	case serialio.ParityEven:
		return "E"
	case serialio.ParityMark:
		return "M"
	case serialio.ParitySpace:
		return "S"
	default:
		return "N"
	}
}

// stopBitsCode returns the stop bits count as used in compact serial
// notation (e.g. the "1" in "9600,8N1").
func stopBitsCode(s serialio.StopBits) string {
	for _, o := range stopBitsOptions {
		if o.Value == s {
			return o.Label
		}
	}
	return "1"
}
