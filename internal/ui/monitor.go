package ui

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/pcapng"
)

type logEntry struct {
	Time     time.Time // when the row started (first chunk's arrival)
	LastTime time.Time // when the row last grew (most recent chunk's arrival)
	TX       bool      // false = received (RX), true = sent (TX)
	Data     []byte
}

// maxLogEntries caps how many lines the monitor keeps, so a busy port
// running unattended for hours doesn't grow memory without bound.
const maxLogEntries = 5000

// maxMergedEntryBytes caps how large a single merged row can grow, so a
// device that streams continuously with small gaps can't turn into one
// unbounded line.
const maxMergedEntryBytes = 4096

// asciiHexSplit is the width ratio given to the ASCII column vs. the Hex
// column, after the timestamp column. Equal, so a wrapBytesPerLine-wide
// row of hex (which needs more horizontal space per byte than ASCII) sets
// the column width and ASCII just has room to spare.
const (
	asciiColumnWeight = 0.5
	hexColumnWeight   = 0.5
)

// wrapBytesPerLine is how many source bytes each byteCellsView (the ASCII
// and Hex columns) puts on each line. Both columns use the same grid
// mechanism at the same wrap count, which keeps them vertically aligned
// line-for-line by construction, rather than each wrapping independently
// based on its own rendered width.
const wrapBytesPerLine = 8

// monitorView is the scrolling RX/TX log panel. Each row shows a timestamp,
// then the data as ASCII (40% width) and Hex (60% width); clicking either
// copies that representation to the clipboard.
type monitorView struct {
	win        fyne.Window
	colors     *colorSettings
	entries    []logEntry
	autoscroll bool
	paused     bool
	portName   string // last-connected serial port, tagged into pcap exports

	list      *widget.List
	exportBtn *widget.Button
}

func newMonitorView(win fyne.Window, colors *colorSettings) *monitorView {
	return &monitorView{win: win, colors: colors, autoscroll: true}
}

// CanvasObject builds the toolbar + scrolling log view.
func (m *monitorView) CanvasObject() fyne.CanvasObject {
	m.list = widget.NewList(
		func() int { return len(m.entries) },
		m.createRow,
		m.updateRow,
	)

	autoscrollCheck := widget.NewCheck("Autoscroll", func(b bool) {
		m.autoscroll = b
		if b {
			m.list.ScrollToBottom()
		}
	})
	autoscrollCheck.Checked = true

	pauseCheck := widget.NewCheck("Pause", func(b bool) {
		m.paused = b
	})

	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		m.Clear()
	})

	m.exportBtn = widget.NewButtonWithIcon("Export", theme.DownloadIcon(), m.showExportDialog)
	m.exportBtn.Disable()

	toolbar := container.NewHBox(
		layoutSpacer(),
		autoscrollCheck, pauseCheck, clearBtn, m.exportBtn,
	)

	// Halve the row-to-row gap, scoped to just this list.
	compactList := container.NewThemeOverride(m.list, compactTheme())

	return container.NewBorder(toolbar, nil, nil, nil, compactList)
}

func (m *monitorView) createRow() fyne.CanvasObject {
	ts := canvas.NewText("", theme.Color(theme.ColorNameForeground))
	ts.TextStyle = fyne.TextStyle{Monospace: true}
	ts.TextSize = m.colors.RowFontSize

	ascii := newAsciiCellsView(nil)
	ascii.SetTextSize(m.colors.RowFontSize)
	hex := newHexCellsView(nil)
	hex.SetTextSize(m.colors.RowFontSize)
	ascii.peer, hex.peer = hex, ascii
	data := container.New(weightedRowLayout{}, ascii, hex)

	return container.NewBorder(nil, nil, ts, nil, data)
}

func (m *monitorView) updateRow(id widget.ListItemID, obj fyne.CanvasObject) {
	e := m.entries[id]

	row := obj.(*fyne.Container)
	data := row.Objects[0].(*fyne.Container)
	ts := row.Objects[1].(*canvas.Text)

	dir := "RX"
	col := m.colors.RX
	if e.TX {
		dir = "TX"
		col = m.colors.TX
	}
	ts.Text = fmt.Sprintf("[%s] %-2s ", e.Time.Format("15:04:05.000"), dir)
	ts.Color = col
	ts.TextSize = m.colors.RowFontSize
	ts.Refresh()

	ascii := data.Objects[0].(*byteCellsView)
	ascii.SetTextSize(m.colors.RowFontSize)
	ascii.SetData(e.Data, col, func() {
		m.copyToClipboard(string(e.Data))
	})

	hex := data.Objects[1].(*byteCellsView)
	hex.SetTextSize(m.colors.RowFontSize)
	hex.SetData(e.Data, col, func() {
		m.copyToClipboard(hexArrayRepr(e.Data))
	})

	// Grow the row to fit however many lines the ASCII/Hex columns wrapped
	// to at their current (already-laid-out) width, so wrapped content is
	// never clipped.
	rowHeight := ascii.MinHeight()
	if h := hex.MinHeight(); h > rowHeight {
		rowHeight = h
	}
	if h := ts.MinSize().Height; h > rowHeight {
		rowHeight = h
	}
	if rowHeight > 0 {
		m.list.SetItemHeight(id, rowHeight)
	}
}

