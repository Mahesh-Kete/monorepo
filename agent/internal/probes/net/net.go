//go:build linux

// Package net wraps the network egress eBPF probe (kprobe on tcp_v4_connect).
//
// The C source lives in /agent/bpf/net.bpf.c and is compiled + embedded into
// this package by bpf2go at `go generate` time.
package net

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	gonet "net"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

// bpf2go generates loadNetProbe / loadNetProbeObjects + the *Objects type from
// net.bpf.c. With -target amd64,arm64 it emits separate .o files per arch
// (netprobe_x86_bpfel.* and netprobe_arm64_bpfel.*) and the Go side picks the
// right one at compile time via build constraints. Path is 3 levels up:
// net/ -> probes/ -> internal/ -> agent/, then bpf/.
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -target arm64 netProbe ../../../bpf/net.bpf.c -- -I../../../bpf

// NetEvent is the userspace representation of one outbound TCP connect.
// Hostname is populated by main.go via the reverse-DNS cache; ProcessChain
// is populated from the in-memory proctree (proc probe). Both are best-effort
// enrichments and may be empty.
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

// rawNetEvent mirrors `struct net_event` in net.bpf.c byte-for-byte (48 bytes).
// Field order, sizes, and the explicit Pad slot must match the C side exactly.
type rawNetEvent struct {
	PID   uint32
	PPID  uint32
	UID   uint32
	Comm  [16]byte
	Saddr uint32 // network byte order; left as 0 by the BPF program
	Daddr uint32 // network byte order
	Dport uint16 // network byte order
	Pad   uint16
	TsNs  uint64
}

// NetProbe owns the loaded eBPF objects, the kprobe attachment, and the
// ringbuffer reader goroutine.
type NetProbe struct {
	objs   netProbeObjects
	kp     link.Link
	rd     *ringbuf.Reader
	events chan NetEvent
}

// Load compiles-in / loads the BPF objects, attaches the kprobe to
// tcp_v4_connect, and starts a reader goroutine that publishes events
// on the channel returned by Events().
func (n *NetProbe) Load() error {
	// On kernels < 5.11 BPF maps were charged against the RLIMIT_MEMLOCK
	// ulimit. RemoveMemlock raises it to infinity; on newer kernels it's
	// a no-op.
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("rlimit memlock: %w", err)
	}

	if err := loadNetProbeObjects(&n.objs, nil); err != nil {
		return fmt.Errorf("load netprobe objects: %w", err)
	}

	kp, err := link.Kprobe("tcp_v4_connect", n.objs.HandleTcpConnect, nil)
	if err != nil {
		_ = n.objs.Close()
		return fmt.Errorf("attach kprobe tcp_v4_connect: %w", err)
	}
	n.kp = kp

	rd, err := ringbuf.NewReader(n.objs.NetEvents)
	if err != nil {
		_ = n.kp.Close()
		_ = n.objs.Close()
		return fmt.Errorf("ringbuf reader: %w", err)
	}
	n.rd = rd

	n.events = make(chan NetEvent, 256)
	go n.readLoop()
	return nil
}

// Events returns the channel of parsed NetEvent values. The channel is closed
// when the underlying ringbuffer is closed (i.e., after Close()).
func (n *NetProbe) Events() <-chan NetEvent { return n.events }

// Close detaches the kprobe, closes the ringbuf reader, and frees the BPF
// objects. The reader goroutine exits on ringbuf.ErrClosed and closes the
// Events() channel on its way out.
func (n *NetProbe) Close() error {
	var firstErr error
	if n.rd != nil {
		if err := n.rd.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if n.kp != nil {
		if err := n.kp.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := n.objs.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (n *NetProbe) readLoop() {
	defer close(n.events)
	for {
		rec, err := n.rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			// Transient error (e.g., ringbuf overflow). Skip and continue.
			continue
		}
		ev, ok := parseRecord(rec.RawSample)
		if !ok {
			continue
		}
		n.events <- ev
	}
}

func parseRecord(b []byte) (NetEvent, bool) {
	var raw rawNetEvent
	if len(b) < binary.Size(raw) {
		return NetEvent{}, false
	}
	// BPF programs write in host byte order (which on x86_64 is little-endian);
	// network fields (daddr, dport) stay in network byte order — we convert
	// those below.
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &raw); err != nil {
		return NetEvent{}, false
	}

	return NetEvent{
		PID:     raw.PID,
		PPID:    raw.PPID,
		UID:     raw.UID,
		Comm:    commString(raw.Comm),
		DstIP:   ipv4FromBE32(raw.Daddr),
		DstPort: ntohs(raw.Dport),
		// ts_ns is kernel monotonic time since boot; for the hackathon we use
		// wall-clock at the moment of userspace receipt. Lag is sub-millisecond.
		Timestamp: time.Now(),
	}, true
}

func commString(b [16]byte) string {
	if i := bytes.IndexByte(b[:], 0); i >= 0 {
		return string(b[:i])
	}
	return string(b[:])
}

// ipv4FromBE32 converts a __be32 (read on a little-endian host as uint32 with
// network-order bytes preserved in memory) into a net.IP.
func ipv4FromBE32(be uint32) gonet.IP {
	return gonet.IPv4(byte(be), byte(be>>8), byte(be>>16), byte(be>>24)).To4()
}

// ntohs swaps a network-order uint16 (port) into host order.
func ntohs(p uint16) uint16 { return (p << 8) | (p >> 8) }
