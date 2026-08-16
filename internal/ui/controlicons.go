package ui

import (
	"bytes"
	"embed"
	"encoding/xml"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

//go:embed controlicons/*.svg
var controlIconFS embed.FS

// controlIconFile maps a C0 control byte to its icon's filename stem under
// controlicons/. Includes ht/cr/lf, which controlCharNames (in monitor.go)
// leaves blank since asciiRepr renders those via \t/\r/\n shorthand instead
// of a <XXX> token - the icon view renders every C0 byte as an icon.
// 0x00 uses "nul_" rather than "nul": Go's embed package rejects "nul" as
// a filename (a reserved Windows device name) regardless of host OS.
var controlIconFile = [0x20]string{
	0x00: "nul_", 0x01: "soh", 0x02: "stx", 0x03: "etx",
	0x04: "eot", 0x05: "enq", 0x06: "ack", 0x07: "bel",
	0x08: "bs", 0x09: "ht", 0x0A: "lf", 0x0B: "vt",
	0x0C: "ff", 0x0D: "cr", 0x0E: "so", 0x0F: "si",
	0x10: "dle", 0x11: "dc1", 0x12: "dc2", 0x13: "dc3",
	0x14: "dc4", 0x15: "nak", 0x16: "syn", 0x17: "etb",
	0x18: "can", 0x19: "em", 0x1A: "sub", 0x1B: "esc",
	0x1C: "fs", 0x1D: "gs", 0x1E: "rs", 0x1F: "us",
}

var (
	// controlIconTemplates holds each icon's raw SVG source (fill/stroke
	//="currentColor"), used to stamp out a recolored resource per
	// controlIcon call.
	controlIconTemplates [0x20][]byte
	// controlIconAspects holds each icon's width/height ratio (from its SVG
	// dimensions), so a cell can size the icon to its natural proportions
	// instead of forcing every control-char icon into a uniform square.
	controlIconAspects [0x20]float32
	// maxControlIconAspect is the widest aspect ratio among all 32 icons,
	// used to size a uniform cell width that fits every icon.
	maxControlIconAspect float32
)

func init() {
	for b, name := range controlIconFile {
		data, err := controlIconFS.ReadFile("controlicons/" + name + ".svg")
		if err != nil {
			panic(err) // programmer error: controlIconFile/embedded file mismatch
		}
		controlIconTemplates[b] = data
		aspect := svgAspectRatio(data)
		controlIconAspects[b] = aspect
		if aspect > maxControlIconAspect {
			maxControlIconAspect = aspect
		}
	}
}

// controlIconCache holds one recolored resource per (byte, color) pair
// already requested. Fyne rasterizes and caches an SVG resource's pixels
// per resource pointer, so reusing the same *fyne.StaticResource for a
// given (byte, color) - rather than building a new one on every SetData
// call - avoids re-rasterizing icons on every scroll/redraw. In practice
// there are only ever as many distinct colors as the user has RX/TX set
// to, so this cache stays tiny. Only ever accessed from the Fyne main
// goroutine (via fyne.Do-wrapped serial callbacks), so no locking needed.
var controlIconCache = map[[2]any]fyne.Resource{}

// controlIcon returns b's icon resource recolored to col - the row's own
// RX/TX color, matching the plain-text cells next to it, rather than a
// fixed theme color.
func controlIcon(b byte, col color.Color) fyne.Resource {
	hex := colorToHex(col)
	key := [2]any{b, hex}
	if res, ok := controlIconCache[key]; ok {
		return res
	}
	svg := bytes.ReplaceAll(controlIconTemplates[b], []byte("currentColor"), []byte(hex))
	res := fyne.NewStaticResource(controlIconFile[b]+"-"+hex+".svg", svg)
	controlIconCache[key] = res
	return res
}

// svgAspectRatio parses an SVG's width/height (in preference to viewBox,
// which for these icons matches the same units) to get its width/height
// ratio, used to size an icon's cell proportionally. Falls back to 1 (a
// square cell) if parsing fails - a decoding bug in an embedded, tested
// asset would be immediately obvious visually, so silently degrading
// rather than panicking is fine here.
func svgAspectRatio(svg []byte) float32 {
	var root struct {
		Width  string `xml:"width,attr"`
		Height string `xml:"height,attr"`
	}
	if err := xml.Unmarshal(svg, &root); err != nil {
		return 1
	}
	w, wErr := strconv.ParseFloat(strings.TrimSpace(root.Width), 32)
	h, hErr := strconv.ParseFloat(strings.TrimSpace(root.Height), 32)
	if wErr != nil || hErr != nil || h <= 0 {
		return 1
	}
	return float32(w / h)
}
