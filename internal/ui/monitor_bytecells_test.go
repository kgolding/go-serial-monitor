package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// linkedCellsViewPair builds an ascii/hex pair peer-linked and populated
// exactly as monitor.go's createRow/updateRow would, for hover tests.
func linkedCellsViewPair(data []byte) (ascii, hex *byteCellsView) {
	ascii = newAsciiCellsView(nil)
	hex = newHexCellsView(nil)
	ascii.peer, hex.peer = hex, ascii
	col := theme.Color(theme.ColorNameForeground)
	ascii.SetData(data, col, nil)
	hex.SetData(data, col, nil)
	return ascii, hex
}

// cellCenter returns a position inside cell idx of v, for feeding to
// MouseMoved/MouseIn in tests - mirrors highlightCell's own position math.
func cellCenter(v *byteCellsView, idx int) fyne.Position {
	row := idx / wrapBytesPerLine
	col := idx % wrapBytesPerLine
	x := float32(col)*(v.cellWidth+cellGap) + v.cellWidth/2
	y := float32(row)*(v.cellSize.Height+cellLineGap) + v.cellSize.Height/2
	return fyne.NewPos(x, y)
}

func moveMouse(v *byteCellsView, pos fyne.Position) {
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: pos}})
}

func TestClassifyAsciiByte(t *testing.T) {
	cases := []struct {
		b        byte
		wantKind asciiCellKind
	}{
		{0x00, cellControl}, {0x06, cellControl}, {0x1B, cellControl}, {0x1F, cellControl},
		{'A', cellPrintable}, {' ', cellPrintable}, {'~', cellPrintable},
		{0x7f, cellDot}, {0xff, cellDot},
	}
	for _, c := range cases {
		if kind, _ := classifyAsciiByte(c.b); kind != c.wantKind {
			t.Errorf("classifyAsciiByte(0x%02X) kind = %v, want %v", c.b, kind, c.wantKind)
		}
	}
}

func TestClassifyHexByteNeverReturnsControl(t *testing.T) {
	for b := 0; b < 256; b++ {
		kind, glyph := classifyHexByte(byte(b))
		if kind != cellPrintable {
			t.Fatalf("classifyHexByte(0x%02X) kind = %v, want cellPrintable (hex never shows icons)", b, kind)
		}
		want := fmt.Sprintf("%02X", b)
		if glyph != want {
			t.Errorf("classifyHexByte(0x%02X) glyph = %q, want %q", b, glyph, want)
		}
	}
}

func TestControlIconsFullyPopulated(t *testing.T) {
	for b := 0; b < 0x20; b++ {
		if len(controlIconTemplates[b]) == 0 {
			t.Errorf("controlIconTemplates[0x%02X] is empty", b)
		}
		if controlIconAspects[b] <= 0 {
			t.Errorf("controlIconAspects[0x%02X] = %v, want > 0", b, controlIconAspects[b])
		}
		if controlIcon(byte(b), theme.Color(theme.ColorNameForeground)) == nil {
			t.Errorf("controlIcon(0x%02X, ...) is nil", b)
		}
	}
}

func TestControlIconRecolorsToRequestedColor(t *testing.T) {
	red := color.NRGBA{R: 0xFF, A: 0xFF}
	blue := color.NRGBA{B: 0xFF, A: 0xFF}

	redRes := controlIcon(0x06, red)
	blueRes := controlIcon(0x06, blue)

	if redRes == blueRes {
		t.Fatal("controlIcon should return a distinct resource per color")
	}
	if !strings.Contains(string(redRes.Content()), colorToHex(red)) {
		t.Errorf("red icon content should contain %s", colorToHex(red))
	}
	if !strings.Contains(string(blueRes.Content()), colorToHex(blue)) {
		t.Errorf("blue icon content should contain %s", colorToHex(blue))
	}

	// Requesting the same (byte, color) again should hit the cache and
	// return the identical resource, not a fresh equal-but-distinct one -
	// that's what lets Fyne reuse its rasterized SVG cache.
	if again := controlIcon(0x06, red); again != redRes {
		t.Error("controlIcon should return the cached resource for a repeated (byte, color)")
	}
}

