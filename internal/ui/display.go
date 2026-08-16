package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// displaySettingsView lets the user customize the monitor's RX/TX colors and
// row font size.
type displaySettingsView struct {
	app *App

	rxSwatch     *colorSwatch
	txSwatch     *colorSwatch
	sizeSlider   *widget.Slider
	sizeValueLbl *widget.Label
}

func newDisplaySettingsView(app *App) *displaySettingsView {
	return &displaySettingsView{app: app}
}

func (d *displaySettingsView) CanvasObject() fyne.CanvasObject {
	d.rxSwatch = newColorSwatch(d.app.colors.RX, func() {
		d.pickColor("RX Color", &d.app.colors.RX, d.rxSwatch)
	})
	d.txSwatch = newColorSwatch(d.app.colors.TX, func() {
		d.pickColor("TX Color", &d.app.colors.TX, d.txSwatch)
	})

	d.sizeValueLbl = widget.NewLabel(fontSizeText(d.app.colors.RowFontSize))
	d.sizeSlider = widget.NewSlider(MinRowFontSize, MaxRowFontSize)
	d.sizeSlider.Value = float64(d.app.colors.RowFontSize)
	d.sizeSlider.OnChanged = func(v float64) {
		d.app.colors.RowFontSize = float32(v)
		d.sizeValueLbl.SetText(fontSizeText(d.app.colors.RowFontSize))
		d.app.monitor.Refresh()
	}
	d.sizeSlider.OnChangeEnded = func(float64) {
		d.app.colors.Save()
	}

	form := widget.NewForm(
		widget.NewFormItem("RX color", d.rxSwatch),
		widget.NewFormItem("TX color", d.txSwatch),
		widget.NewFormItem("Row font size", container.NewBorder(nil, nil, nil, d.sizeValueLbl, d.sizeSlider)),
	)

	resetBtn := widget.NewButtonWithIcon("Reset to default", theme.ContentUndoIcon(), func() {
		d.app.colors.ResetToDefault()
		d.rxSwatch.SetColor(d.app.colors.RX)
		d.txSwatch.SetColor(d.app.colors.TX)
		d.sizeSlider.Value = float64(d.app.colors.RowFontSize)
		d.sizeSlider.Refresh()
		d.sizeValueLbl.SetText(fontSizeText(d.app.colors.RowFontSize))
		d.app.monitor.Refresh()
	})

	return container.NewVBox(form, resetBtn)
}

func fontSizeText(size float32) string {
	return fmt.Sprintf("%.0fpt", size)
}

// pickColor opens a color picker for one of the RX/TX colors. target points
// directly at the colorSettings field being edited.
func (d *displaySettingsView) pickColor(title string, target *color.Color, swatch *colorSwatch) {
	picker := dialog.NewColorPicker(title, "Choose the color used for this in the monitor", func(c color.Color) {
		*target = c
		swatch.SetColor(c)
		d.app.colors.Save()
		d.app.monitor.Refresh()
	}, d.app.win)
	picker.Advanced = true
	picker.SetColor(*target)
	picker.Show()
}

// colorSwatch is a small tappable rectangle showing a color; tapping it
// invokes onTapped (used to open the color picker).
type colorSwatch struct {
	widget.BaseWidget
	rect     *canvas.Rectangle
	onTapped func()
}

func newColorSwatch(c color.Color, onTapped func()) *colorSwatch {
	s := &colorSwatch{rect: canvas.NewRectangle(c), onTapped: onTapped}
	s.rect.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	s.rect.StrokeWidth = 1
	s.rect.SetMinSize(fyne.NewSize(64, 28))
	s.ExtendBaseWidget(s)
	return s
}

// SetColor updates the swatch to show a new color.
func (s *colorSwatch) SetColor(c color.Color) {
	s.rect.FillColor = c
	s.rect.Refresh()
}

func (s *colorSwatch) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.rect)
}

func (s *colorSwatch) Tapped(*fyne.PointEvent) {
	if s.onTapped != nil {
		s.onTapped()
	}
}

func (s *colorSwatch) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}
