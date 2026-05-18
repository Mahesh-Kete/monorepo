//go:build linux

// Package proc wraps the process-exec eBPF probe (tracepoint
// sched/sched_process_exec).
//
// C source: /agent/bpf/proc.bpf.c. bpf2go compiles it and embeds the .o
// into this package at `go generate` time.
package proc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -target amd64,arm64 ProcProbe ../../../bpf/proc.bpf.c -- -I../../../bpf

// ProcEvent is the userspace representation of one process exec.
type ProcEvent struct {
	PID       uint32    `json:"pid"`
	PPID      uint32    `json:"ppid"`
	UID       uint32    `json:"uid"`
	Comm      string    `json:"comm"`
	Filename  string    `json:"filename"`
	Args      []string  `json:"args"`
	Timestamp time.Time `json:"timestamp"`
}

// rawProcEvent mirrors `struct proc_event` in proc.bpf.c byte-for-byte
// (424 bytes). Pad field reflects the C compiler's 4-byte alignment insert
// between args[256] (ends offset 412) and the 8-aligned ts_ns.
type rawProcEvent struct {
	PID      uint32
	PPID     uint32
	UID      uint32
	Comm     [16]byte
	Filename [128]byte
	Args     [256]byte
	Pad      uint32
	TsNs     uint64
}

type ProcProbe struct {
	objs   procProbeObjects
	tp     link.Link
	rd     *ringbuf.Reader
	events chan ProcEvent
}

func (p *ProcProbe) Load() error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("rlimit memlock: %w", err)
	}

	if err := loadProcProbeObjects(&p.objs, nil); err != nil {
		return fmt.Errorf("load procprobe objects: %w", err)
	}

	tp, err := link.Tracepoint("sched", "sched_process_exec", p.objs.HandleSchedProcessExec, nil)
	if err != nil {
		_ = p.objs.Close()
		return fmt.Errorf("attach tracepoint sched/sched_process_exec: %w", err)
	}
	p.tp = tp

	rd, err := ringbuf.NewReader(p.objs.ProcEvents)
	if err != nil {
		_ = p.tp.Close()
		_ = p.objs.Close()
		return fmt.Errorf("ringbuf reader: %w", err)
	}
	p.rd = rd

	p.events = make(chan ProcEvent, 512)
	go p.readLoop()
	return nil
}

func (p *ProcProbe) Events() <-chan ProcEvent { return p.events }

func (p *ProcProbe) Close() error {
	var firstErr error
	if p.rd != nil {
		if err := p.rd.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if p.tp != nil {
		if err := p.tp.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := p.objs.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (p *ProcProbe) readLoop() {
	defer close(p.events)
	for {
		rec, err := p.rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			continue
		}
		ev, ok := parseRecord(rec.RawSample)
		if !ok {
			continue
		}
		p.events <- ev
	}
}

func parseRecord(b []byte) (ProcEvent, bool) {
	var raw rawProcEvent
	if len(b) < binary.Size(raw) {
		return ProcEvent{}, false
	}
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &raw); err != nil {
		return ProcEvent{}, false
	}
	return ProcEvent{
		PID:       raw.PID,
		PPID:      raw.PPID,
		UID:       raw.UID,
		Comm:      cstr(raw.Comm[:]),
		Filename:  cstr(raw.Filename[:]),
		Args:      splitArgv(raw.Args[:]),
		Timestamp: time.Now(),
	}, true
}

// cstr returns the string up to (but not including) the first NUL byte.
func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// splitArgv splits a NUL-separated argv blob. Trailing NUL run is trimmed,
// then we split on NUL. Empty strings are dropped.
func splitArgv(b []byte) []string {
	// Find last non-NUL byte
	end := -1
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			end = i
			break
		}
	}
	if end < 0 {
		return nil
	}
	parts := bytes.Split(b[:end+1], []byte{0})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 0 {
			out = append(out, string(p))
		}
	}
	return out
}
