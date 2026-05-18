// Package backend implements the agent → Citadel backend HTTP client.
//
// Events are submitted via Send() which is non-blocking — if the internal
// queue is full we drop and log rather than back-pressure the probes (a
// blocked probe means lost kernel events, which is worse than lost
// userspace events).
//
// A single goroutine batches events: up to 100 events per batch or one
// flush every 2 seconds, whichever fires first. Each batch POSTs to
// /api/events with up to 3 retries (exponential backoff 1s, 2s, 4s) before
// the batch is dropped.
//
// Local-dev mode: if backendURL is "", Send() writes the event as JSON to
// stdout instead of enqueuing. Useful for `sudo ./citadel-agent run`
// without a backend running.
package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Mahesh-Kete/citadel/agent/internal/events"
)

const (
	queueCapacity = 10000
	batchSize     = 100
	batchInterval = 2 * time.Second
	httpTimeout   = 10 * time.Second
	maxAttempts   = 3
)

type Client struct {
	backendURL string
	httpClient *http.Client
	queue      chan events.Event
	logger     *slog.Logger

	// stdoutEnc is used when backendURL is empty (local-dev mode).
	stdoutEnc *json.Encoder

	stopCh   chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

// New creates a Client. backendURL may be "" to enable local-dev stdout
// mode. logger must be non-nil.
func New(backendURL string, logger *slog.Logger) *Client {
	return &Client{
		backendURL: strings.TrimRight(backendURL, "/"),
		httpClient: &http.Client{Timeout: httpTimeout},
		queue:      make(chan events.Event, queueCapacity),
		logger:     logger,
		stdoutEnc:  json.NewEncoder(os.Stdout),
		stopCh:     make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start launches the background batcher goroutine. Returns immediately.
// ctx cancellation triggers a drain + flush, same as Stop().
func (c *Client) Start(ctx context.Context) {
	if c.backendURL == "" {
		// In local-dev mode the goroutine has nothing to do; mark done.
		close(c.done)
		return
	}
	go c.runBatcher(ctx)
}

// Send enqueues an event (non-blocking). On overflow the event is dropped
// and a warning is logged.
//
// In local-dev mode (backendURL == "") the event is JSON-encoded to stdout.
func (c *Client) Send(e events.Event) {
	if c.backendURL == "" {
		_ = c.stdoutEnc.Encode(e)
		return
	}
	select {
	case <-c.stopCh:
		return
	case c.queue <- e:
	default:
		c.logger.Warn("event queue full; dropping", "type", e.Type, "id", e.ID)
	}
}

// Stop signals the batcher to flush + drain + exit. Blocks until the
// goroutine returns or timeout elapses (whichever first). Safe to call
// multiple times.
func (c *Client) Stop(timeout time.Duration) {
	c.stopOnce.Do(func() { close(c.stopCh) })
	select {
	case <-c.done:
	case <-time.After(timeout):
		c.logger.Warn("backend client stop timed out", "timeout", timeout)
	}
}

func (c *Client) runBatcher(ctx context.Context) {
	defer close(c.done)

	batch := make([]events.Event, 0, batchSize)
	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		c.sendBatch(batch)
		batch = batch[:0]
	}

	drainAndFlush := func() {
		for {
			select {
			case e := <-c.queue:
				batch = append(batch, e)
				if len(batch) >= batchSize {
					flush()
				}
			default:
				flush()
				return
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			drainAndFlush()
			return
		case <-c.stopCh:
			drainAndFlush()
			return
		case e := <-c.queue:
			batch = append(batch, e)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// sendBatch POSTs one batch with up to 3 attempts and exponential backoff.
func (c *Client) sendBatch(batch []events.Event) {
	body, err := json.Marshal(struct {
		Events []events.Event `json:"events"`
	}{Events: batch})
	if err != nil {
		c.logger.Warn("marshal batch", "err", err)
		return
	}

	url := c.backendURL + "/api/events"
	backoff := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ok, retryable := c.postOnce(url, body)
		if ok {
			return
		}
		if !retryable {
			return
		}
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	c.logger.Warn("dropping batch after retries", "count", len(batch), "attempts", maxAttempts)
}

// postOnce sends one HTTP request. Returns (success, retryable).
//   - success=true: 2xx; we're done.
//   - retryable=true: network error or 5xx; caller should back off + retry.
//   - retryable=false: 4xx; the backend is rejecting us, retrying won't help.
func (c *Client) postOnce(url string, body []byte) (success, retryable bool) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		c.logger.Warn("build request", "err", err)
		return false, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Debug("post failed", "err", err)
		return false, true
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, false
	}
	if resp.StatusCode >= 500 {
		c.logger.Debug("server error", "status", resp.StatusCode)
		return false, true
	}
	c.logger.Warn("backend rejected batch", "status", resp.StatusCode, "url", url)
	return false, false
}

// PostDetection is a convenience used by the snapshot/diff subcommands to
// emit file_tamper events directly without going through the batcher
// (the subcommands are short-lived processes; batching would be silly).
func (c *Client) PostDetection(ctx context.Context, ev events.Event) error {
	if c.backendURL == "" {
		return c.stdoutEnc.Encode(ev)
	}
	body, err := json.Marshal(struct {
		Events []events.Event `json:"events"`
	}{Events: []events.Event{ev}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.backendURL+"/api/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("backend responded %d", resp.StatusCode)
	}
	return nil
}
