package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

func newTestMonitorView(t *testing.T) *monitorView {
	t.Helper()
	app := test.NewApp()
	colors := newColorSettings(app)
	m := newMonitorView(nil, colors)
	m.CanvasObject() // initializes m.list
	return m
}

func TestAppendMergesCloseRXChunks(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()
	window := 200 * time.Millisecond

	m.Append(false, []byte("AB"), base, window)
	m.Append(false, []byte("CD"), base.Add(50*time.Millisecond), window)

	if len(m.entries) != 1 {
		t.Fatalf("got %d entries, want 1 (merged)", len(m.entries))
	}
	if got := string(m.entries[0].Data); got != "ABCD" {
		t.Fatalf("got data %q, want %q", got, "ABCD")
	}
}

func TestAppendSplitsFarApartRXChunks(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()
	window := 200 * time.Millisecond

	m.Append(false, []byte("AB"), base, window)
	m.Append(false, []byte("CD"), base.Add(500*time.Millisecond), window)

	if len(m.entries) != 2 {
		t.Fatalf("got %d entries, want 2 (not merged)", len(m.entries))
	}
}

func TestAppendNeverMergesTX(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()
	window := 200 * time.Millisecond

	m.Append(true, []byte("AB"), base, 0)
	m.Append(true, []byte("CD"), base.Add(1*time.Millisecond), window)

	if len(m.entries) != 2 {
		t.Fatalf("got %d entries, want 2 (TX never merges)", len(m.entries))
	}
}

func TestAppendDoesNotMergeAcrossTX(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()
	window := 200 * time.Millisecond

	m.Append(false, []byte("AB"), base, window)
	m.Append(true, []byte("XY"), base.Add(10*time.Millisecond), 0)
	m.Append(false, []byte("CD"), base.Add(20*time.Millisecond), window)

	if len(m.entries) != 3 {
		t.Fatalf("got %d entries, want 3 (RX shouldn't merge across a TX row)", len(m.entries))
	}
}

func TestAppendZeroWindowNeverMerges(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()

	m.Append(false, []byte("AB"), base, 0)
	m.Append(false, []byte("CD"), base.Add(1*time.Millisecond), 0)

	if len(m.entries) != 2 {
		t.Fatalf("got %d entries, want 2 (mergeWindow=0 disables merging)", len(m.entries))
	}
}

func TestAppendRespectsMaxMergedEntryBytes(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()
	window := 200 * time.Millisecond

	big := make([]byte, maxMergedEntryBytes)
	m.Append(false, big, base, window)
	m.Append(false, []byte("more"), base.Add(1*time.Millisecond), window)

	if len(m.entries) != 2 {
		t.Fatalf("got %d entries, want 2 (row at cap should not keep merging)", len(m.entries))
	}
}
