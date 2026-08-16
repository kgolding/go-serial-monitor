// Package pcapng writes serial monitor log entries as a pcapng capture
// file (https://www.ietf.org/archive/id/draft-ietf-opsawg-pcapng) that
// Wireshark can open directly, tagging each packet's RX/TX direction via
// the Enhanced Packet Block's epb_flags option.
package pcapng

import (
	"encoding/binary"
	"io"
	"time"
)

// LinkTypeUser0 is the pcapng/libpcap link-layer type written into the
// Interface Description Block. It is one of DLT_USER0-DLT_USER15
// (147-162), reserved for private use: Wireshark opens it as raw,
// undissected "Data", which is exactly right for arbitrary serial
// payloads that have no link-layer framing of their own.
const LinkTypeUser0 = 147

// Direction is an Enhanced Packet Block's epb_flags inbound/outbound
// value (bits 0-1: 0 = not available, 1 = inbound, 2 = outbound). Its
// numeric value is the epb_flags option payload itself, written directly
// as a little-endian uint32.
type Direction uint32

const (
	DirectionRX Direction = 1 // inbound
	DirectionTX Direction = 2 // outbound
)

// Block types.
const (
	blockTypeSHB = 0x0A0D0D0A
	blockTypeIDB = 0x00000001
	blockTypeEPB = 0x00000006
)

const byteOrderMagic = 0x1A2B3C4D

// Option codes. These are scoped per block type: code 2 means "if_name" in
// an Interface Description Block but "epb_flags" in an Enhanced Packet
// Block - that's normal pcapng, not a collision to resolve.
const (
	optEndOfOpt    = 0
	optIfName      = 2
	optEpbFlags    = 2
	optShbUserAppl = 4
)

// Writer incrementally writes a pcapng file: NewWriter emits the Section
// Header Block and one Interface Description Block; each WritePacket call
// emits one Enhanced Packet Block. There is no Close/Finalize step - the
// SHB's section_length is written as -1 ("unknown"), so the stream is
// valid pcapng after every block. The caller is responsible for closing w.
type Writer struct {
	w io.Writer
}

// NewWriter writes the Section Header Block and a single Interface
// Description Block (linktype LinkTypeUser0, snaplen 0/unlimited) to w.
// ifaceName is written as the IDB's if_name option (e.g. the serial port
// path, "/dev/ttyUSB0"); pass "" to omit it.
func NewWriter(w io.Writer, ifaceName string) (*Writer, error) {
	pw := &Writer{w: w}

	var shb blockBuf
	shb.u32(byteOrderMagic)
	shb.u16(1)                  // major version
	shb.u16(0)                  // minor version
	shb.u64(0xFFFFFFFFFFFFFFFF) // section_length: unknown
	shb.option(optShbUserAppl, []byte("go-serial-app"))
	shb.endOfOpt()
	if err := shb.writeBlock(w, blockTypeSHB); err != nil {
		return nil, err
	}

	var idb blockBuf
	idb.u16(LinkTypeUser0)
	idb.u16(0) // reserved
	idb.u32(0) // snaplen: unlimited
	if ifaceName != "" {
		idb.option(optIfName, []byte(ifaceName))
	}
	idb.endOfOpt()
	if err := idb.writeBlock(w, blockTypeIDB); err != nil {
		return nil, err
	}

	return pw, nil
}

// WritePacket appends one Enhanced Packet Block for a single log row. t is
// stored at microsecond resolution (pcapng's implicit default when
// if_tsresol is not set). dir is written as the epb_flags option.
func (pw *Writer) WritePacket(t time.Time, dir Direction, data []byte) error {
	us := uint64(t.UnixMicro())

	var epb blockBuf
	epb.u32(0) // interface_id
	epb.u32(uint32(us >> 32))
	epb.u32(uint32(us))
	epb.u32(uint32(len(data))) // captured_len
	epb.u32(uint32(len(data))) // original_len (snaplen is unlimited, never truncated)
	epb.bytesPad4(data)

	var flags [4]byte
	binary.LittleEndian.PutUint32(flags[:], uint32(dir))
	epb.option(optEpbFlags, flags[:])
	epb.endOfOpt()

	return epb.writeBlock(pw.w, blockTypeEPB)
}

// blockBuf accumulates one block's body (and options) in little-endian
// byte order. Every append is padded to a 4-byte boundary, so the body's
// total length is always a multiple of 4.
type blockBuf struct {
	b []byte
}

func (bb *blockBuf) u16(v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	bb.b = append(bb.b, buf[:]...)
}

func (bb *blockBuf) u32(v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	bb.b = append(bb.b, buf[:]...)
}

func (bb *blockBuf) u64(v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	bb.b = append(bb.b, buf[:]...)
}

// bytesPad4 appends data followed by zero-padding out to a multiple of 4
// bytes.
func (bb *blockBuf) bytesPad4(data []byte) {
	bb.b = append(bb.b, data...)
	if pad := (4 - len(data)%4) % 4; pad > 0 {
		bb.b = append(bb.b, make([]byte, pad)...)
	}
}

// option appends a TLV option: code(2) + length(2) + value padded to a
// multiple of 4 bytes. length is the true (unpadded) value length.
func (bb *blockBuf) option(code uint16, value []byte) {
	bb.u16(code)
	bb.u16(uint16(len(value)))
	bb.bytesPad4(value)
}

// endOfOpt appends the opt_endofopt terminator.
func (bb *blockBuf) endOfOpt() {
	bb.u16(optEndOfOpt)
	bb.u16(0)
}

// writeBlock wraps the accumulated body with the pcapng block framing
// (block_type, block_total_length, body, block_total_length) and writes it
// to w in one call.
func (bb *blockBuf) writeBlock(w io.Writer, blockType uint32) error {
	totalLen := uint32(12 + len(bb.b))

	out := make([]byte, 0, totalLen)
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], blockType)
	binary.LittleEndian.PutUint32(hdr[4:8], totalLen)
	out = append(out, hdr[:]...)
	out = append(out, bb.b...)
	var trailer [4]byte
	binary.LittleEndian.PutUint32(trailer[:], totalLen)
	out = append(out, trailer[:]...)

	_, err := w.Write(out)
	return err
}
