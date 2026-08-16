//go:build !android && !ios

package serialio

import (
	"sync/atomic"
	"time"

	"go.bug.st/serial"
)

// ListPorts returns the names of the serial ports currently available on
// this machine.
func ListPorts() ([]string, error) {
	return serial.GetPortsList()
}

// session is the desktop Port implementation backed by go.bug.st/serial.
type session struct {
	port    serial.Port
	closing atomic.Bool
}

// Open opens the named serial port with the given configuration and starts a
// background goroutine that reads incoming data and reports it (along with
// connection state changes) through onEvent. onEvent may be called from a
// goroutine other than the caller's.
func Open(cfg Config, onEvent func(Event)) (Port, error) {
	mode := &serial.Mode{
		BaudRate: cfg.BaudRate,
		DataBits: cfg.DataBits,
		Parity:   serial.Parity(cfg.Parity),
		StopBits: serial.StopBits(cfg.StopBits),
	}

	p, err := serial.Open(cfg.Port, mode)
	if err != nil {
		return nil, err
	}
	_ = p.SetReadTimeout(200 * time.Millisecond)

	s := &session{port: p}
	go s.readLoop(onEvent)
	return s, nil
}

func (s *session) readLoop(onEvent func(Event)) {
	buf := make([]byte, 4096)
	for {
		n, err := s.port.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			onEvent(Event{Kind: EventRX, Data: data, Time: time.Now()})
		}
		if err != nil {
			if s.closing.Load() {
				onEvent(Event{Kind: EventClosed, Time: time.Now()})
			} else {
				onEvent(Event{Kind: EventError, Err: err, Time: time.Now()})
			}
			return
		}
		// A zero-byte, nil-error read means the read timeout elapsed with
		// no data available; just loop and try again so Close() is noticed
		// promptly.
	}
}

func (s *session) Write(data []byte) (int, error) {
	return s.port.Write(data)
}

func (s *session) Close() error {
	s.closing.Store(true)
	return s.port.Close()
}
