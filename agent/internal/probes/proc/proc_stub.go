//go:build !linux

// Non-Linux stub. eBPF only runs on Linux; we keep the package buildable on
// macOS/Windows so editing and `go build` don't break, but Load() refuses.
package proc

import (
	"errors"
	"time"
)

type ProcEvent struct {
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	UID       uint32    `json:"uid"`
	Comm      string    `json:"comm"`
	Filename  string    `json:"filename"`
	Args      []string  `json:"args"`
	Timestamp time.Time `json:"timestamp"`
}

type ProcProbe struct {
	closed chan ProcEvent
}

func (p *ProcProbe) Load() error {
	return errors.New("citadel process probe requires linux (eBPF); rebuild on the runner")
}

func (p *ProcProbe) Events() <-chan ProcEvent {
	if p.closed == nil {
		p.closed = make(chan ProcEvent)
		close(p.closed)
	}
	return p.closed
}

func (p *ProcProbe) Close() error { return nil }
