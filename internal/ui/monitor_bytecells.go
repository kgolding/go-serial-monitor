package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// cellGap/cellLineGap are the margin between cells on a line and
	// between lines in a byteCellsView.
	cellGap     = 4
	cellLineGap = 4
	// iconHeightScale shrinks a control-char icon below the cell's full
	// line height, so it reads at the same visual size as a capital
	// letter's glyph ink instead of the taller line-height box (which
	// includes ascent/descent space a text glyph doesn't fill edge to
	// edge, but an SVG icon - drawn tight to its own bounds - would).
	iconHeightScale = 0.62
)

type asciiCellKind int

const (
	cellPrintable asciiCellKind = iota // literal byte, plain text
	cellControl                        // < 0x20, icon (ASCII column only)
	cellDot                            // >= 0x7f, plain "."
)

// classifyAsciiByte is the single source of truth for how one source byte
// renders in the ASCII column - shared by asciiRepr (the plain-text form,
// used for copy-to-clipboard) and byteCellsView (the per-byte widget
// form), so the two never disagree about which bytes are control bytes.
func classifyAsciiByte(c byte) (kind asciiCellKind, glyph string) {
	switch {
	case c < 0x20:
		return cellControl, controlCharNames[c]
	case c < 0x7f:
		return cellPrintable, string(c)
	default:
		return cellDot, "."
	}
}

// classifyHexByte renders byte b as its two-character uppercase hex code.
// The Hex column never shows icons, so this always reports cellPrintable
// regardless of the input byte's value.
func classifyHexByte(b byte) (kind asciiCellKind, glyph string) {
	return cellPrintable, fmt.Sprintf("%02X", b)
}

// byteCellsView renders data as one grid cell per byte, wrapping every
// wrapBytesPerLine bytes by construction (one grid cell per byte) - used
// for both the monitor's ASCII and Hex columns, via classify:
//
//   - newAsciiCellsView: printable bytes as plain monospace text, C0
//     control bytes (0x00-0x1F) as their icon from controlicons/, so
//     control characters read as a distinct glyph instead of inline
//     "<ACK>" text.
//   - newHexCellsView: every byte as its two-character uppercase hex code,
//     plain text only - giving the Hex column the same structural
//     wrap-every-8-bytes grid as the ASCII column (rather than a single
//     wrapped Label relying on matching rendered text length), so the two
//     columns' line counts and row height stay aligned by construction.
//
// Every cell (text or icon) occupies the same uniform width within a
// view, so columns of bytes line up regardless of which bytes are icons.
//
// widget.List recycles a small pool of these across many logical rows;
// SetData repopulates one from a []byte each time it scrolls into view.
// Rather than rebuilding the cell tree every call (a merged row can be up
// to maxMergedEntryBytes/wrapBytesPerLine = 512 lines), it keeps a pool of
// (canvas.Image, canvas.Text) pairs indexed by byte position that only
// grows, reusing already-built primitives - so a recycled row settles at
// the largest size it's ever shown and stops allocating.
type byteCellsView struct {
	widget.BaseWidget
	grid        *fyne.Container     // Layout: cellGridLayout; Objects = pairs[:2*len(data)]
	pairs       []fyne.CanvasObject // pool of (image, text) pairs, grows only
	cellAspects []float32           // per-cell icon aspect ratio (0 for text cells), grows only

	classify    func(byte) (asciiCellKind, string) // how a byte becomes a cell's kind/glyph
	sampleGlyph string                             // measured to size a plain-text cell: "M" (ascii) or "00" (hex)

	fontSize  float32
	cellSize  fyne.Size // one glyph's (width, line height)
	cellWidth float32   // uniform cell width - shared by both columns (see recomputeMetrics)
	onTapped  func()

	// peer is this row's other column (ascii<->hex), set once by createRow.
	// Hovering a cell here highlights the same byte index's cell on peer,
	// not on this view itself - see MouseMoved/highlightCell.
	peer             *byteCellsView
	highlightedIndex int // last cell index reported to peer, or -1
	highlightRect    *canvas.Rectangle
}

// newAsciiCellsView renders data with control bytes as icons - see
// byteCellsView's doc comment.
func newAsciiCellsView(onTapped func()) *byteCellsView {
	return newByteCellsView(classifyAsciiByte, "M", onTapped)
}

// newHexCellsView renders data as two-character hex codes, one per byte -
// see byteCellsView's doc comment.
func newHexCellsView(onTapped func()) *byteCellsView {
	return newByteCellsView(classifyHexByte, "00", onTapped)
}

func newByteCellsView(classify func(byte) (asciiCellKind, string), sampleGlyph string, onTapped func()) *byteCellsView {
	v := &byteCellsView{
		onTapped:         onTapped,
		fontSize:         theme.TextSize(),
		classify:         classify,
		sampleGlyph:      sampleGlyph,
		highlightedIndex: -1,
	}
	v.recomputeMetrics()
	v.grid = container.New(&cellGridLayout{view: v})
	v.highlightRect = canvas.NewRectangle(theme.Color(theme.ColorNameSelection))
	v.highlightRect.CornerRadius = 3
	v.highlightRect.Hide()
	v.ExtendBaseWidget(v)
	return v
}

