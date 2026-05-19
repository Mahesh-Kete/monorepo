// Package dns is a tiny reverse-DNS lookup cache used to enrich network
// events with a hostname.
//
// Lookup is non-blocking: on a cache miss we kick off the resolver in a
// background goroutine and return "" immediately. The hot path (the agent's
// main event loop) is never stalled on the system resolver — that was
// previously causing eBPF ringbuf overruns whose events were silently
// dropped, so the dashboard appeared empty for any new destination.
//
// A more complete approach would be to attach a second eBPF program to
// udp_recvmsg, sniff DNS responses, and prepopulate this cache from the
// forward A/AAAA records the process itself just looked up — guaranteed-
// correct hostname per IP. That's out of scope for the hackathon.
package dns

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL     = 5 * time.Minute
	lookupTimeout  = 2 * time.Second
	maxInflight    = 64
)

type cacheEntry struct {
	hostname string
	at       time.Time
}

type Cache struct {
	mu        sync.RWMutex
	m         map[string]cacheEntry
	ttl       time.Duration
	inflight  map[string]struct{} // IPs currently being resolved
	resolveCh chan string         // bounded queue of IPs to resolve
	once      sync.Once
}

// NewCache returns a cache with the default 5-minute TTL.
func NewCache() *Cache {
	return &Cache{
		m:         make(map[string]cacheEntry),
		ttl:       defaultTTL,
		inflight:  make(map[string]struct{}),
		resolveCh: make(chan string, maxInflight),
	}
}

// Lookup returns the cached hostname for ip if present and fresh. On miss it
// schedules a background reverse-DNS resolution and returns "" immediately —
// the caller never blocks. Subsequent lookups for the same IP get the
// resolved value once the background goroutine finishes.
func (c *Cache) Lookup(ip net.IP) string {
	if ip == nil {
		return ""
	}
	c.once.Do(c.startResolver)
	key := ip.String()

	c.mu.RLock()
	entry, ok := c.m[key]
	c.mu.RUnlock()
	if ok && time.Since(entry.at) < c.ttl {
		return entry.hostname
	}

	// Schedule a background resolution. We drop the request if the queue is
	// full or already in-flight; the next event for this IP will retry.
	c.mu.Lock()
	if _, busy := c.inflight[key]; !busy {
		select {
		case c.resolveCh <- key:
			c.inflight[key] = struct{}{}
		default:
			// queue full — skip; we'll try again next time.
		}
	}
	c.mu.Unlock()
	return ""
}

func (c *Cache) startResolver() {
	go func() {
		for ip := range c.resolveCh {
			c.resolveOne(ip)
		}
	}()
}

func (c *Cache) resolveOne(ip string) {
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	name := ""
	r := net.DefaultResolver
	if names, err := r.LookupAddr(ctx, ip); err == nil && len(names) > 0 {
		name = strings.TrimSuffix(names[0], ".")
	}

	c.mu.Lock()
	c.m[ip] = cacheEntry{hostname: name, at: time.Now()}
	delete(c.inflight, ip)
	c.mu.Unlock()
}
