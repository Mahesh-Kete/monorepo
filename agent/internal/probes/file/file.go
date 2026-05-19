//go:build linux

// Package file wraps the filesystem-write eBPF probe (tracepoint
// syscalls/sys_enter_openat).
//
// C source: /agent/bpf/file.bpf.c. bpf2go embeds the compiled .o into this
// package at `go generate` time.
package file

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -target arm64 fileProbe ../../../bpf/file.bpf.c -- -I../../../bpf

// FileEvent is the userspace representation of one writeable file open.
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

// rawFileEvent mirrors `struct file_event` (296 bytes, naturally aligned).
type rawFileEvent struct {
	PID      uint32
	PPID     uint32
	UID      uint32
	Comm     [16]byte
	Filename [256]byte
	Flags    int32
	TsNs     uint64
}

// Default workspace prefix. Settable via CITADEL_WATCH_PATH env var.
const defaultWatchPath = "/home/runner/work/"

// Noisy path prefixes to always drop.
var noisyPrefixes = []string{
	"/proc/", "/sys/", "/tmp/", "/dev/", "/run/", "/var/log/", "/var/lib/dpkg/",
}

type FileProbe struct {
	objs      fileProbeObjects
	tp        link.Link
	rd        *ringbuf.Reader
	events    chan FileEvent
	watchPath string
}

func (f *FileProbe) Load() error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("rlimit memlock: %w", err)
	}

	if err := loadFileProbeObjects(&f.objs, nil); err != nil {
		return fmt.Errorf("load fileprobe objects: %w", err)
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_openat", f.objs.HandleOpenatEnter, nil)
	if err != nil {
		_ = f.objs.Close()
		return fmt.Errorf("attach tracepoint syscalls/sys_enter_openat: %w", err)
	}
	f.tp = tp

	rd, err := ringbuf.NewReader(f.objs.FileEvents)
	if err != nil {
		_ = f.tp.Close()
		_ = f.objs.Close()
		return fmt.Errorf("ringbuf reader: %w", err)
	}
	f.rd = rd

	f.watchPath = os.Getenv("CITADEL_WATCH_PATH")
	if f.watchPath == "" {
		f.watchPath = defaultWatchPath
	}

	f.events = make(chan FileEvent, 512)
	go f.readLoop()
	return nil
}

func (f *FileProbe) Events() <-chan FileEvent { return f.events }

func (f *FileProbe) Close() error {
	var firstErr error
	if f.rd != nil {
		if err := f.rd.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if f.tp != nil {
		if err := f.tp.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := f.objs.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (f *FileProbe) readLoop() {
	defer close(f.events)
	for {
		rec, err := f.rd.Read()
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
		if !f.shouldKeep(ev.Filename) {
			continue
		}
		f.events <- ev
	}
}

// shouldKeep drops paths that aren't under the watch prefix, or that match
// known-noisy roots, or that are our own files.
func (f *FileProbe) shouldKeep(path string) bool {
	if path == "" {
		return false
	}
	for _, np := range noisyPrefixes {
		if strings.HasPrefix(path, np) {
			return false
		}
	}
	if strings.Contains(path, "citadel-agent") || strings.Contains(path, "citadel-baseline") {
		return false
	}
	// If a watch prefix is set, require the path to be under it.
	if f.watchPath != "" && !strings.HasPrefix(path, f.watchPath) {
		return false
	}
	return true
}

func parseRecord(b []byte) (FileEvent, bool) {
	var raw rawFileEvent
	if len(b) < binary.Size(raw) {
		return FileEvent{}, false
	}
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &raw); err != nil {
		return FileEvent{}, false
	}
	return FileEvent{
		PID:       raw.PID,
		PPID:      raw.PPID,
		UID:       raw.UID,
		Comm:      cstr(raw.Comm[:]),
		Filename:  cstr(raw.Filename[:]),
		Flags:     raw.Flags,
		FlagsStr:  renderFlags(raw.Flags),
		Timestamp: time.Now(),
	}, true
}

func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// renderFlags returns a human-readable bitmask string for the openat flags
// we care about. Other bits are ignored — full openat flag set is large.
func renderFlags(f int32) string {
	const (
		oWronly = 0x1
		oRdwr   = 0x2
		oCreat  = 0x40
		oTrunc  = 0x200
		oAppend = 0x400
		oExcl   = 0x80
	)
	var parts []string
	if f&oWronly != 0 {
		parts = append(parts, "WRONLY")
	}
	if f&oRdwr != 0 {
		parts = append(parts, "RDWR")
	}
	if f&oCreat != 0 {
		parts = append(parts, "CREAT")
	}
	if f&oTrunc != 0 {
		parts = append(parts, "TRUNC")
	}
	if f&oAppend != 0 {
		parts = append(parts, "APPEND")
	}
	if f&oExcl != 0 {
		parts = append(parts, "EXCL")
	}
	if len(parts) == 0 {
		return "0"
	}
	return strings.Join(parts, "|")
}