func TestAsciiCellsViewControlByteShowsIconHidesText(t *testing.T) {
	test.NewApp()
	col := theme.Color(theme.ColorNameForeground)
	v := newAsciiCellsView(nil)
	v.SetData([]byte{0x06}, col, nil)

	img := v.grid.Objects[0].(*canvas.Image)
	text := v.grid.Objects[1].(*canvas.Text)
	if img.Hidden {
		t.Error("control byte: icon image should be visible")
	}
	if img.Resource != controlIcon(0x06, col) {
		t.Error("control byte: icon resource should be controlIcon(0x06, col)")
	}
	if !text.Hidden {
		t.Error("control byte: text cell should be hidden")
	}
}

func TestAsciiCellsViewPrintableByteShowsTextHidesIcon(t *testing.T) {
	test.NewApp()
	v := newAsciiCellsView(nil)
	v.SetData([]byte("A"), theme.Color(theme.ColorNameForeground), nil)

	img := v.grid.Objects[0].(*canvas.Image)
	text := v.grid.Objects[1].(*canvas.Text)
	if !img.Hidden {
		t.Error("printable byte: icon image should be hidden")
	}
	if text.Hidden {
		t.Error("printable byte: text cell should be visible")
	}
	if text.Text != "A" {
		t.Errorf("printable byte: text = %q, want %q", text.Text, "A")
	}
}

func TestHexCellsViewNeverShowsIcon(t *testing.T) {
	test.NewApp()
	v := newHexCellsView(nil)
	// 0x06 would be a control byte in the ASCII column; the Hex column
	// must always render it as plain text "06", never an icon.
	v.SetData([]byte{0x06, 'A'}, theme.Color(theme.ColorNameForeground), nil)

	for cell, want := range map[int]string{0: "06", 1: "41"} {
		img := v.grid.Objects[cell*2].(*canvas.Image)
		text := v.grid.Objects[cell*2+1].(*canvas.Text)
		if !img.Hidden {
			t.Errorf("cell %d: icon image should always be hidden in the Hex column", cell)
		}
		if text.Hidden {
			t.Errorf("cell %d: text cell should be visible", cell)
		}
		if text.Text != want {
			t.Errorf("cell %d: text = %q, want %q", cell, text.Text, want)
		}
	}
}

func TestByteCellsViewLineCountsForBothColumns(t *testing.T) {
	test.NewApp()
	sizes := []int{0, 1, 7, 8, 9, 4096}
	for _, n := range sizes {
		data := make([]byte, n)
		for i := range data {
			if i%3 == 0 {
				data[i] = 0x06 // mix in control bytes (ASCII-only concept)
			} else {
				data[i] = byte('A' + i%26)
			}
		}

		wantLines := 0
		if n > 0 {
			wantLines = (n + wrapBytesPerLine - 1) / wrapBytesPerLine
		}

		for name, newView := range map[string]func(func()) *byteCellsView{
			"ascii": newAsciiCellsView,
			"hex":   newHexCellsView,
		} {
			v := newView(nil)
			v.SetData(data, theme.Color(theme.ColorNameForeground), nil)

			wantHeight := float32(0)
			if wantLines > 0 {
				wantHeight = float32(wantLines)*(v.cellSize.Height+cellLineGap) - cellLineGap
			}
			if got := v.MinHeight(); got != wantHeight {
				t.Errorf("%s n=%d: MinHeight() = %v, want %v (%d lines)", name, n, got, wantHeight, wantLines)
			}
		}
	}
}

func TestAsciiAndHexCellsViewsStayRowAligned(t *testing.T) {
	// The ASCII and Hex columns must wrap at the exact same byte boundary
	// for a given entry, or their rows drift apart vertically. Since both
	// are now the same byteCellsView grid (just different classify
	// functions), their line counts for identical data are guaranteed
	// equal by construction - verify that directly, rather than via each
	// column's own independent wrapBytesPerLine math.
	data := []byte("Hi\x06\x07there World! more data here")
	ascii := newAsciiCellsView(nil)
	hex := newHexCellsView(nil)
	ascii.SetData(data, theme.Color(theme.ColorNameForeground), nil)
	hex.SetData(data, theme.Color(theme.ColorNameForeground), nil)

	asciiLines := (len(ascii.grid.Objects)/2 + wrapBytesPerLine - 1) / wrapBytesPerLine
	hexLines := (len(hex.grid.Objects)/2 + wrapBytesPerLine - 1) / wrapBytesPerLine
	if asciiLines != hexLines {
		t.Errorf("ascii produced %d lines, hex produced %d lines for the same data; want equal", asciiLines, hexLines)
	}
}

