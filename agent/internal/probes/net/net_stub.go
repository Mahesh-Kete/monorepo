//go:build !linux

// Non-Linux stub for the network probe. eBPF only runs on Linux, but we keep
// the package buildable on macOS/Windows so editors and dev workflows aren't
// broken. Calling Load() on a non-Linux build returns an error.
package net

import (
	"errors"
	gonet "net"
	"time"
)

type NetEvent struct {
	PID          uint32    `json:"pid"`
	PPID         uint32    `json:"ppid"`
	UID          uint32    `json:"uid"`
	Comm         string    `json:"comm"`
	DstIP        gonet.IP  `json:"dst_ip"`
	DstPort      uint16    `json:"dst_port"`
	Hostname     string    `json:"hostname,omitempty"`
	ProcessChain []string  `json:"process_chain,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

type NetProbe struct {
	closed chan NetEvent
}

func (n *NetProbe) Load() error {
	return errors.New("citadel network probe requires linux (eBPF); rebuild on the runner")
}

func (n *NetProbe) Events() <-chan NetEvent {
	if n.closed == nil {
		n.closed = make(chan NetEvent)
		close(n.closed)
	}
	return n.closed
}

func (n *NetProbe) Close() error { return nil }
