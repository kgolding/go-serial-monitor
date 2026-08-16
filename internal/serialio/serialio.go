// Package serialio provides a small, platform-independent wrapper around a
// serial port connection, used by the UI layer so it never has to import
// go.bug.st/serial (and its platform build constraints) directly.
package serialio

import "time"

// Parity mirrors go.bug.st/serial's Parity setting without depending on it.
type Parity int

const (
	ParityNone Parity = iota
	ParityOdd
	ParityEven
	ParityMark
	ParitySpace
)

func (p Parity) String() string {
	switch p {
	case ParityOdd:
		return "Odd"
	case ParityEven:
		return "Even"
	case ParityMark:
		return "Mark"
	case ParitySpace:
		return "Space"
	default:
		return "None"
	}
}

// StopBits mirrors go.bug.st/serial's StopBits setting without depending on it.
type StopBits int

const (
	StopBits1 StopBits = iota
	StopBits1Half
	StopBits2
)

func (s StopBits) String() string {
	switch s {
	case StopBits1Half:
		return "1.5"
	case StopBits2:
		return "2"
	default:
		return "1"
	}
}

// Config describes how to open a serial port.
type Config struct {
	Port     string
	BaudRate int
	DataBits int
	Parity   Parity
	StopBits StopBits
}

// EventKind identifies what kind of Event was raised by a Port.
type EventKind int

const (
	// EventRX is raised when data is received from the port.
	EventRX EventKind = iota
	// EventError is raised when the port hits a read error and has stopped.
	EventError
	// EventClosed is raised when the port was closed (deliberately or not).
	EventClosed
)

// Event is delivered to the callback passed to Open for RX data and
// connection state changes. It is always safe to read from any goroutine,
// but callers updating a UI must hop back onto the UI thread themselves
// (e.g. with fyne.Do).
type Event struct {
	Kind EventKind
	Data []byte
	Err  error
	Time time.Time
}

// Port is an open serial connection.
type Port interface {
	// Write sends data to the serial port.
	Write(data []byte) (int, error)
	// Close closes the port. Safe to call more than once.
	Close() error
}

// CommonBaudRates lists baud rates typically offered by serial hardware.
var CommonBaudRates = []int{300, 1200, 2400, 4800, 9600, 14400, 19200, 28800, 38400, 57600, 115200, 230400, 460800, 921600}

// FrameDuration returns how long it takes to transmit one byte at cfg's
// baud rate, i.e. the wire time of one start bit + data bits + parity bit
// (if any) + stop bits.
func FrameDuration(cfg Config) time.Duration {
	if cfg.BaudRate <= 0 {
		return 0
	}

	bits := 1.0 + float64(cfg.DataBits) // start bit + data bits
	if cfg.Parity != ParityNone {
		bits++
	}
	switch cfg.StopBits {
	case StopBits1Half:
		bits += 1.5
	case StopBits2:
		bits += 2
	default:
		bits++
	}

	return time.Duration(bits / float64(cfg.BaudRate) * float64(time.Second))
}
