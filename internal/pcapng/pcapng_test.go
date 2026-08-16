package pcapng

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// --- test-only pcapng block/option parsing helpers ---

type parsedBlock struct {
	Type uint32
	Body []byte
}

// walkBlocks parses data into its constituent blocks, verifying every
// block's header length matches its trailer length and that the blocks
// exactly fill the buffer with no gap or trailing garbage.
func walkBlocks(t *testing.T, data []byte) []parsedBlock {
	t.Helper()
	var blocks []parsedBlock
	off := 0
	for off < len(data) {
		if off+8 > len(data) {
			t.Fatalf("truncated block header at offset %d", off)
		}
		blockType := binary.LittleEndian.Uint32(data[off : off+4])
		totalLen := binary.LittleEndian.Uint32(data[off+4 : off+8])
		if totalLen%4 != 0 {
			t.Fatalf("block at offset %d: total length %d is not a multiple of 4", off, totalLen)
		}
		if off+int(totalLen) > len(data) {
			t.Fatalf("block at offset %d claims length %d, exceeds buffer (%d)", off, totalLen, len(data))
		}
		trailerLen := binary.LittleEndian.Uint32(data[off+int(totalLen)-4 : off+int(totalLen)])
		if trailerLen != totalLen {
			t.Fatalf("block at offset %d: header length %d != trailer length %d", off, totalLen, trailerLen)
		}
		body := data[off+8 : off+int(totalLen)-4]
		blocks = append(blocks, parsedBlock{Type: blockType, Body: body})
		off += int(totalLen)
	}
	if off != len(data) {
		t.Fatalf("blocks consumed %d bytes, buffer is %d", off, len(data))
	}
	return blocks
}

type option struct {
	Code  uint16
	Value []byte
}

// parseOptions parses a TLV options list starting at body[start:], up to
// and including the opt_endofopt terminator.
func parseOptions(t *testing.T, body []byte, start int) []option {
	t.Helper()
	var opts []option
	off := start
	for {
		if off+4 > len(body) {
			t.Fatalf("truncated option at offset %d", off)
		}
		code := binary.LittleEndian.Uint16(body[off : off+2])
		length := binary.LittleEndian.Uint16(body[off+2 : off+4])
		off += 4
		if code == optEndOfOpt {
			return opts
		}
		val := body[off : off+int(length)]
		opts = append(opts, option{Code: code, Value: val})
		pad := (4 - int(length)%4) % 4
		off += int(length) + pad
	}
}

func findOption(opts []option, code uint16) (option, bool) {
	for _, o := range opts {
		if o.Code == code {
			return o, true
		}
	}
	return option{}, false
}

// --- tests ---

func TestNewWriterEmitsSHBThenIDB(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewWriter(&buf, ""); err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (SHB, IDB)", len(blocks))
	}

	shb := blocks[0]
	if shb.Type != blockTypeSHB {
		t.Fatalf("block 0 type = 0x%08X, want SHB 0x%08X", shb.Type, blockTypeSHB)
	}
	if got := binary.LittleEndian.Uint32(shb.Body[0:4]); got != byteOrderMagic {
		t.Fatalf("SHB byte-order magic = 0x%08X, want 0x%08X", got, byteOrderMagic)
	}

	idb := blocks[1]
	if idb.Type != blockTypeIDB {
		t.Fatalf("block 1 type = 0x%08X, want IDB 0x%08X", idb.Type, blockTypeIDB)
	}
	linktype := binary.LittleEndian.Uint16(idb.Body[0:2])
	snaplen := binary.LittleEndian.Uint32(idb.Body[4:8])
	if linktype != LinkTypeUser0 {
		t.Fatalf("IDB linktype = %d, want %d", linktype, LinkTypeUser0)
	}
	if snaplen != 0 {
		t.Fatalf("IDB snaplen = %d, want 0 (unlimited)", snaplen)
	}
}

func TestNewWriterIfaceNameOption(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewWriter(&buf, "/dev/ttyUSB0"); err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	idb := blocks[1]
	opts := parseOptions(t, idb.Body, 8)
	opt, ok := findOption(opts, optIfName)
	if !ok {
		t.Fatalf("if_name option not found")
	}
	if got := string(opt.Value); got != "/dev/ttyUSB0" {
		t.Fatalf("if_name = %q, want %q", got, "/dev/ttyUSB0")
	}
}

func TestNewWriterEmptyIfaceNameOmitsOption(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewWriter(&buf, ""); err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	idb := blocks[1]
	opts := parseOptions(t, idb.Body, 8)
	if len(opts) != 0 {
		t.Fatalf("got %d options, want 0 when ifaceName is empty", len(opts))
	}
}

func epbOptionsStart(body []byte) int {
	dataLen := binary.LittleEndian.Uint32(body[12:16])
	padded := (int(dataLen) + 3) / 4 * 4
	return 20 + padded
}

