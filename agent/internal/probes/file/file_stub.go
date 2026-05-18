//go:build !linux

// Non-Linux stub. Load() refuses; package builds on Mac/Windows for editor
// and `go build` compatibility.
package file

import (
	"errors"
	"time"
)

type FileEvent struct {
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	UID       uint32    `json:"uid"`
	Comm      string    `json:"comm"`
	Filename  string    `json:"filename"`
	Flags     int32     `json:"flags_raw"`
	FlagsStr  string    `json:"flags"`
	Timestamp time.Time `json:"timestamp"`
}

type FileProbe struct {
	closed chan FileEvent
}

func (f *FileProbe) Load() error {
	return errors.New("citadel file probe requires linux (eBPF); rebuild on the runner")
}

func (f *FileProbe) Events() <-chan FileEvent {
	if f.closed == nil {
		f.closed = make(chan FileEvent)
		close(f.closed)
	}
	return f.closed
}

func (f *FileProbe) Close() error { return nil }
