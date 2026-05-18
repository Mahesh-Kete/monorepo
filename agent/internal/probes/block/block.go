//go:build linux

// Package block manages the cgroup_skb/egress eBPF program used to drop
// outbound packets to disallowed destinations.
//
// Userspace (the agent's main loop) calls Block(ip) when it decides a
// connection should not be allowed. The eBPF program in /agent/bpf/block.bpf.c
// looks up every egress packet's daddr in a hash map and drops on a hit.
package block

import (
	"encoding/binary"
	"fmt"
	gonet "net"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -target amd64,arm64 BlockProgram ../../../bpf/block.bpf.c -- -I../../../bpf

// BlockProgram holds the loaded eBPF objects + the cgroup attachment.
type BlockProgram struct {
	objs blockProgramObjects
	link link.Link
}

// Load loads the BPF object and attaches it to the cgroup at cgroupPath.
// For the hackathon we use the cgroup v2 root mount (`/sys/fs/cgroup`); a
// real installation might pick a per-runner cgroup.
func (b *BlockProgram) Load(cgroupPath string) error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("rlimit memlock: %w", err)
	}
	if err := loadBlockProgramObjects(&b.objs, nil); err != nil {
		return fmt.Errorf("load blockprogram objects: %w", err)
	}
	l, err := link.AttachCgroup(link.CgroupOptions{
		Path:    cgroupPath,
		Attach:  ebpf.AttachCGroupInetEgress,
		Program: b.objs.CgEgressFilter,
	})
	if err != nil {
		_ = b.objs.Close()
		return fmt.Errorf("attach cgroup egress: %w", err)
	}
	b.link = l
	return nil
}

// Block adds ip to the kernel's blocked-IPs map. Subsequent egress packets
// with that destination IPv4 are dropped.
func (b *BlockProgram) Block(ip gonet.IP) error {
	key, err := ipv4ToBE32(ip)
	if err != nil {
		return err
	}
	var val uint8 = 1
	return b.objs.BlockedIps.Update(key, val, ebpf.UpdateAny)
}

// Unblock removes ip from the blocked-IPs map.
func (b *BlockProgram) Unblock(ip gonet.IP) error {
	key, err := ipv4ToBE32(ip)
	if err != nil {
		return err
	}
	if err := b.objs.BlockedIps.Delete(key); err != nil {
		if err != ebpf.ErrKeyNotExist {
			return err
		}
	}
	return nil
}

// Close detaches the cgroup program and frees the loaded objects.
func (b *BlockProgram) Close() error {
	var firstErr error
	if b.link != nil {
		if err := b.link.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := b.objs.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// ipv4ToBE32 converts a 4-byte IPv4 into the network-byte-order uint32 used
// as the map key. The map key must match exactly how the BPF program reads
// daddr from the packet (which is just the raw bytes from offset 16 of the
// IP header — i.e. network byte order).
func ipv4ToBE32(ip gonet.IP) (uint32, error) {
	v4 := ip.To4()
	if v4 == nil {
		return 0, fmt.Errorf("not an IPv4 address: %s", ip)
	}
	return binary.BigEndian.Uint32(v4), nil
}