func TestAsciiAndHexCellsViewsShareCellWidth(t *testing.T) {
	// A Hex cell ("41") is narrower than an ASCII column's icon-
	// accommodating cell width, so the Hex column needs extra margin
	// reserved around its text to match - otherwise the two columns'
	// cell grids have different pitches even though they show the same
	// number of cells per line.
	test.NewApp()
	ascii := newAsciiCellsView(nil)
	hex := newHexCellsView(nil)

	if ascii.cellWidth != hex.cellWidth {
		t.Errorf("ascii cellWidth = %v, hex cellWidth = %v, want equal", ascii.cellWidth, hex.cellWidth)
	}

	// The relationship should hold after a font size change too, not just
	// at construction.
	ascii.SetTextSize(28)
	hex.SetTextSize(28)
	if ascii.cellWidth != hex.cellWidth {
		t.Errorf("after SetTextSize: ascii cellWidth = %v, hex cellWidth = %v, want equal", ascii.cellWidth, hex.cellWidth)
	}
}

func TestAsciiCellsViewPoolGrowsMonotonically(t *testing.T) {
	test.NewApp()
	v := newAsciiCellsView(nil)

	v.SetData(make([]byte, 100), theme.Color(theme.ColorNameForeground), nil)
	big := len(v.pairs)

	v.SetData(make([]byte, 5), theme.Color(theme.ColorNameForeground), nil)
	if len(v.pairs) != big {
		t.Fatalf("pool shrank after smaller SetData: got %d, want %d", len(v.pairs), big)
	}

	v.SetData(make([]byte, 100), theme.Color(theme.ColorNameForeground), nil)
	if len(v.pairs) != big {
		t.Fatalf("pool grew again for a size it already reached: got %d, want %d", len(v.pairs), big)
	}
}

func TestAsciiCellsViewSetTextSizeGrowsHeight(t *testing.T) {
	test.NewApp()
	v := newAsciiCellsView(nil)
	v.SetData([]byte("hello"), theme.Color(theme.ColorNameForeground), nil)
	small := v.MinHeight()

	v.SetTextSize(40)
	v.SetData([]byte("hello"), theme.Color(theme.ColorNameForeground), nil)
	big := v.MinHeight()

	if big <= small {
		t.Fatalf("MinHeight after growing font size = %v, want > %v", big, small)
	}
}

func TestAsciiCellsViewCellsShareUniformWidth(t *testing.T) {
	test.NewApp()
	v := newAsciiCellsView(nil)
	v.SetData([]byte{'A', 0x06, 'B', 0x1B}, theme.Color(theme.ColorNameForeground), nil)
	v.grid.Resize(fyne.NewSize(1000, 1000)) // force a layout pass

	// Text cells fill their whole slot; icon cells are centered within a
	// same-width slot but narrower, preserving their own aspect ratio - so
	// what must match across cell kinds is the slot pitch (x-advance
	// between consecutive cells), not the drawn icon's own width. Read the
	// text object's position (index i*2+1): Layout always positions it,
	// whereas the image object (i*2) is only moved for icon cells.
	cellPos := func(i int) float32 { return v.grid.Objects[i*2+1].Position().X }
	wantPitch := v.cellWidth + cellGap
	for i := 0; i < 3; i++ {
		if pitch := cellPos(i+1) - cellPos(i); pitch != wantPitch {
			t.Errorf("cell %d->%d pitch = %v, want %v", i, i+1, pitch, wantPitch)
		}
	}

	iconCell := v.grid.Objects[2] // 0x06, an icon cell
	wantIconHeight := v.cellSize.Height * iconHeightScale
	if got := iconCell.Size().Height; got != wantIconHeight {
		t.Errorf("icon height = %v, want %v (%v of the text line height)", got, wantIconHeight, iconHeightScale)
	}
	if got := iconCell.Size().Width; got > v.cellWidth {
		t.Errorf("icon width = %v, should not exceed the cell width %v", got, v.cellWidth)
	}
}

func TestAsciiCellsViewTappedInvokesCallback(t *testing.T) {
	test.NewApp()
	calls := 0
	v := newAsciiCellsView(nil)
	v.SetData([]byte("hi"), theme.Color(theme.ColorNameForeground), func() { calls++ })

	v.Tapped(nil)

	if calls != 1 {
		t.Fatalf("onTapped called %d times, want 1", calls)
	}
}

