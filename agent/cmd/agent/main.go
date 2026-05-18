// Package main is the citadel-agent entrypoint.
//
// Subcommands:
//   - run                Production entrypoint: load all eBPF probes (net,
//                        proc, file), optionally attach the cgroup_skb block
//                        program, enrich every event, batch + POST to the
//                        backend (or stream to stdout in local-dev mode),
//                        and run the userspace enforcer in parallel.
//                        Requires Linux + root.
//   - snapshot           Walk a directory and SHA256 every file → baseline JSON.
//   - diff               Compare baseline JSON to current path; emit
//                        file_tamper events to the backend.
//
// Signals:
//   - SIGINT / SIGTERM   Graceful shutdown.
//   - SIGHUP             Reload policy from the backend without restarting probes.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mahesh-Kete/citadel/agent/internal/backend"
	"github.com/Mahesh-Kete/citadel/agent/internal/dns"
	"github.com/Mahesh-Kete/citadel/agent/internal/enforcer"
	"github.com/Mahesh-Kete/citadel/agent/internal/events"
	"github.com/Mahesh-Kete/citadel/agent/internal/integrity"
	"github.com/Mahesh-Kete/citadel/agent/internal/policy"
	blockprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/block"
	fileprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/file"
	netprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/net"
	procprobe "github.com/Mahesh-Kete/citadel/agent/internal/probes/proc"
	"github.com/Mahesh-Kete/citadel/agent/internal/proctree"
	"github.com/Mahesh-Kete/citadel/agent/internal/workflow"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "snapshot":
			os.Exit(cmdSnapshot(os.Args[2:]))
		case "diff":
			os.Exit(cmdDiff(os.Args[2:]))
		case "help", "-h", "--help":
			printUsage(os.Stderr)
			os.Exit(0)
		case "run":
			os.Exit(cmdRun(os.Args[2:]))
		}
	}
	os.Exit(cmdRun(os.Args[1:]))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  citadel-agent run [flags]                                  production mode")
	fmt.Fprintln(w, "  citadel-agent snapshot --path PATH [--out FILE]")
	fmt.Fprintln(w, "  citadel-agent diff --before FILE --after-path PATH [--backend-url URL]")
}

// ----------------------------------------------------------------------------
// run subcommand
// ----------------------------------------------------------------------------

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	backendURL := fs.String("backend-url", "", "Citadel backend URL; empty = stdout streaming")
	mode := fs.String("mode", "", "audit | block (override the policy's mode)")
	policyPath := fs.String("policy", "", "(reserved) local policy file path")
	metaFile := fs.String("meta-file", "/tmp/citadel-meta.json", "GitHub Actions metadata JSON path")
	watchPath := fs.String("watch-path", "/home/runner/work/", "workspace root for file probe filter")
	cgroupPath := fs.String("cgroup", "/sys/fs/cgroup", "cgroup v2 path for block-mode attachment")
	logFilePath := fs.String("log-file", "", "duplicate logs to this file (e.g. /var/log/citadel-agent.log)")
	_ = policyPath // local policy files are a Phase 8 stretch goal
	_ = fs.Parse(args)

	logger, closeLog := newLogger(*logFilePath)
	defer closeLog()

	if *watchPath != "" {
		_ = os.Setenv("CITADEL_WATCH_PATH", *watchPath)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	meta := workflow.NewLoader(*metaFile)

	// --- Policy: load from backend, fall back to permissive default ---
	pol, err := policy.LoadFromBackend(ctx, *backendURL, meta.Get().Repository, meta.Get().Workflow)
	if err != nil {
		logger.Warn("load policy", "err", err)
	}
	if *mode != "" {
		pol.Mode = *mode
	}
	polWatcher := policy.NewWatcher(pol)
	logger.Info("policy loaded",
		"name", pol.Name, "mode", pol.Mode,
		"allowed_domains", len(pol.AllowedDomains),
		"detection_actions", len(pol.DetectionActions))

	// --- Backend client (or local-dev stdout streaming) ---
	bc := backend.New(*backendURL, logger)
	bc.Start(ctx)

	// --- Block program (only in block mode, only on Linux) ---
	var bp *blockprobe.BlockProgram
	if pol.Mode == "block" {
		bp = &blockprobe.BlockProgram{}
		if err := bp.Load(*cgroupPath); err != nil {
			logger.Warn("block-mode requested but program failed to load; falling back to audit",
				"err", err)
			bp = nil
			pol.Mode = "audit"
			polWatcher.Set(pol)
		} else {
			logger.Info("block program attached", "cgroup", *cgroupPath)
		}
	}

	// --- Probes ---
	np := &netprobe.NetProbe{}
	if err := np.Load(); err != nil {
		logger.Error("load network probe", "err", err)
		bc.Stop(2 * time.Second)
		if bp != nil {
			_ = bp.Close()
		}
		return 1
	}
	pp := &procprobe.ProcProbe{}
	if err := pp.Load(); err != nil {
		logger.Error("load process probe", "err", err)
		_ = np.Close()
		bc.Stop(2 * time.Second)
		if bp != nil {
			_ = bp.Close()
		}
		return 1
	}
	fp := &fileprobe.FileProbe{}
	if err := fp.Load(); err != nil {
		logger.Error("load file probe", "err", err)
		_ = pp.Close()
		_ = np.Close()
		bc.Stop(2 * time.Second)
		if bp != nil {
			_ = bp.Close()
		}
		return 1
	}

	// --- Enforcer (kill actions for high-severity detections) ---
	enf := enforcer.New(*backendURL, logger, polWatcher)
	enf.Start(ctx)

	// --- SIGHUP → reload policy without restarting probes ---
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	defer signal.Stop(hupCh)
	go func() {
		for range hupCh {
			logger.Info("SIGHUP received; reloading policy")
			newPol, err := policy.LoadFromBackend(ctx, *backendURL,
				meta.Get().Repository, meta.Get().Workflow)
			if err != nil {
				logger.Warn("reload policy", "err", err)
				continue
			}
			if *mode != "" {
				newPol.Mode = *mode
			}
			polWatcher.Set(newPol)
			logger.Info("policy reloaded", "name", newPol.Name, "mode", newPol.Mode)
		}
	}()

	// --- Enrichment state ---
	dnsCache := dns.NewCache()
	tree := proctree.New()

	logger.Info("citadel-agent ready", "probes", []string{"net", "proc", "file"}, "mode", pol.Mode)

	netCh := np.Events()
	procCh := pp.Events()
	fileCh := fp.Events()

	var doneCh <-chan struct{} = ctx.Done()
	shuttingDown := false

	for {
		if netCh == nil && procCh == nil && fileCh == nil {
			break
		}
		select {
		case <-doneCh:
			if !shuttingDown {
				shuttingDown = true
				logger.Info("shutdown signal received; closing probes")
				_ = np.Close()
				_ = pp.Close()
				_ = fp.Close()
			}
			doneCh = nil

		case e, ok := <-netCh:
			if !ok {
				netCh = nil
				continue
			}
			e.Hostname = dnsCache.Lookup(e.DstIP)
			e.ProcessChain = tree.AncestryComms(e.PID)

			// In block mode: if the destination domain isn't allow-listed,
			// add the IP to the kernel block map so subsequent packets to
			// it get dropped.
			if bp != nil && polWatcher.Get().ShouldBlockDomain(e.Hostname) {
				if err := bp.Block(e.DstIP); err != nil {
					logger.Warn("block ip", "ip", e.DstIP, "err", err)
				} else {
					logger.Info("blocked egress",
						"hostname", e.Hostname, "ip", e.DstIP, "port", e.DstPort, "comm", e.Comm)
				}
			}

			bc.Send(events.NewFromNetEvent(e, tree, dnsCache, meta.Get()))

		case e, ok := <-procCh:
			if !ok {
				procCh = nil
				continue
			}
			tree.Add(e)
			bc.Send(events.NewFromProcEvent(e, meta.Get()))

		case e, ok := <-fileCh:
			if !ok {
				fileCh = nil
				continue
			}
			bc.Send(events.NewFromFileEvent(e, tree, meta.Get()))
		}
	}

	if !shuttingDown {
		_ = np.Close()
		_ = pp.Close()
		_ = fp.Close()
	}
	if bp != nil {
		_ = bp.Close()
	}
	bc.Stop(5 * time.Second)
	logger.Info("citadel-agent exited cleanly")
	return 0
}

