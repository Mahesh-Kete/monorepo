// Package main is the citadel-agent entrypoint.
//
// Phase 1: loads the network eBPF probe (kprobe tcp_v4_connect), enriches
// each event with a reverse-DNS hostname, and prints JSON to stdout.
// The backend client, process probe, file probe, and policy engine come
// in later phases.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mahesh-Kete/citadel/agent/internal/dns"
	netprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/net"
)

func main() {
	sub := "run"
	if len(os.Args) > 1 {
		sub = os.Args[1]
	}

	switch sub {
	case "run":
		os.Exit(runAgent())
	case "snapshot", "diff":
		fmt.Fprintf(os.Stderr, "citadel-agent: %q is a Phase 0 stub (lands in Phase 3)\n", sub)
		os.Exit(0)
	case "-h", "--help", "help":
		fmt.Fprintln(os.Stderr, "usage: citadel-agent [run|snapshot|diff]")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n", sub)
		os.Exit(2)
	}
}

func runAgent() int {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	probe := &netprobe.NetProbe{}
	if err := probe.Load(); err != nil {
		logger.Error("load network probe", "err", err)
		return 1
	}
	defer func() { _ = probe.Close() }()

	cache := dns.NewCache()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("citadel-agent ready", "kprobe", "tcp_v4_connect")

	enc := json.NewEncoder(os.Stdout)
	events := probe.Events()
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return 0
		case e, ok := <-events:
			if !ok {
				logger.Info("event channel closed; exiting")
				return 0
			}
			e.Hostname = cache.Lookup(e.DstIP)
			if err := enc.Encode(e); err != nil {
				logger.Warn("encode event", "err", err)
			}
		}
	}
}
