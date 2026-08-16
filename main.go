// Command go-serial-app is a cross-platform serial port monitor and sender
// for engineers: it connects to a serial port, logs RX/TX traffic in ASCII
// and/or hex with timestamps, and lets the user send ad-hoc or saved
// ASCII/hex snippets.
package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/kgolding/go-serial-monitor/internal/ui"
)

func main() {
	fyneApp := app.NewWithID("uk.co.kgolding.go-serial-app")
	fyneApp.Settings().SetTheme(ui.Theme())
	win := fyneApp.NewWindow("Serial Monitor")

	a, err := ui.NewApp(fyneApp, win)
	if err != nil {
		log.Fatalf("failed to start: %v", err)
	}

	win.SetContent(a.Content())
	win.Resize(fyne.NewSize(1000, 700))
	win.ShowAndRun()
}