func TestWritePacketRXFlag(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := pw.WritePacket(time.Now(), DirectionRX, []byte("hi")); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	opts := parseOptions(t, epb.Body, epbOptionsStart(epb.Body))
	opt, ok := findOption(opts, optEpbFlags)
	if !ok {
		t.Fatalf("epb_flags option not found")
	}
	if got := binary.LittleEndian.Uint32(opt.Value); got != uint32(DirectionRX) {
		t.Fatalf("epb_flags = %d, want %d (RX)", got, DirectionRX)
	}
}

func TestWritePacketTXFlag(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := pw.WritePacket(time.Now(), DirectionTX, []byte("hi")); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	opts := parseOptions(t, epb.Body, epbOptionsStart(epb.Body))
	opt, ok := findOption(opts, optEpbFlags)
	if !ok {
		t.Fatalf("epb_flags option not found")
	}
	if got := binary.LittleEndian.Uint32(opt.Value); got != uint32(DirectionTX) {
		t.Fatalf("epb_flags = %d, want %d (TX)", got, DirectionTX)
	}
}

func TestWritePacketOddLengthPayloadIsPadded(t *testing.T) {
	// 3 bytes of data must be padded to the next multiple of 4 (i.e. 1 pad
	// byte, for a 4-byte data region), so the following option must start
	// exactly at offset 20 (fixed EPB fields) + 4 (padded data) = 24, not
	// at 23 (unpadded) or any other offset.
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := pw.WritePacket(time.Now(), DirectionRX, []byte("abc")); err != nil { // 3 bytes
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	captured := binary.LittleEndian.Uint32(epb.Body[12:16])
	original := binary.LittleEndian.Uint32(epb.Body[16:20])
	if captured != 3 || original != 3 {
		t.Fatalf("captured_len=%d original_len=%d, want 3, 3", captured, original)
	}
	opts := parseOptions(t, epb.Body, 24)
	if _, ok := findOption(opts, optEpbFlags); !ok {
		t.Fatalf("epb_flags option not found at expected padded offset 24")
	}
}

func TestWritePacketEvenLengthPayloadNotOverpadded(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := pw.WritePacket(time.Now(), DirectionRX, []byte("abcd")); err != nil { // 4 bytes
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	// data starts at offset 20 and is exactly 4 bytes with no padding, so
	// the epb_flags option should start immediately at offset 24.
	opts := parseOptions(t, epb.Body, 24)
	if _, ok := findOption(opts, optEpbFlags); !ok {
		t.Fatalf("epb_flags option not found at expected unpadded offset")
	}
}

func TestWritePacketTimestampMicroseconds(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	want := time.Date(2026, 8, 15, 12, 34, 56, 789000000, time.UTC)
	if err := pw.WritePacket(want, DirectionRX, []byte("x")); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	tsHigh := binary.LittleEndian.Uint32(epb.Body[4:8])
	tsLow := binary.LittleEndian.Uint32(epb.Body[8:12])
	us := uint64(tsHigh)<<32 | uint64(tsLow)
	if got, want := int64(us), want.UnixMicro(); got != want {
		t.Fatalf("timestamp = %d us, want %d us", got, want)
	}
}

func TestMultiPacketFileBlockChain(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "com0")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	now := time.Now()
	if err := pw.WritePacket(now, DirectionRX, []byte("hello")); err != nil {
		t.Fatalf("WritePacket 1: %v", err)
	}
	if err := pw.WritePacket(now.Add(time.Millisecond), DirectionTX, []byte("hi")); err != nil {
		t.Fatalf("WritePacket 2: %v", err)
	}
	if err := pw.WritePacket(now.Add(2*time.Millisecond), DirectionRX, []byte("world!")); err != nil {
		t.Fatalf("WritePacket 3: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes()) // also asserts blocks exactly fill the buffer
	if len(blocks) != 5 {
		t.Fatalf("got %d blocks, want 5 (SHB, IDB, EPB, EPB, EPB)", len(blocks))
	}
	wantTypes := []uint32{blockTypeSHB, blockTypeIDB, blockTypeEPB, blockTypeEPB, blockTypeEPB}
	for i, b := range blocks {
		if b.Type != wantTypes[i] {
			t.Fatalf("block %d type = 0x%08X, want 0x%08X", i, b.Type, wantTypes[i])
		}
	}
}

func TestWritePacketEmptyData(t *testing.T) {
	var buf bytes.Buffer
	pw, err := NewWriter(&buf, "")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	if err := pw.WritePacket(time.Now(), DirectionRX, nil); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	blocks := walkBlocks(t, buf.Bytes())
	epb := blocks[2]
	captured := binary.LittleEndian.Uint32(epb.Body[12:16])
	original := binary.LittleEndian.Uint32(epb.Body[16:20])
	if captured != 0 || original != 0 {
		t.Fatalf("captured_len=%d original_len=%d, want 0, 0", captured, original)
	}
}
