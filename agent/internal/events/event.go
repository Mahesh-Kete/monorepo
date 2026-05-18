// Package events defines the unified Event schema that the agent ships to
// the backend. The four probe types (network, process, file, file_tamper)
// and the detector's findings (detection) all serialize as this single
// shape, so the backend's POST /api/events handler only needs to deal with
// one struct.
//
// Constructor functions translate from each probe's native event type into
// the unified Event, attaching process ancestry and reverse-DNS enrichment
// at the same point. This keeps main.go's event loop short.
package events

import (
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/Mahesh-Kete/citadel/agent/internal/dns"
	"github.com/Mahesh-Kete/citadel/agent/internal/integrity"
	fileprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/file"
	netprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/net"
	procprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/proc"
	"github.com/Mahesh-Kete/citadel/agent/internal/proctree"
	"github.com/Mahesh-Kete/citadel/agent/internal/workflow"
)

// EventType values used in Event.Type. Detector findings ("detection") are
// included for forward-compatibility with Phase 6.
const (
	TypeNetwork    = "network"
	TypeProcess    = "process"
	TypeFile       = "file"
	TypeFileTamper = "file_tamper"
	TypeDetection  = "detection"
)

// Event is the wire format for everything the agent ships.
// At most one of Network / Process / File is populated; the discriminant is
// Type. ProcessChain and WorkflowMeta are common enrichments.
type Event struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Timestamp    time.Time     `json:"timestamp"`
	Network      *NetData      `json:"network,omitempty"`
	Process      *ProcessData  `json:"process,omitempty"`
	File         *FileData     `json:"file,omitempty"`
	ProcessChain []string      `json:"process_chain,omitempty"`
	WorkflowMeta workflow.Meta `json:"workflow"`
}

type NetData struct {
	SrcIP    string `json:"src_ip,omitempty"`
	DstIP    string `json:"dst_ip"`
	DstPort  uint16 `json:"dst_port"`
	Hostname string `json:"hostname,omitempty"`
	// Process is the comm of the process that opened the connection.
	// The full ancestry chain is on Event.ProcessChain.
	Process string `json:"process,omitempty"`
}

type ProcessData struct {
	PID      uint32   `json:"pid"`
	PPID     uint32   `json:"ppid"`
	UID      uint32   `json:"uid"`
	Comm     string   `json:"comm"`
	Filename string   `json:"filename"`
	Args     []string `json:"args,omitempty"`
}

type FileData struct {
	Path    string `json:"path"`
	Flags   string `json:"flags,omitempty"`
	OldHash string `json:"old_hash,omitempty"`
	NewHash string `json:"new_hash,omitempty"`
	// Action is "modified" | "added" | "deleted" for file_tamper events;
	// empty for the live file probe.
	Action string `json:"action,omitempty"`
}

// NewFromNetEvent builds a unified Event from a raw NetEvent, attaching
// reverse-DNS hostname (if not already set), workflow metadata, and the
// process ancestry chain from the proctree.
func NewFromNetEvent(e netprobe.NetEvent, tree *proctree.Tree, cache *dns.Cache, meta workflow.Meta) Event {
	hostname := e.Hostname
	if hostname == "" && e.DstIP != nil {
		hostname = cache.Lookup(e.DstIP)
	}
	chain := e.ProcessChain
	if len(chain) == 0 {
		chain = tree.AncestryComms(e.PID)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      TypeNetwork,
		Timestamp: e.Timestamp,
		Network: &NetData{
			DstIP:    ipString(e.DstIP),
			DstPort:  e.DstPort,
			Hostname: hostname,
			Process:  e.Comm,
		},
		ProcessChain: chain,
		WorkflowMeta: meta,
	}
}

// NewFromProcEvent builds a unified Event from a raw ProcEvent. ProcessChain
// is intentionally NOT populated here — the proctree is updated *from* this
// event; the ancestry chain at exec-time is just the parent.
func NewFromProcEvent(e procprobe.ProcEvent, meta workflow.Meta) Event {
	return Event{
		ID:        uuid.NewString(),
		Type:      TypeProcess,
		Timestamp: e.Timestamp,
		Process: &ProcessData{
			PID:      e.PID,
			PPID:     e.PPID,
			UID:      e.UID,
			Comm:     e.Comm,
			Filename: e.Filename,
			Args:     e.Args,
		},
		WorkflowMeta: meta,
	}
}

// NewFromFileEvent builds a unified Event from a raw FileEvent. The path
// is taken from FileEvent.Filename; we use FlagsStr for the human-readable
// flag bitmask string.
func NewFromFileEvent(e fileprobe.FileEvent, tree *proctree.Tree, meta workflow.Meta) Event {
	return Event{
		ID:        uuid.NewString(),
		Type:      TypeFile,
		Timestamp: e.Timestamp,
		File: &FileData{
			Path:  e.Filename,
			Flags: e.FlagsStr,
		},
		ProcessChain: tree.AncestryComms(e.PID),
		WorkflowMeta: meta,
	}
}

// NewFromFileDiff builds an Event of type "file_tamper" from one entry in
// an integrity Diff. Called by the `diff` subcommand and by the post-job
// hook in the GitHub Action.
func NewFromFileDiff(d integrity.FileDiff, meta workflow.Meta) Event {
	return Event{
		ID:        uuid.NewString(),
		Type:      TypeFileTamper,
		Timestamp: time.Now(),
		File: &FileData{
			Path:    d.Path,
			OldHash: d.OldHash,
			NewHash: d.NewHash,
			Action:  d.Action,
		},
		WorkflowMeta: meta,
	}
}

func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}
