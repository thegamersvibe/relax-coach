package main

// Optional desktop-only fallback. No tooltip HTML, stats, or player data is
// retained. The WebView never contacts Wowhead or loads its scripts.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const wowheadTooltipURL = "https://nether.wowhead.com/tooltip/item/"
const (
	wowheadMaxPerRefresh    = 200
	wowheadNameTTL          = 30 * 24 * time.Hour
	wowheadNegativeTTL      = 24 * time.Hour
	wowheadRequestInterval  = time.Second
	wowheadBackoffMin       = time.Minute
	wowheadBackoffMax       = 30 * time.Minute
	wowheadRequestTimeout   = 10 * time.Second
	wowheadMaxResponseBytes = 256 << 10
	wowheadMaxCacheBytes    = 4 << 20
	wowheadMaxCacheItems    = 10000
)
const wowheadDataEnv = 16 // Forever; live page/endpoint evidence in docs/farming-wowhead-evidence.md.

type wowheadNameEntry struct {
	Name      string `json:"name,omitempty"`
	Quality   *int   `json:"quality,omitempty"`
	FetchedAt int64  `json:"fetched_at"`
	Missing   bool   `json:"missing,omitempty"`
}

type wowheadNames struct {
	mu              sync.Mutex
	workers         sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	closed, running bool
	enabled         bool
	batchCancel     context.CancelFunc
	path, baseURL   string
	client          *http.Client
	now             func() time.Time
	wait            func(context.Context, time.Duration) error
	nextRequest     time.Time
	retryAt         time.Time
	backoff         time.Duration
	entries         map[int]wowheadNameEntry
	wanted          map[int]bool
}