// SetData repopulates the view for data. col is the row's RX/TX color,
// applied to both plain-text cells and icon cells - icons are recolored
// (via controlIcon in controlicons.go) to match, rather than a fixed
// theme color, so a control byte reads as part of the same colored row as
// the bytes around it.
//
// Called by updateRow every time a pooled row scrolls into view holding
// different data, so it also clears any hover highlight left over from
// what this pooled instance previously displayed - otherwise a highlight
// can keep pointing at a stale index if the mouse sits still while the
// data underneath it changes (no new MouseMoved event to correct it).
// This clears two independent things: v's own highlight rect (shown when
// v's peer was the hover source) and, if v itself was the hover source,
// tells the peer to clear its rect too.
func (v *byteCellsView) SetData(data []byte, col color.Color, onTapped func()) {
	v.onTapped = onTapped
	v.clearHighlight()
	if v.highlightedIndex != -1 {
		v.highlightedIndex = -1
		if v.peer != nil {
			v.peer.clearHighlight()
		}
	}
	need := len(data) * 2
	for len(v.pairs) < need {
		img := canvas.NewImageFromResource(nil)
		img.FillMode = canvas.ImageFillContain
		img.Hidden = true
		text := canvas.NewText("", col)
		text.TextStyle = fyne.TextStyle{Monospace: true}
		text.Alignment = fyne.TextAlignCenter
		v.pairs = append(v.pairs, img, text)
		v.cellAspects = append(v.cellAspects, 0)
	}
	for i, b := range data {
		img := v.pairs[i*2].(*canvas.Image)
		text := v.pairs[i*2+1].(*canvas.Text)
		kind, glyph := v.classify(b)
		if kind == cellControl {
			img.Resource = controlIcon(b, col)
			img.Hidden = false
			text.Hidden = true
			v.cellAspects[i] = controlIconAspects[b]
		} else {
			text.Text = glyph
			text.Color = col
			text.TextSize = v.fontSize
			img.Hidden = true
			text.Hidden = false
			v.cellAspects[i] = 0
		}
	}
	v.grid.Objects = v.pairs[:need] // reslice, no realloc of the pool
	v.grid.Refresh()
}

// SetTextSize updates the font size used for plain-text cells and
// recomputes cached cell dimensions (including the icon height derived
// from it, so icons grow/shrink alongside the text).
func (v *byteCellsView) SetTextSize(size float32) {
	if size <= 0 || v.fontSize == size {
		return
	}
	v.fontSize = size
	v.recomputeMetrics()
}

// recomputeMetrics measures this view's own glyph width but always sizes
// cellWidth against the icon-accommodating width (using the shared
// maxControlIconAspect, regardless of whether this view's classify ever
// returns cellControl), so the Hex column's cells end up the same width
// as the ASCII column's - unused space reads as margin around each
// two-character hex code rather than the two columns drifting to
// different cell widths.
func (v *byteCellsView) recomputeMetrics() {
	v.cellSize = fyne.MeasureText(v.sampleGlyph, v.fontSize, fyne.TextStyle{Monospace: true})
	iconHeight := v.cellSize.Height * iconHeightScale

	v.cellWidth = v.cellSize.Width
	if w := iconHeight * maxControlIconAspect; w > v.cellWidth {
		v.cellWidth = w
	}
}

// MinHeight returns the height needed to show the current data, wrapped
// at wrapBytesPerLine bytes per line - used to grow the row (via
// widget.List.SetItemHeight) to fit however many lines the data needs.
func (v *byteCellsView) MinHeight() float32 {
	n := len(v.grid.Objects) / 2
	if n == 0 {
		return 0
	}
	lines := (n + wrapBytesPerLine - 1) / wrapBytesPerLine
	return float32(lines)*(v.cellSize.Height+cellLineGap) - cellLineGap
}

// CreateRenderer layers the highlight rect behind v.grid via overlayLayout,
// rather than widget.NewSimpleRenderer(v.grid) alone (which only supports
// one object) - see overlayLayout's doc comment.
func (v *byteCellsView) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.New(overlayLayout{}, v.highlightRect, v.grid))
}

func (v *byteCellsView) Tapped(*fyne.PointEvent) {
	if v.onTapped != nil {
		v.onTapped()
	}
}

func (v *byteCellsView) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (v *byteCellsView) MouseIn(ev *desktop.MouseEvent) {
	v.updateHover(ev.Position)
}

func (v *byteCellsView) MouseMoved(ev *desktop.MouseEvent) {
	v.updateHover(ev.Position)
}

func (v *byteCellsView) MouseOut() {
	if v.highlightedIndex == -1 {
		return
	}
	v.highlightedIndex = -1
	if v.peer != nil {
		v.peer.clearHighlight()
	}
}

