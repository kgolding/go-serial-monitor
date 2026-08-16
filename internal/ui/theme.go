package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// appTheme wraps the default theme with two small adjustments:
//
//   - Widens the presets/main split divider. The Fyne 2.8 built-in theme has
//     no case for theme.SizeNameSplitThickness (added in 2.8) in its Size()
//     switch, so it silently falls back to a bare 5px hit target - too thin
//     to reliably grab and drag with a real mouse, especially at high DPI.
//   - Renders disabled widget text (e.g. the serial settings fields while
//     connected) in the theme's placeholder color instead of its disabled
//     color. The built-in disabled color is nearly invisible against the
//     background in both light and dark variants, since it's designed for
//     things like disabled button fills, not for text a user still needs to
//     read.
type appTheme struct {
	fyne.Theme
}

func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameSplitThickness {
		return 10
	}
	return theme.DefaultTheme().Size(name)
}

func (appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameDisabled {
		name = theme.ColorNamePlaceHolder
	}
	return theme.DefaultTheme().Color(name, variant)
}

// Theme returns the application's theme: a more easily draggable split
// divider and more legible disabled/read-only field text than Fyne's
// default.
func Theme() fyne.Theme {
	return appTheme{Theme: theme.DefaultTheme()}
}

// compactRowTheme halves the theme's padding, which Fyne's widget.List uses
// as the gap between rows. Scoped to just the monitor's list via
// container.NewThemeOverride, so it tightens row spacing there without
// shrinking padding anywhere else in the app.
type compactRowTheme struct {
	fyne.Theme
}

func (t compactRowTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding {
		return t.Theme.Size(name) / 2
	}
	return t.Theme.Size(name)
}

// compactTheme returns the app theme with row spacing halved.
func compactTheme() fyne.Theme {
	return compactRowTheme{Theme: Theme()}
}
