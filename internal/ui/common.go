package ui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kgolding/go-serial-monitor/internal/store"
)

// layoutSpacer returns an expanding, invisible spacer for HBox toolbars.
func layoutSpacer() fyne.CanvasObject {
	return layout.NewSpacer()
}

// buildSendBytes turns a preset/send-box's text into the raw bytes to write
// to the serial port, appending the requested line ending.
func buildSendBytes(format store.Format, text string, le store.LineEnding) ([]byte, error) {
	var data []byte
	switch format {
	case store.FormatHex:
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, text)
		if len(cleaned)%2 != 0 {
			return nil, errors.New("hex data must have an even number of hex digits")
		}
		b, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("invalid hex data: %w", err)
		}
		data = b
	default:
		data = []byte(text)
	}
	return append(data, le.Bytes()...), nil
}

// setDataPlaceholder updates entry's placeholder text to match the format
// currently selected in formatRadio ("ASCII" or "Hex").
func setDataPlaceholder(entry *widget.Entry, formatRadio *widget.RadioGroup) {
	if formatFromLabel(formatRadio.Selected) == store.FormatHex {
		entry.SetPlaceHolder("Type hex bytes, e.g. 48 65 6C 6C 6F")
	} else {
		entry.SetPlaceHolder("Type ASCII text")
	}
}

func formatLabel(f store.Format) string {
	if f == store.FormatHex {
		return "Hex"
	}
	return "ASCII"
}

func formatFromLabel(s string) store.Format {
	if s == "Hex" {
		return store.FormatHex
	}
	return store.FormatASCII
}

func lineEndingLabels() []string {
	labels := make([]string, len(store.LineEndingOptions))
	for i, o := range store.LineEndingOptions {
		labels[i] = o.Label
	}
	return labels
}

func lineEndingLabel(le store.LineEnding) string {
	for _, o := range store.LineEndingOptions {
		if o.Value == le {
			return o.Label
		}
	}
	return store.LineEndingOptions[0].Label
}

func lineEndingFromLabel(label string) store.LineEnding {
	for _, o := range store.LineEndingOptions {
		if o.Label == label {
			return o.Value
		}
	}
	return store.LineEndingNone
}