// updateHover inverts cellGridLayout.Layout's forward math to find which
// cell (if any) pos falls in, and - since hovering a cell highlights the
// *other* column's corresponding cell, not this one - tells v.peer to
// show or hide its highlight accordingly. v.highlightedIndex is bookkeeping
// only (which cell v last reported as hovered), not something rendered on
// v itself.
func (v *byteCellsView) updateHover(pos fyne.Position) {
	col := int(pos.X / (v.cellWidth + cellGap))
	row := int(pos.Y / (v.cellSize.Height + cellLineGap))
	n := len(v.grid.Objects) / 2
	idx := row*wrapBytesPerLine + col
	if col < 0 || col >= wrapBytesPerLine || row < 0 || idx >= n {
		v.MouseOut()
		return
	}
	if idx == v.highlightedIndex {
		return
	}
	v.highlightedIndex = idx
	if v.peer != nil {
		v.peer.highlightCell(idx)
	}
}

// highlightCell shows this view's highlight rect over cell idx, using the
// same position math as cellGridLayout.Layout. Uses v.Refresh() (the
// widget-level refresh), not v.highlightRect.Refresh() alone - a bare
// Refresh() on a primitive that's newly transitioning Hidden->visible
// isn't picked up by Fyne's renderer until the owning widget is told to
// redraw, confirmed by manual testing against the real GL driver (a gap
// headless widget tests can't catch, since they don't exercise painting).
func (v *byteCellsView) highlightCell(idx int) {
	row := idx / wrapBytesPerLine
	col := idx % wrapBytesPerLine
	x := float32(col) * (v.cellWidth + cellGap)
	y := float32(row) * (v.cellSize.Height + cellLineGap)
	v.highlightRect.Move(fyne.NewPos(x, y))
	v.highlightRect.Resize(fyne.NewSize(v.cellWidth, v.cellSize.Height))
	v.highlightRect.Show()
	v.Refresh()
}

func (v *byteCellsView) clearHighlight() {
	v.highlightRect.Hide()
	v.Refresh()
}

// overlayLayout stretches objects[1] (v.grid) to fill the available size,
// exactly as widget.NewSimpleRenderer(v.grid) did before the highlight
// rect existed, and leaves objects[0] (the highlight rect) untouched -
// its position/size are set manually by highlightCell whenever the
// hovered cell changes. Needed because widget.NewSimpleRenderer only
// supports a single object, so a second freestanding overlay object
// requires a (trivial) custom fyne.Layout, the same pattern already used
// for weightedRowLayout/cellGridLayout in this file.
type overlayLayout struct{}

func (overlayLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return objects[1].MinSize()
}

func (overlayLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[1].Move(fyne.NewPos(0, 0))
	objects[1].Resize(size)
}

// cellGridLayout arranges byteCellsView's (image, text) primitive pairs
// into rows of wrapBytesPerLine cells, each view.cellWidth wide - text and
// icon cells share the same width so columns of bytes stay aligned
// regardless of which bytes are icons. An icon is drawn at
// iconHeightScale of the line height (see byteCellsView's doc comment)
// and centered within its cell, both horizontally and vertically. Wrapping
// is always by cell count, never by pixel width, so it doesn't respond to
// the incoming layout size - keeping the ASCII/Hex columns' line counts
// identical regardless of column width.
type cellGridLayout struct {
	view *byteCellsView
}

func (g *cellGridLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	n := len(objects) / 2
	if n == 0 {
		return fyne.NewSize(0, 0)
	}
	lines := (n + wrapBytesPerLine - 1) / wrapBytesPerLine
	perLine := n
	if perLine > wrapBytesPerLine {
		perLine = wrapBytesPerLine
	}
	width := float32(perLine)*(g.view.cellWidth+cellGap) - cellGap
	return fyne.NewSize(width, float32(lines)*(g.view.cellSize.Height+cellLineGap)-cellLineGap)
}

func (g *cellGridLayout) Layout(objects []fyne.CanvasObject, _ fyne.Size) {
	cellW := g.view.cellWidth
	cellH := g.view.cellSize.Height
	iconH := cellH * iconHeightScale

	var x, y float32
	for i := 0; i < len(objects); i += 2 {
		cellIdx := i / 2
		if cellIdx > 0 && cellIdx%wrapBytesPerLine == 0 {
			x = 0
			y += cellH + cellLineGap
		}

		img := objects[i]
		text := objects[i+1]

		// Position both objects every pass, even the hidden one, so
		// neither is left holding a stale position/size from whatever
		// this pooled pair last rendered as.
		iconW := iconH * g.view.cellAspects[cellIdx]
		img.Move(fyne.NewPos(x+(cellW-iconW)/2, y+(cellH-iconH)/2))
		img.Resize(fyne.NewSize(iconW, iconH))
		text.Move(fyne.NewPos(x, y))
		text.Resize(fyne.NewSize(cellW, cellH))

		x += cellW + cellGap
	}
}
