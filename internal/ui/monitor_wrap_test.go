package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestTappableLabelDoesNotAutoWrapByWidth(t *testing.T) {
	test.NewApp()
	col := theme.Color(theme.ColorNameForeground)
	longText := strings.Repeat("wordword ", 60) // no embedded newlines

	wide := newTappableLabel(nil)
	wide.Resize(fyne.NewSize(1000, 20))
	wide.SetText(longText, col, nil)

	narrow := newTappableLabel(nil)
	narrow.Resize(fyne.NewSize(60, 20))
	narrow.SetText(longText, col, nil)

	if narrow.MinHeight() != wide.MinHeight() {
		t.Fatalf("narrow column height %v should equal wide column height %v: automatic width-based wrapping should be disabled", narrow.MinHeight(), wide.MinHeight())
	}
}

func TestTappableLabelStillRespectsEmbeddedNewlines(t *testing.T) {
	test.NewApp()
	col := theme.Color(theme.ColorNameForeground)

	oneLine := newTappableLabel(nil)
	oneLine.Resize(fyne.NewSize(1000, 20))
	oneLine.SetText("01234567 89ABCDEF", col, nil)

	// Same content, but wrapped as asciiRepr would at a fixed byte count -
	// an explicit "\n" rather than relying on width.
	twoLines := newTappableLabel(nil)
	twoLines.Resize(fyne.NewSize(1000, 20))
	twoLines.SetText("01234567\n89ABCDEF", col, nil)

	if !(twoLines.MinHeight() > oneLine.MinHeight()) {
		t.Fatalf("explicit newline height %v should exceed single-line height %v even though width-based wrapping is off", twoLines.MinHeight(), oneLine.MinHeight())
	}
}

func TestTappableLabelSetTextSizeGrowsHeight(t *testing.T) {
	test.NewApp()
	col := theme.Color(theme.ColorNameForeground)

	l := newTappableLabel(nil)
	l.Resize(fyne.NewSize(200, 20))
	l.SetText("hello", col, nil)
	small := l.MinHeight()

	l.SetTextSize(40)
	big := l.MinHeight()

	if big <= small {
		t.Fatalf("MinHeight after growing font size = %v, want > %v", big, small)
	}
}
