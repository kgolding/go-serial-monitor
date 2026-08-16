//go:build android || ios

package serialio

import "errors"

// ErrUnsupported is returned by every operation on this platform build.
//
// go.bug.st/serial talks to serial hardware through the host OS's termios /
// Win32 COM APIs, none of which exist on Android or iOS: apps there can only
// reach a USB-serial adapter through the platform's USB host APIs (Android's
// UsbManager, or iOS's ExternalAccessory/MFi framework), which requires
// per-platform native integration this app does not implement. The UI still
// builds and runs on these platforms so presets and the rest of the app
// remain usable, but connecting to a port will fail with ErrUnsupported.
var ErrUnsupported = errors.New("serial port access is not supported on this platform build (android/ios)")

// ListPorts always returns an empty list on this platform.
func ListPorts() ([]string, error) {
	return nil, ErrUnsupported
}

// Open always fails on this platform. See ErrUnsupported.
func Open(cfg Config, onEvent func(Event)) (Port, error) {
	return nil, ErrUnsupported
}
