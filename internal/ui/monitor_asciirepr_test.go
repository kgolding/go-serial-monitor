package ui

import (
	"strings"
	"testing"
)

func TestAsciiReprNamesControlChars(t *testing.T) {
	cases := []struct {
		data []byte
		want string
	}{
		{[]byte{0x06}, "<ACK>"},
		{[]byte{0x00}, "<NUL>"},
		{[]byte{0x1B}, "<ESC>"},
		{[]byte{0x1F}, "<US>"},
		{[]byte("A\x06B"), "A<ACK>B"},
	}
	for _, c := range cases {
		if got := asciiRepr(c.data); got != c.want {
			t.Errorf("asciiRepr(%v) = %q, want %q", c.data, got, c.want)
		}
	}
}

func TestAsciiReprKeepsCRLFTabShorthand(t *testing.T) {
	if got, want := asciiRepr([]byte("\r\n\t")), `\r\n\t`; got != want {
		t.Errorf("asciiRepr(CRLFTab) = %q, want %q", got, want)
	}
}

func TestAsciiReprLeavesSpaceLiteral(t *testing.T) {
	if got, want := asciiRepr([]byte("Hi you")), "Hi you"; got != want {
		t.Errorf("asciiRepr(%q) = %q, want %q", "Hi you", got, want)
	}
}

func TestAsciiReprPrintableAndHighBytes(t *testing.T) {
	if got, want := asciiRepr([]byte{'A', 0x7f, 0x80, 0xff}), "A..."; got != want {
		t.Errorf("asciiRepr = %q, want %q", got, want)
	}
}

func TestAsciiReprWrapsEveryEightBytes(t *testing.T) {
	got := asciiRepr([]byte("0123456789"))
	want := "01234567\n89"
	if got != want {
		t.Errorf("asciiRepr(10 bytes) = %q, want %q", got, want)
	}
}

func TestAsciiReprExactlyEightBytesDoesNotTrailingWrap(t *testing.T) {
	got := asciiRepr([]byte("01234567"))
	if strings.Contains(got, "\n") {
		t.Errorf("asciiRepr(8 bytes) = %q, want no newline", got)
	}
}

func TestAsciiReprWrapsAtExpectedLineCount(t *testing.T) {
	// Even though a byte can expand to a multi-character ASCII token (e.g.
	// "<ACK>"), asciiRepr must wrap at the BYTE boundary, not the rendered
	// text length - see TestAsciiAndHexCellsViewsStayRowAligned (in
	// monitor_bytecells_test.go) for the corresponding on-screen guarantee
	// between the ASCII and Hex columns.
	data := []byte("Hi\x06\x07there World! more data here")
	asciiLines := strings.Count(asciiRepr(data), "\n") + 1
	wantLines := (len(data) + wrapBytesPerLine - 1) / wrapBytesPerLine
	if asciiLines != wantLines {
		t.Errorf("got %d lines for %d bytes at %d bytes/line, want %d", asciiLines, len(data), wrapBytesPerLine, wantLines)
	}
}