// Refresh redraws the log, e.g. after the RX/TX display colors change.
func (m *monitorView) Refresh() {
	if m.list != nil {
		m.list.Refresh()
	}
}

func (m *monitorView) copyToClipboard(text string) {
	if m.win == nil || text == "" {
		return
	}
	m.win.Clipboard().SetContent(text)
}

// Append adds one entry (a single read chunk, or one send operation) to the
// log. RX chunks arriving within mergeWindow of the previous RX chunk are
// appended onto that row instead of starting a new one; pass 0 to always
// start a new row (used for TX, and when disconnected).
func (m *monitorView) Append(tx bool, data []byte, t time.Time, mergeWindow time.Duration) {
	if m.paused {
		return
	}
	if !tx && mergeWindow > 0 && len(m.entries) > 0 {
		last := &m.entries[len(m.entries)-1]
		if !last.TX && len(last.Data) < maxMergedEntryBytes && t.Sub(last.LastTime) <= mergeWindow {
			last.Data = append(last.Data, data...)
			last.LastTime = t
			m.list.Refresh()
			if m.autoscroll {
				m.list.ScrollToBottom()
			}
			return
		}
	}
	m.entries = append(m.entries, logEntry{Time: t, LastTime: t, TX: tx, Data: data})
	if len(m.entries) > maxLogEntries {
		m.entries = m.entries[len(m.entries)-maxLogEntries:]
	}
	m.updateExportEnabled()
	m.list.Refresh()
	if m.autoscroll {
		m.list.ScrollToBottom()
	}
}

// Clear empties the log.
func (m *monitorView) Clear() {
	m.entries = nil
	m.updateExportEnabled()
	m.list.Refresh()
}

// updateExportEnabled enables the Export button once the log has at least
// one entry, mirroring how sendPanel/presetsView gate their own actions.
func (m *monitorView) updateExportEnabled() {
	if m.exportBtn == nil {
		return
	}
	if len(m.entries) == 0 {
		m.exportBtn.Disable()
	} else {
		m.exportBtn.Enable()
	}
}

// SetPortName records the currently (or most recently) connected serial
// port, used to tag pcap exports' interface name. Safe to call with "".
func (m *monitorView) SetPortName(name string) {
	m.portName = name
}

// ExportPCAP writes every held log entry to w as a pcapng capture file, one
// packet per row (using each row's start time, matching what's displayed),
// tagged with its RX/TX direction so Wireshark shows an inbound/outbound
// arrow per packet.
func (m *monitorView) ExportPCAP(w io.Writer) error {
	pw, err := pcapng.NewWriter(w, m.portName)
	if err != nil {
		return err
	}
	for _, e := range m.entries {
		dir := pcapng.DirectionRX
		if e.TX {
			dir = pcapng.DirectionTX
		}
		if err := pw.WritePacket(e.Time, dir, e.Data); err != nil {
			return err
		}
	}
	return nil
}

// showExportDialog prompts for a save location and writes the current log
// to it as pcapng.
func (m *monitorView) showExportDialog() {
	if len(m.entries) == 0 {
		return
	}
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, m.win)
			return
		}
		if writer == nil {
			return // user cancelled
		}
		defer writer.Close()
		if err := m.ExportPCAP(writer); err != nil {
			dialog.ShowError(err, m.win)
		}
	}, m.win)
	d.SetFileName(fmt.Sprintf("serial-log-%s.pcapng", time.Now().Format("20060102-150405")))
	d.SetFilter(storage.NewExtensionFileFilter([]string{".pcapng"}))
	d.Show()
}