// avoid unused-import error for net (used indirectly through gonet alias in
// probes); kept here so future callers can refer to net.IP without re-import
var _ = net.IPv4

func newLogger(logFilePath string) (*slog.Logger, func()) {
	var w io.Writer = os.Stderr
	closeFn := func() {}
	if logFilePath != "" {
		if f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			w = io.MultiWriter(os.Stderr, f)
			closeFn = func() { _ = f.Close() }
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not open %s for logging: %v (stderr-only)\n", logFilePath, err)
		}
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})), closeFn
}

// ----------------------------------------------------------------------------
// snapshot subcommand (unchanged from Phase 3)
// ----------------------------------------------------------------------------

func cmdSnapshot(args []string) int {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	path := fs.String("path", "", "directory to snapshot (required)")
	out := fs.String("out", "-", "output JSON file path; - for stdout")
	_ = fs.Parse(args)

	if *path == "" {
		fmt.Fprintln(os.Stderr, "snapshot: --path is required")
		return 2
	}
	snap, err := integrity.Snapshot(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		return 1
	}
	if err := integrity.WriteJSON(*out, snap); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot: write:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "snapshot: hashed %d files under %s -> %s\n", len(snap), *path, *out)
	return 0
}

// ----------------------------------------------------------------------------
// diff subcommand (unchanged from Phase 4)
// ----------------------------------------------------------------------------

func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	before := fs.String("before", "", "baseline JSON file (required)")
	afterPath := fs.String("after-path", "", "current workspace directory (required)")
	backendURL := fs.String("backend-url", "", "if set, POST file_tamper events here; otherwise stdout")
	metaFile := fs.String("meta-file", "/tmp/citadel-meta.json", "GitHub Actions metadata JSON path")
	_ = fs.Parse(args)

	if *before == "" || *afterPath == "" {
		fmt.Fprintln(os.Stderr, "diff: --before and --after-path are required")
		return 2
	}
	beforeSnap, err := integrity.ReadJSON(*before)
	if err != nil {
		fmt.Fprintln(os.Stderr, "diff: read baseline:", err)
		return 1
	}
	afterSnap, err := integrity.Snapshot(*afterPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "diff: snapshot:", err)
		return 1
	}
	diffs := integrity.Diff(beforeSnap, afterSnap)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	bc := backend.New(*backendURL, logger)
	wm := workflow.NewLoader(*metaFile).Get()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, d := range diffs {
		ev := events.NewFromFileDiff(d, wm)
		if err := bc.PostDetection(ctx, ev); err != nil {
			logger.Warn("post file_tamper", "path", d.Path, "err", err)
		}
	}
	fmt.Fprintf(os.Stderr, "diff: %d changes\n", len(diffs))
	return 0
}
