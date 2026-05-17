// Package dns is a tiny reverse-DNS lookup cache used to enrich network
// events with a hostname.
//
// LIMITATION: We use synchronous net.LookupAddr against the system resolver.
// This means (a) the first lookup per IP blocks the calling goroutine for a
// few ms, and (b) many CDN / cloud IPs don't reverse-resolve to the name the
// process actually dialed (a `curl https://github.com` connect to 140.82.x.x
// often reverses to `lb-...-iad.github.com.` if at all). A more complete
// approach would be to attach a second eBPF program to udp_recvmsg, sniff DNS
// responses, and prepopulate this cache from the forward A/AAAA records the
// process itself just looked up — guaranteed-correct hostname per IP. That's
// out of scope for the hackathon; we accept the noisy reverse-DNS approach.
package dns

import (
	"net"
	"strings"
	"sync"
	"time"
)

const defaultTTL = 5 * time.Minute

type cacheEntry struct {
	hostname string
	at       time.Time
}

type Cache struct {
	mu  sync.RWMutex
	m   map[string]cacheEntry
	ttl time.Duration
}

// NewCache returns a cache with the default 5-minute TTL.
func NewCache() *Cache {
	return &Cache{m: make(map[string]cacheEntry), ttl: defaultTTL}
}

// Lookup returns the cached hostname for ip, or performs a fresh reverse-DNS
// resolution on miss / expired entry. Empty string on failure (callers can
// distinguish "no hostname" from "lookup failed" — we don't, intentionally).
func (c *Cache) Lookup(ip net.IP) string {
	if ip == nil {
		return ""
	}
	key := ip.String()

	c.mu.RLock()
	entry, ok := c.m[key]
	c.mu.RUnlock()
	if ok && time.Since(entry.at) < c.ttl {
		return entry.hostname
	}

	name := ""
	if names, err := net.LookupAddr(key); err == nil && len(names) > 0 {
		// LookupAddr returns FQDNs with a trailing dot (RFC convention).
		name = strings.TrimSuffix(names[0], ".")
	}

	c.mu.Lock()
	c.m[key] = cacheEntry{hostname: name, at: time.Now()}
	c.mu.Unlock()
	return name
}
