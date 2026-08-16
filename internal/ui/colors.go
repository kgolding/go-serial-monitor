package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Preferences keys used to remember the monitor's RX/TX colors and row font
// size.
const (
	prefRXColor     = "display.rxColor"
	prefTXColor     = "display.txColor"
	prefRowFontSize = "display.rowFontSize"
)

// MinRowFontSize and MaxRowFontSize bound the monitor row font size setting.
const (
	MinRowFontSize = 8
	MaxRowFontSize = 32
)

// colorSettings holds the user-configurable monitor display settings (RX/TX
// colors and row font size), persisted via Fyne preferences.
type colorSettings struct {
	app         fyne.App
	RX          color.Color
	TX          color.Color
	RowFontSize float32
}

func newColorSettings(app fyne.App) *colorSettings {
	c := &colorSettings{app: app}
	c.Load()
	return c
}

// Load restores the saved display settings, falling back to the app's
// original look for anything unsaved.
func (c *colorSettings) Load() {
	prefs := c.app.Preferences()
	c.RX = parseHexColor(prefs.StringWithFallback(prefRXColor, ""), defaultRXColor())
	c.TX = parseHexColor(prefs.StringWithFallback(prefTXColor, ""), defaultTXColor())
	c.RowFontSize = float32(prefs.FloatWithFallback(prefRowFontSize, float64(defaultRowFontSize())))
}

// Save persists the current display settings.
func (c *colorSettings) Save() {
	prefs := c.app.Preferences()
	prefs.SetString(prefRXColor, colorToHex(c.RX))
	prefs.SetString(prefTXColor, colorToHex(c.TX))
	prefs.SetFloat(prefRowFontSize, float64(c.RowFontSize))
}

// ResetToDefault restores the display settings to the app's built-in
// defaults and persists the change.
func (c *colorSettings) ResetToDefault() {
	c.RX = defaultRXColor()
	c.TX = defaultTXColor()
	c.RowFontSize = defaultRowFontSize()
	c.Save()
}

func defaultRXColor() color.Color {
	return theme.Color(theme.ColorNameSuccess)
}

func defaultTXColor() color.Color {
	return theme.Color(theme.ColorNamePrimary)
}

func defaultRowFontSize() float32 {
	return theme.TextSize()
}

// colorToHex renders c as "#RRGGBB"; the monitor text is always fully
// opaque so alpha isn't preserved.
func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

// parseHexColor parses a "#RRGGBB" string, returning fallback if s is empty
// or malformed.
func parseHexColor(s string, fallback color.Color) color.Color {
	var r, g, b int
	if _, err := fmt.Sscanf(s, "#%02X%02X%02X", &r, &g, &b); err != nil {
		return fallback
	}
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}
}