// asciiRepr renders data as text, escaping CR/LF/TAB visibly and replacing
// other non-printable bytes with a dot - used for the ASCII column's
// copy-to-clipboard value. A newline is inserted every wrapBytesPerLine
// bytes, matching the on-screen byteCellsView wrapping.
func asciiRepr(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for i, c := range data {
		if i > 0 && i%wrapBytesPerLine == 0 {
			b.WriteByte('\n')
		}
		switch c {
		case '\r':
			b.WriteString(`\r`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			kind, glyph := classifyAsciiByte(c)
			if kind == cellControl {
				b.WriteByte('<')
				b.WriteString(glyph)
				b.WriteByte('>')
			} else {
				b.WriteString(glyph)
			}
		}
	}
	return b.String()
}

// controlCharNames maps the C0 control codes (0x00-0x1F) to their standard
// abbreviations, rendered as e.g. "<ACK>" in the ASCII view. \r, \n and \t
// are rendered via their familiar escape shorthand instead (handled above)
// so are left as the zero value here and never looked up.
var controlCharNames = [0x20]string{
	0x00: "NUL", 0x01: "SOH", 0x02: "STX", 0x03: "ETX",
	0x04: "EOT", 0x05: "ENQ", 0x06: "ACK", 0x07: "BEL",
	0x08: "BS", 0x0B: "VT", 0x0C: "FF",
	0x0E: "SO", 0x0F: "SI",
	0x10: "DLE", 0x11: "DC1", 0x12: "DC2", 0x13: "DC3",
	0x14: "DC4", 0x15: "NAK", 0x16: "SYN", 0x17: "ETB",
	0x18: "CAN", 0x19: "EM", 0x1A: "SUB", 0x1B: "ESC",
	0x1C: "FS", 0x1D: "GS", 0x1E: "RS", 0x1F: "US",
}

// hexArrayRepr renders data as a "[0x01, 0x02]"-style array literal, used
// for the copy-to-clipboard value of the Hex column.
func hexArrayRepr(data []byte) string {
	parts := make([]string, len(data))
	for i, c := range data {
		parts[i] = fmt.Sprintf("0x%02X", c)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// weightedRowLayout arranges exactly two objects (ASCII then Hex columns)
// side by side, given asciiColumnWeight/hexColumnWeight of the width.
type weightedRowLayout struct{}

const weightedRowGap = 12

func (weightedRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		if m := o.MinSize().Height; m > h {
			h = m
		}
	}
	return fyne.NewSize(0, h)
}

func (weightedRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	avail := size.Width - weightedRowGap
	if avail < 0 {
		avail = 0
	}
	asciiWidth := avail * asciiColumnWeight
	hexWidth := avail - asciiWidth

	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(asciiWidth, size.Height))
	objects[1].Move(fyne.NewPos(asciiWidth+weightedRowGap, 0))
	objects[1].Resize(fyne.NewSize(hexWidth, size.Height))
}

// rowTheme lets a wrapped Label show an arbitrary literal color and font
// size - rather than being limited to Fyne's fixed theme Importance/
// SizeName enums - by resolving ColorNameForeground/SizeNameText through
// mutable pointers that SetText/SetTextSize can update in place as list
// rows are recycled between entries.
type rowTheme struct {
	fyne.Theme
	col  *color.Color
	size *float32
}

func (t *rowTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameForeground {
		return *t.col
	}
	return t.Theme.Color(name, variant)
}

func (t *rowTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return *t.size
	}
	return t.Theme.Size(name)
}

// tappableLabel is a wrapping label that invokes onTapped when clicked -
// used for the click-to-copy ASCII/Hex monitor columns (which wrap within
// their column instead of overflowing it) and the click-to-send preset
// names. It renders with an explicit color and font size (via rowTheme,
// above) so the monitor can show user-chosen RX/TX colors and row font
// sizes that aren't tied to the app's theme.
type tappableLabel struct {
	widget.BaseWidget
	label    *widget.Label
	wrapped  fyne.CanvasObject
	col      *color.Color
	fontSize *float32
	onTapped func()
}

func newTappableLabel(onTapped func()) *tappableLabel {
	col := new(color.Color)
	*col = theme.Color(theme.ColorNameForeground)
	size := new(float32)
	*size = theme.TextSize()

	label := widget.NewLabel("")
	// Off, not TextWrapBreak: Fyne's RichText layout always honors embedded
	// newlines as line breaks regardless of Wrapping mode (it splits on
	// "\n" before ever considering width), so this still respects any
	// caller's own explicit line breaks (e.g. asciiRepr's) - it just
	// disables the *additional* automatic width-based wrapping within
	// each of those lines.
	label.Wrapping = fyne.TextWrapOff
	label.TextStyle = fyne.TextStyle{Monospace: true}

	t := &tappableLabel{label: label, col: col, fontSize: size, onTapped: onTapped}
	t.wrapped = container.NewThemeOverride(label, &rowTheme{Theme: Theme(), col: col, size: size})
	t.ExtendBaseWidget(t)
	return t
}

// SetText updates the displayed text, color and tap handler in one go, for
// reuse by widget.List's item recycling.
func (t *tappableLabel) SetText(text string, col color.Color, onTapped func()) {
	t.onTapped = onTapped
	*t.col = col
	t.label.SetText(text)
}

// SetTextSize updates the font size used to render the text.
func (t *tappableLabel) SetTextSize(size float32) {
	if size <= 0 || *t.fontSize == size {
		return
	}
	*t.fontSize = size
	t.label.Refresh()
}

// MinHeight returns the height needed to show the current text, wrapped at
// whatever width this label was last resized to - used to grow the row
// (via widget.List.SetItemHeight) to fit wrapped content.
func (t *tappableLabel) MinHeight() float32 {
	return t.label.MinSize().Height
}

func (t *tappableLabel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.wrapped)
}

func (t *tappableLabel) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tappableLabel) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}
