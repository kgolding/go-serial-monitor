package ui

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// countPcapngPackets counts Enhanced Packet Blocks (type 0x00000006) in a
// pcapng byte stream, verifying each block's header/trailer lengths agree
// along the way. This intentionally re-derives only the minimal framing
// needed for a UI-level smoke test; internal/pcapng's own test suite
// covers the format in full detail.
func countPcapngPackets(t *testing.T, data []byte) int {
	t.Helper()
	const blockTypeEPB = 0x00000006

	count := 0
	off := 0
	for off < len(data) {
		if off+8 > len(data) {
			t.Fatalf("truncated block header at offset %d", off)
		}
		blockType := binary.LittleEndian.Uint32(data[off : off+4])
		totalLen := binary.LittleEndian.Uint32(data[off+4 : off+8])
		if off+int(totalLen) > len(data) {
			t.Fatalf("block at offset %d claims length %d, exceeds buffer (%d)", off, totalLen, len(data))
		}
		trailerLen := binary.LittleEndian.Uint32(data[off+int(totalLen)-4 : off+int(totalLen)])
		if trailerLen != totalLen {
			t.Fatalf("block at offset %d: header length %d != trailer length %d", off, totalLen, trailerLen)
		}
		if blockType == blockTypeEPB {
			count++
		}
		off += int(totalLen)
	}
	return count
}

func TestMonitorViewExportPCAPWritesOnePacketPerEntry(t *testing.T) {
	m := newTestMonitorView(t)
	base := time.Now()

	m.Append(false, []byte("hello"), base, 0)
	m.Append(true, []byte("hi"), base.Add(time.Millisecond), 0)
	m.Append(false, []byte("world"), base.Add(2*time.Millisecond), 0)

	var buf bytes.Buffer
	if err := m.ExportPCAP(&buf); err != nil {
		t.Fatalf("ExportPCAP: %v", err)
	}

	if got, want := countPcapngPackets(t, buf.Bytes()), len(m.entries); got != want {
		t.Fatalf("got %d packets, want %d (one per entry)", got, want)
	}
}

func TestMonitorViewExportPCAPEmptyLogWritesNoPackets(t *testing.T) {
	m := newTestMonitorView(t)

	var buf bytes.Buffer
	if err := m.ExportPCAP(&buf); err != nil {
		t.Fatalf("ExportPCAP: %v", err)
	}

	if got := countPcapngPackets(t, buf.Bytes()); got != 0 {
		t.Fatalf("got %d packets, want 0 for an empty log", got)
	}
}

func TestMonitorViewExportEnabledTracksEntryCount(t *testing.T) {
	m := newTestMonitorView(t)

	if !m.exportBtn.Disabled() {
		t.Fatalf("export button should start disabled with no entries")
	}

	m.Append(false, []byte("x"), time.Now(), 0)
	if m.exportBtn.Disabled() {
		t.Fatalf("export button should be enabled once there's an entry")
	}

	m.Clear()
	if !m.exportBtn.Disabled() {
		t.Fatalf("export button should be disabled again after Clear")
	}
}
