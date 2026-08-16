package serialio

import (
	"testing"
	"time"
)

func TestFrameDuration8N1(t *testing.T) {
	// 9600 8N1: 1 start + 8 data + 1 stop = 10 bits/byte.
	got := FrameDuration(Config{BaudRate: 9600, DataBits: 8, Parity: ParityNone, StopBits: StopBits1})
	bits, baud := 10.0, 9600.0
	want := time.Duration(bits / baud * float64(time.Second))
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFrameDurationWithParityAnd2StopBits(t *testing.T) {
	// 1200 7E2: 1 start + 7 data + 1 parity + 2 stop = 11 bits/byte.
	got := FrameDuration(Config{BaudRate: 1200, DataBits: 7, Parity: ParityEven, StopBits: StopBits2})
	bits, baud := 11.0, 1200.0
	want := time.Duration(bits / baud * float64(time.Second))
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFrameDurationZeroBaud(t *testing.T) {
	if got := FrameDuration(Config{BaudRate: 0}); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}