func newWowheadNames(ctx context.Context, path, baseURL string, client *http.Client) *wowheadNames {
	ctx, cancel := context.WithCancel(ctx)
	// Follow the updater's bounded client convention, but never follow a
	// redirect (another request could leak IDs or bypass the rate budget).
	if client == nil {
		client = &http.Client{}
	}
	bounded := *client
	bounded.Timeout = wowheadRequestTimeout
	if client.Timeout > 0 && client.Timeout < bounded.Timeout {
		bounded.Timeout = client.Timeout
	}
	bounded.Jar = nil
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	w := &wowheadNames{ctx: ctx, cancel: cancel, enabled: true, path: path, baseURL: baseURL, client: &bounded, now: time.Now, wait: waitWowheadRequest, entries: map[int]wowheadNameEntry{}}
	f, err := os.Open(path)
	if err != nil {
		return w
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > wowheadMaxCacheBytes {
		return w
	}
	data, err := io.ReadAll(io.LimitReader(f, wowheadMaxCacheBytes+1))
	var saved map[string]wowheadNameEntry
	if err != nil || len(data) > wowheadMaxCacheBytes || json.Unmarshal(data, &saved) != nil || len(saved) > wowheadMaxCacheItems {
		return w
	}
	for key, e := range saved {
		id, ok := farmNameID(key)
		if ok && e.fresh(w.now()) && (e.Missing && e.Name == "" && e.Quality == nil || !e.Missing && validWowheadName(id, e.Name, e.Quality)) {
			w.entries[id] = e
		}
	}
	return w
}

func validWowheadName(id int, name string, quality *int) bool {
	return validFarmName(name) && !strings.EqualFold(strings.TrimSpace(name), "item "+strconv.Itoa(id)) && (quality == nil || *quality >= 0 && *quality <= 7)
}

func waitWowheadRequest(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e wowheadNameEntry) fresh(now time.Time) bool {
	ttl := wowheadNameTTL
	if e.Missing {
		ttl = wowheadNegativeTTL
	}
	age := now.Sub(time.Unix(e.FetchedAt, 0))
	return e.FetchedAt > 0 && age >= 0 && age < ttl
}

func (w *wowheadNames) setEnabled(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = enabled
	if !enabled && w.batchCancel != nil {
		w.batchCancel()
	}
}

func (w *wowheadNames) Close() {
	w.mu.Lock()
	w.closed = true
	w.cancel()
	w.mu.Unlock()
	w.workers.Wait()
}

// refresh only snapshots local results and admits a single background batch.
// Repeated refreshes replace eligibility, never grow a running queue. In-flight
// results are applied only by the next render, after all local name sources.
func (w *wowheadNames) refresh(db map[string]any, local map[int]farmItemRef) map[int]farmItemRef {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wanted = map[int]bool{}
	for _, raw := range M(db["hist"]) {
		for key, value := range M(raw) {
			id, ok := farmNameID(key)
			_, packed := value.(string)
			if ok && packed && local[id].Name == "" {
				w.wanted[id] = true
			}
		}
	}
	out := map[int]farmItemRef{}
	var ids []int
	for id := range w.wanted {
		if e, ok := w.entries[id]; ok && e.fresh(w.now()) {
			if e.Missing {
				continue
			}
			q := ""
			if e.Quality != nil {
				q = farmQualityClass(float64(*e.Quality))
			}
			out[id] = farmItemRef{Name: e.Name, QualityClass: q}
		} else {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 && !w.running && !w.closed && w.enabled && !w.now().Before(w.retryAt) {
		sort.Ints(ids)
		if len(ids) > wowheadMaxPerRefresh {
			ids = ids[:wowheadMaxPerRefresh]
		}
		w.running = true
		w.workers.Add(1)
		ctx, cancel := context.WithCancel(w.ctx)
		w.batchCancel = cancel
		go w.run(ctx, ids)
	}
	return out
}

func (w *wowheadNames) run(ctx context.Context, ids []int) {
	defer w.workers.Done()
	defer func() { w.mu.Lock(); w.batchCancel(); w.running = false; w.mu.Unlock() }()
	changed := false
	defer func() {
		// Flush once before releasing this batch, including partial progress
		// when a later request fails or is cancelled. No report-path disk I/O.
		if changed {
			w.save()
		}
	}()
	for _, id := range ids {
		w.mu.Lock()
		wanted := w.wanted[id] && !w.closed && w.enabled && ctx.Err() == nil
		delay := w.nextRequest.Sub(w.now())
		w.mu.Unlock()
		if !wanted {
			continue
		}
		if delay > 0 && w.wait(ctx, delay) != nil {
			return
		}
		w.mu.Lock()
		wanted = w.wanted[id] && !w.closed && w.enabled && ctx.Err() == nil
		if wanted {
			w.nextRequest = w.now().Add(wowheadRequestInterval)
		}
		w.mu.Unlock()
		if !wanted {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, "GET", w.baseURL+strconv.Itoa(id)+"?dataEnv="+strconv.Itoa(wowheadDataEnv)+"&locale=0", nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "RelaxCoach/"+Version+" (+https://github.com/"+Repo+")")
		resp, err := w.client.Do(req)
		if err != nil {
			w.failed(ctx, "")
			return
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, wowheadMaxResponseBytes+1))
		resp.Body.Close()
		oversized := len(data) > wowheadMaxResponseBytes
		if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound) || (oversized && resp.StatusCode != http.StatusOK) {
			w.failed(ctx, resp.Header.Get("Retry-After"))
			return
		}
		// Invalid 200s are item-local misses, like 404s, not server pressure.
		// Cache no response data for them and continue with the next ID.
		entry := wowheadNameEntry{Missing: true}
		if resp.StatusCode == http.StatusOK && !oversized {
			// Decode only these two fields; never retain the response body.
			var value struct {
				Name    string `json:"name"`
				Quality *int   `json:"quality"`
			}
			if json.Unmarshal(data, &value) == nil && validWowheadName(id, value.Name, value.Quality) {
				entry = wowheadNameEntry{Name: value.Name, Quality: value.Quality}
			}
		}
		w.mu.Lock()
		if ctx.Err() != nil {
			w.mu.Unlock()
			return
		}
		w.backoff = 0
		entry.FetchedAt = w.now().Unix()
		w.storeLocked(id, entry)
		w.mu.Unlock()
		changed = true
	}
}

// Stop, rather than sleeping through server backpressure. A later refresh can
// retry after the larger of exponential backoff and either Retry-After form.
func (w *wowheadNames) failed(ctx context.Context, header string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	w.backoff *= 2
	if w.backoff < wowheadBackoffMin {
		w.backoff = wowheadBackoffMin
	}
	if w.backoff > wowheadBackoffMax {
		w.backoff = wowheadBackoffMax
	}
	now := w.now()
	w.retryAt = now.Add(w.backoff)
	until, err := http.ParseTime(header)
	if header != "" && strings.Trim(header, "0123456789") == "" {
		// An overflowing decimal is still a delay, not a malformed header.
		// Clamp before duration conversion so neither integer type can wrap.
		seconds, e := strconv.ParseUint(header, 10, 64)
		delay := wowheadBackoffMax
		if e == nil && seconds <= uint64(wowheadBackoffMax/time.Second) {
			delay = time.Duration(seconds) * time.Second
		}
		until, err = now.Add(delay), nil
	}
	if err == nil && until.After(w.retryAt) {
		w.retryAt = until
		if limit := now.Add(wowheadBackoffMax); w.retryAt.After(limit) {
			w.retryAt = limit
		}
	}
}

func (w *wowheadNames) storeLocked(id int, entry wowheadNameEntry) {
	if _, exists := w.entries[id]; !exists && len(w.entries) >= wowheadMaxCacheItems {
		oldest := 0
		for key, e := range w.entries {
			if oldest == 0 || e.FetchedAt < w.entries[oldest].FetchedAt || e.FetchedAt == w.entries[oldest].FetchedAt && key < oldest {
				oldest = key
			}
		}
		delete(w.entries, oldest)
	}
	w.entries[id] = entry
}

// Copy under the lock, but marshal/sync/replace only on the background worker.
// A slow disk must not hold up a report refresh or the opt-out control.
func (w *wowheadNames) save() {
	w.mu.Lock()
	snapshot := make(map[int]wowheadNameEntry, len(w.entries))
	for id, e := range w.entries {
		snapshot[id] = e
	}
	w.mu.Unlock()
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	// This is a local JSON file, never HTML. Avoid six-byte HTML escapes
	// making valid 128-byte names overflow the bounded file on reopen.
	enc.SetEscapeHTML(false)
	err := enc.Encode(snapshot)
	data := encoded.Bytes()
	if err != nil || len(data) > wowheadMaxCacheBytes || w.path == "" {
		return
	}
	if os.MkdirAll(filepath.Dir(w.path), 0700) != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(w.path), ".wowhead-names-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(f.Name(), w.path)
	}
}

// Keep the local cache next to config.json, never in the game or executable tree.
func wowheadCachePath() string {
	if p := configPath(); p != "" {
		return filepath.Join(filepath.Dir(p), fmt.Sprintf("wowhead-names-%d.json", wowheadDataEnv))
	}
	return ""
}