func TestByteCellsViewHoverHighlightsPeerCellOnly(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCDEFGHIJ")) // 10 bytes, 2 lines

	const idx = 3
	moveMouse(ascii, cellCenter(ascii, idx))

	if !hex.highlightRect.Visible() {
		t.Fatal("hex highlightRect should be visible after hovering the corresponding ascii cell")
	}
	if ascii.highlightRect.Visible() {
		t.Error("ascii highlightRect should stay hidden - hovering ascii highlights only its peer (hex)")
	}
	wantPos := fyne.NewPos(
		float32(idx%wrapBytesPerLine)*(hex.cellWidth+cellGap),
		float32(idx/wrapBytesPerLine)*(hex.cellSize.Height+cellLineGap),
	)
	if got := hex.highlightRect.Position(); got != wantPos {
		t.Errorf("hex highlightRect position = %v, want %v", got, wantPos)
	}
	wantSize := fyne.NewSize(hex.cellWidth, hex.cellSize.Height)
	if got := hex.highlightRect.Size(); got != wantSize {
		t.Errorf("hex highlightRect size = %v, want %v", got, wantSize)
	}
}

func TestByteCellsViewHoverIsBidirectional(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCDEFGH"))

	moveMouse(hex, cellCenter(hex, 5))

	if !ascii.highlightRect.Visible() {
		t.Fatal("ascii highlightRect should be visible after hovering the corresponding hex cell")
	}
	if hex.highlightRect.Visible() {
		t.Error("hex highlightRect should stay hidden - hovering hex highlights only its peer (ascii)")
	}
}

func TestByteCellsViewHoverMovingBetweenCellsUpdatesSameRect(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCDEFGHIJ"))

	moveMouse(ascii, cellCenter(ascii, 1))
	firstPos := hex.highlightRect.Position()
	rect := hex.highlightRect

	moveMouse(ascii, cellCenter(ascii, 6))
	if hex.highlightRect != rect {
		t.Fatal("hovering a different cell should reuse the same highlightRect, not create a new one")
	}
	if hex.highlightRect.Position() == firstPos {
		t.Error("highlightRect position should have moved to the new cell")
	}
	if !hex.highlightRect.Visible() {
		t.Error("highlightRect should still be visible while hovering a valid cell")
	}
}

func TestByteCellsViewMouseOutClearsPeerHighlight(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCDEFGH"))

	moveMouse(ascii, cellCenter(ascii, 2))
	if !hex.highlightRect.Visible() {
		t.Fatal("setup: hex highlightRect should be visible")
	}

	ascii.MouseOut()

	if hex.highlightRect.Visible() {
		t.Error("hex highlightRect should be hidden after MouseOut on ascii")
	}
}

func TestByteCellsViewHoverDeadSpaceClearsHighlight(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCD")) // 4 bytes: one short line

	moveMouse(ascii, cellCenter(ascii, 1))
	if !hex.highlightRect.Visible() {
		t.Fatal("setup: hex highlightRect should be visible")
	}

	// Far past the 4 real cells and the 8-wide line - stretched dead space,
	// same as ascii/hex columns being resized to their full weighted
	// column width regardless of actual content width.
	moveMouse(ascii, fyne.NewPos(ascii.cellWidth*20, ascii.cellSize.Height*20))

	if hex.highlightRect.Visible() {
		t.Error("hovering dead space should clear the peer's highlight")
	}
}

func TestByteCellsViewSetDataClearsStaleHighlight(t *testing.T) {
	test.NewApp()
	ascii, hex := linkedCellsViewPair([]byte("ABCDEFGH"))

	moveMouse(ascii, cellCenter(ascii, 2))
	if !hex.highlightRect.Visible() {
		t.Fatal("setup: hex highlightRect should be visible")
	}

	// Simulate the row scrolling to new data while the mouse sits still -
	// no new MouseMoved event fires, so SetData itself must clear things.
	ascii.SetData([]byte("NEWDATA!"), theme.Color(theme.ColorNameForeground), nil)

	if hex.highlightRect.Visible() {
		t.Error("hex highlightRect should be cleared when ascii's row is recycled with new data")
	}
	if ascii.highlightedIndex != -1 {
		t.Errorf("ascii.highlightedIndex = %d, want -1 after SetData", ascii.highlightedIndex)
	}
}
