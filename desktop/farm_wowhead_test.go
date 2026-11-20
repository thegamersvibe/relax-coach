package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func wowheadHistory(ids ...int) map[string]any {
	rows := map[string]any{}
	for _, id := range ids {
		rows[fmt.Sprint(id)] = "packed"
	}
	return map[string]any{"hist": map[string]any{"Test house": rows}}
}

func waitWowhead(t *testing.T, w *wowheadNames) {
	t.Helper()
	done := make(chan struct{})
	go func() { w.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("background lookup did not finish")
	}
}

func TestWowheadLookupAndMinimalCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		if req.URL.RequestURI() != "/tooltip/item/900001?dataEnv=16&locale=0" {
			t.Errorf("unexpected URL: %s", req.URL)
		}
		if req.Header.Get("User-Agent") != "RelaxCoach/"+Version+" (+https://github.com/"+Repo+")" {
			t.Error("missing app/version/repo User-Agent")
		}
		if req.Method != "GET" || req.ContentLength > 0 || req.Header.Get("Cookie") != "" {
			t.Error("only anonymous item-ID requests allowed")
		}
		fmt.Fprint(rw, `{"name":"Test Item","quality":0,"tooltip":"DO NOT STORE","stats":{"x":99},"icon":"ignored"}`)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "wowhead-names.json")
	w := newWowheadNames(context.Background(), path, server.URL+"/tooltip/item/", server.Client())
	defer w.Close()
	db := wowheadHistory(900001)
	w.refresh(db, nil)
	waitWowhead(t, w)
	got := w.refresh(db, nil)[900001]
	if got.Name != "Test Item" || got.QualityClass != "q0" || calls.Load() != 1 {
		t.Fatalf("lookup: %+v, requests=%d", got, calls.Load())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	entry := stored["900001"]
	if len(stored) != 1 || len(entry) != 3 || string(entry["name"]) != `"Test Item"` || string(entry["quality"]) != "0" || len(entry["fetched_at"]) == 0 {
		t.Fatalf("only name, quality, fetch time allowed: %s", data)
	}
	w2 := newWowheadNames(context.Background(), path, server.URL+"/tooltip/item/", server.Client())
	defer w2.Close()
	got = w2.refresh(db, nil)[900001]
	waitWowhead(t, w2)
	if got.Name != "Test Item" || calls.Load() != 1 {
		t.Fatal("restart must use the local cache")
	}
}

type wowheadClock struct {
	mu    sync.Mutex
	value time.Time
}

func (c *wowheadClock) now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.value }
func (c *wowheadClock) advance(d time.Duration) { c.mu.Lock(); c.value = c.value.Add(d); c.mu.Unlock() }
func (c *wowheadClock) wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.advance(d)
	return nil
}
func fakeWowheadTime(w *wowheadNames) *wowheadClock {
	c := &wowheadClock{value: time.Now().Truncate(time.Second)}
	w.now = c.now
	w.wait = c.wait
	return c
}

func TestWowheadBoundsAndRate(t *testing.T) {
	var times []time.Time
	var clock *wowheadClock
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		times = append(times, clock.now())
		fmt.Fprint(rw, `{"name":"Test Item","quality":3}`)
	}))
	defer server.Close()
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	clock = fakeWowheadTime(w)
	ids := make([]int, 201)
	for i := range ids {
		ids[i] = 900000 + i
	}
	w.refresh(wowheadHistory(ids...), nil)
	waitWowhead(t, w)
	if len(times) != 200 {
		t.Fatalf("per-refresh limit: requests=%d, want 200", len(times))
	}
	w.refresh(wowheadHistory(ids...), nil)
	waitWowhead(t, w)
	if len(times) != 201 {
		t.Fatalf("next refresh must finish remaining ID: %d", len(times))
	}
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) < time.Second {
			t.Fatalf("requests %d/%d too close: %v", i-1, i, times[i].Sub(times[i-1]))
		}
	}
}

func TestWowheadRateUsesRealTimer(t *testing.T) {
	var times []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		times = append(times, time.Now())
		fmt.Fprint(rw, `{"name":"Test Item"}`)
	}))
	defer server.Close()
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	w.refresh(wowheadHistory(900001, 900002), nil)
	waitWowhead(t, w)
	if len(times) != 2 || times[1].Sub(times[0]) < 950*time.Millisecond {
		t.Fatalf("one request per second, got %v", times)
	}
}

func TestWowheadTTLAndNegativeCache(t *testing.T) {
	for _, status := range []int{200, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				rw.WriteHeader(status)
				if status == 200 {
					fmt.Fprint(rw, `{"name":"Test Item","quality":2}`)
				}
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "cache.json")
			w := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
			defer w.Close()
			c := fakeWowheadTime(w)
			db := wowheadHistory(900001)
			w.refresh(db, nil)
			waitWowhead(t, w)
			ttl := 30 * 24 * time.Hour
			if status == 404 {
				ttl = 24 * time.Hour
			}
			c.advance(ttl - time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 1 {
				t.Fatalf("fresh entry re-requested: %d", calls.Load())
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if status == 404 {
				var saved map[string]map[string]any
				if err := json.Unmarshal(data, &saved); err != nil {
					t.Fatal(err)
				}
				if len(saved["900001"]) != 2 || saved["900001"]["missing"] != true || saved["900001"]["fetched_at"] == nil {
					t.Fatalf("negative marker must contain no response data: %s", data)
				}
				reopened := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
				reopened.refresh(db, nil)
				waitWowhead(t, reopened)
				reopened.Close()
				if calls.Load() != 1 {
					t.Fatal("negative cache lost at restart")
				}
			}
			c.advance(time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 2 {
				t.Fatalf("expired entry not refreshed: %d", calls.Load())
			}
		})
	}
}

func TestWowheadBackoff(t *testing.T) {
	for _, tt := range []struct {
		status int
		header string
		delay  time.Duration
	}{
		{429, "120", 120 * time.Second}, {500, "", time.Minute}, {503, "date", 3 * time.Minute}, {502, "invalid", time.Minute}, {429, "0", time.Minute},
	} {
		t.Run(fmt.Sprintf("%d/%s", tt.status, tt.header), func(t *testing.T) {
			var calls atomic.Int32
			var c *wowheadClock
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				header := tt.header
				if header == "date" {
					header = c.now().Add(tt.delay).UTC().Format(http.TimeFormat)
				}
				rw.Header().Set("Retry-After", header)
				rw.WriteHeader(tt.status)
			}))
			defer server.Close()
			w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
			defer w.Close()
			c = fakeWowheadTime(w)
			db := wowheadHistory(900001, 900002)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 1 {
				t.Fatal("must stop the entire batch on server pressure")
			}
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 1 {
				t.Fatal("refresh must not bypass backoff")
			}
			c.advance(tt.delay - time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 1 {
				t.Fatal("Retry-After/backoff ended early")
			}
			c.advance(time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 2 {
				t.Fatal("backoff should allow a later refresh")
			}
			c.advance(time.Minute)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 2 {
				t.Fatal("repeated failures need exponential backoff")
			}
		})
	}
}

func TestWowheadMalformedAndOfflineLeaveCacheAlone(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"broken", `{"name":`, 200}, {"trailing", `{"name":"Test Item"} {}`, 200},
		{"quality", `{"name":"Test Item","quality":9}`, 200}, {"wrong type", `{"name":12}`, 200},
		{"placeholder", `{"name":"Item 900001"}`, 200}, {"large", `{"name":"Test Item","tooltip":"` + strings.Repeat("x", (256<<10)) + `"}`, 200},
		{"offline", "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(rw, tt.body) }))
			if tt.status == 0 {
				server.Close()
			} else {
				defer server.Close()
			}
			path := filepath.Join(t.TempDir(), "cache.json")
			original := fmt.Sprintf(`{"900002":{"name":"Test Saved Item","quality":2,"fetched_at":%d}}`, time.Now().Unix())
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			w := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
			defer w.Close()
			fakeWowheadTime(w)
			db := wowheadHistory(900001, 900002)
			w.refresh(db, nil)
			waitWowhead(t, w)
			got := w.refresh(db, nil)
			waitWowhead(t, w)
			if got[900001].Name != "" || got[900002].Name != "Test Saved Item" {
				t.Fatalf("invalid/offline response changed names: %+v", got)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.status == 0 {
				if string(after) != original {
					t.Fatal("offline failure changed the cache")
				}
			} else {
				// Invalid 200s now add a negative marker, without altering any
				// previously cached names, qualities or fetch timestamps.
				var before, saved map[string]map[string]any
				if json.Unmarshal([]byte(original), &before) != nil || json.Unmarshal(after, &saved) != nil {
					t.Fatal("invalid cache JSON")
				}
				marker := saved["900001"]
				if len(marker) != 2 || marker["missing"] != true || marker["fetched_at"] == nil {
					t.Fatal("invalid 200 must persist only a negative marker")
				}
				delete(saved, "900001")
				if !reflect.DeepEqual(saved, before) {
					t.Fatal("invalid response changed existing cache entries")
				}
			}
			if calls.Load() > 1 {
				t.Fatal("negative caching or offline backoff must prevent repeated requests")
			}
		})
	}
}

func TestWowheadCorruptCacheRebuilt(t *testing.T) {
	for _, bad := range []string{`{`, `null`, fmt.Sprintf(`{"900001":{"name":"Test Bad","quality":99,"fetched_at":%d}}`, time.Now().Unix()),
		fmt.Sprintf(`{"900001":{"name":"Test Future Item","fetched_at":%d}}`, time.Now().Add(time.Hour).Unix()),
		strings.Repeat(" ", 4<<20) + `{}`} {
		path := filepath.Join(t.TempDir(), "cache.json")
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			fmt.Fprint(rw, `{"name":"Test Rebuilt Item","quality":3}`)
		}))
		w := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
		before := w.refresh(wowheadHistory(900001), nil)
		waitWowhead(t, w)
		if before[900001].Name != "" {
			t.Fatalf("invalid cache exposed a name: %+v", before)
		}
		after := w.refresh(wowheadHistory(900001), nil)
		if after[900001].Name != "Test Rebuilt Item" || calls.Load() != 1 {
			t.Fatalf("corrupt cache not rebuilt: %+v", after)
		}
		w.Close()
		server.Close()
	}
}

func TestWowheadTimeoutAndNoRedirects(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	for _, redirect := range []bool{true, false} {
		t.Run(fmt.Sprint(redirect), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if redirect {
					http.Redirect(rw, r, target.URL, http.StatusFound)
				} else {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			client := server.Client()
			client.Timeout = 20 * time.Millisecond
			w := newWowheadNames(context.Background(), "", server.URL+"/", client)
			defer w.Close()
			w.refresh(wowheadHistory(900001), nil)
			waitWowhead(t, w)
			if forwarded.Load() != 0 {
				t.Fatal("redirect must not send IDs to another endpoint")
			}
		})
	}
}

func TestWowheadCacheEntryBound(t *testing.T) {
	saved := map[int]wowheadNameEntry{}
	for i := 1; i <= 10001; i++ {
		saved[i] = wowheadNameEntry{Name: "Test Item", FetchedAt: time.Now().Unix()}
	}
	data, _ := json.Marshal(saved)
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	w := newWowheadNames(context.Background(), path, "http://unused.invalid/", &http.Client{})
	defer w.Close()
	if len(w.entries) != 0 {
		t.Fatal("oversized cache must be ignored")
	}
}

func TestWowheadPreferenceNativeRoundTrip(t *testing.T) {
	accountConfigEnv(t)
	a := &App{}
	b := newNativeBridge(a, true, nativeServices{})
	defer b.Close()
	read := func(b *nativeBridge) bool {
		t.Helper()
		v, err := b.Call("reportRefresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(v)
		var values map[string]any
		_ = json.Unmarshal(raw, &values)
		enabled, ok := values["WowheadNames"].(bool)
		if !ok {
			t.Fatal("report must expose persisted Wowhead preference")
		}
		return enabled
	}
	if !read(b) {
		t.Fatal("lookup defaults on")
	}
	if err := saveGame("Test Game"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("setWowheadNames", nativeArgs("false")); err != nil {
		t.Fatal(err)
	}
	if read(b) {
		t.Fatal("off was not persisted")
	}
	other := newNativeBridge(&App{}, true, nativeServices{})
	defer other.Close()
	if read(other) {
		t.Fatal("restart lost off")
	}
	if savedGame() != "Test Game" {
		t.Fatal("toggle clobbered existing settings")
	}
	if _, err := b.Call("setWowheadNames", nativeArgs("true")); err != nil {
		t.Fatal(err)
	}
	if !read(other) {
		t.Fatal("on was not persisted")
	}
	for _, arg := range []string{"", "1", "TRUE", "null"} {
		if _, err := b.Call("setWowheadNames", nativeArgs(arg)); err == nil {
			t.Fatalf("accepted nonboolean %q", arg)
		}
	}
}

func TestWowheadCacheRetentionBound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { fmt.Fprint(rw, `{"name":"Test New Item","quality":1}`) }))
	defer server.Close()
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	for i := 1; i <= 10000; i++ {
		w.entries[i] = wowheadNameEntry{Name: "Test Prior Item", FetchedAt: time.Now().Unix()}
	}
	w.refresh(wowheadHistory(900001), nil)
	waitWowhead(t, w)
	if len(w.entries) != 10000 || w.entries[900001].Name != "Test New Item" {
		t.Fatalf("retain latest entry within 10000 limit: %d", len(w.entries))
	}
}

func TestWowheadAppAsyncAndLocalPriority(t *testing.T) {
	a, game := nameGame(t, "", "", "")
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			fmt.Fprint(rw, `{"name":"Test Online Item","quality":4}`)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	w := newWowheadNames(context.Background(), filepath.Join(t.TempDir(), "cache.json"), server.URL+"/", server.Client())
	defer w.Close()
	a.wowhead = w
	b := newNativeBridge(a, true, nativeServices{})
	defer b.Close()
	reportDone := make(chan error, 1)
	go func() { _, err := b.Call("reportRefresh", nil); reportDone <- err }()
	select {
	case err := <-reportDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh blocked on network")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("missing history ID never queued")
	}
	close(release)
	waitWowhead(t, w)
	a.refresh()
	assertFarmName(t, a, "Test Online Item")
	if a.snapshot().Farm.Hot.Houses[0].Rows[0].QualityClass != "q4" {
		t.Fatal("online quality not rendered")
	}
	path := filepath.Join(game, "WTF", "Account", "A#1", "SavedVariables", "ForeverCoach.lua")
	if err := os.WriteFile(path, []byte(`ForeverCoachDB={schema=1,itemNames={[900001]={n="Test Local Item",q=2}},chars={}}`), 0600); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	assertFarmName(t, a, "Test Local Item")
	if a.snapshot().Farm.Sell[0].QualityClass != "q2" {
		t.Fatal("online quality overrode local quality")
	}
}

func TestWowheadAppAllLocalSourcesFirst(t *testing.T) {
	for _, tt := range []struct{ name, save, coach, gear, bundle string }{
		{"save", `[900001]={"Test Local Item",0}`, "", "", ""},
		{"bundle", "", "", "", `{"Test Local Item",900001,1,2,3,1}`},
		{"coach", "", `[900001]={n="Test Local Item",q=2}`, "", ""},
		{"gear", "", "", `GQ.Data.foreverAudit = {[900001]={name="Test Local Item",quality=3}}`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, game := nameGame(t, tt.save, tt.coach, tt.gear)
			if tt.bundle != "" {
				nameAddonFile(t, game, "AHProfit", "FarmData.lua", "ns.farmData = {1,{"+tt.bundle+"},{},{}}")
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(rw, `{"name":"Test Online Item"}`)
			}))
			defer server.Close()
			w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
			defer w.Close()
			a.wowhead = w
			w.entries[900001] = wowheadNameEntry{Name: "Test Cached Item", FetchedAt: time.Now().Unix()}
			a.refresh()
			waitWowhead(t, w)
			assertFarmName(t, a, "Test Local Item")
			if calls.Load() != 0 {
				t.Fatal("requested a locally named ID")
			}
		})
	}
}

func TestWowheadStoredOffMakesNoRequests(t *testing.T) {
	a, _ := nameGame(t, "", "", "")
	if err := saveWowheadNames(false); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	// A fresh worker starts enabled; the bridge must apply the stored setting
	// before refreshing the eligible, still-unnamed history ID.
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	a.wowhead = w
	b := newNativeBridge(a, true, nativeServices{})
	defer b.Close()
	for i := 0; i < 2; i++ {
		result, err := b.Call("reportRefresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		waitWowhead(t, w)
		if result.(nativeReport).WowheadNames || calls.Load() != 0 {
			t.Fatalf("stored off must report off and make zero requests: requests=%d", calls.Load())
		}
	}
	// Positive control: the same fixture must really have lookup work.
	if _, err := b.Call("setWowheadNames", nativeArgs("true")); err != nil {
		t.Fatal(err)
	}
	waitWowhead(t, w)
	if calls.Load() != 1 {
		t.Fatalf("enabled control did not request the eligible ID: requests=%d", calls.Load())
	}
}

func TestWowheadToggleOffAndCancel(t *testing.T) {
	a, _ := nameGame(t, "", "", "")
	var calls atomic.Int32
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { calls.Add(1); close(started); <-r.Context().Done() }))
	defer server.Close()
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	a.wowhead = w
	b := newNativeBridge(a, true, nativeServices{})
	defer b.Close()
	if _, err := b.Call("setWowheadNames", nativeArgs("false")); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	waitWowhead(t, w)
	if calls.Load() != 0 {
		t.Fatal("off must make zero requests")
	}
	if _, err := b.Call("setWowheadNames", nativeArgs("true")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("on did not start missing lookup")
	}
	if _, err := b.Call("setWowheadNames", nativeArgs("false")); err != nil {
		t.Fatal(err)
	}
	waitWowhead(t, w)
	if len(w.entries) != 0 {
		t.Fatal("cancelled request changed cache")
	}
}

func TestWowheadHistoryEligibilityAndRecheck(t *testing.T) {
	var paths []string
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if len(paths) == 1 {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(rw, `{"name":"Test Item","quality":4}`)
	}))
	defer server.Close()
	w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
	defer w.Close()
	fakeWowheadTime(w)
	db := wowheadHistory(900001, 900002)
	db["bank"] = map[string]any{"Test": map[string]any{"900003": "not history"}}
	hist := M(db["hist"])
	hist["Second house"] = map[string]any{"900001": "duplicate", "0900004": "bad", "0": "bad", "10000001": "bad", "900005": false}
	w.refresh(db, nil)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not started")
	}
	// A refresh while the first request is held must neither duplicate the
	// worker nor request a queued ID which just acquired a local name.
	local := map[int]farmItemRef{900002: {Name: "Test Local Item"}}
	w.refresh(db, local)
	close(release)
	waitWowhead(t, w)
	if len(paths) != 1 || paths[0] != "/900001" {
		t.Fatalf("only still-unnamed canonical history IDs allowed: %v", paths)
	}
	out := w.refresh(db, local)
	waitWowhead(t, w)
	if out[900002].Name != "" || out[900001].Name != "Test Item" || len(paths) != 1 {
		t.Fatalf("local names/duplicates leaked: %+v %v", out, paths)
	}
}

func TestWowheadLocalQualityAndCachedSourcePriority(t *testing.T) {
	for _, q := range []int{0, 2} {
		a, _ := nameGame(t, "", fmt.Sprintf(`[900001]={q=%d}`, q), "")
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			fmt.Fprint(rw, `{"name":"Test Online Item","quality":4}`)
		}))
		w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
		a.wowhead = w
		a.refresh()
		waitWowhead(t, w)
		a.refresh()
		assertFarmName(t, a, "Test Online Item")
		if a.snapshot().Farm.Sell[0].QualityClass != fmt.Sprintf("q%d", q) {
			t.Fatal("online quality overrode local ID quality")
		}
		w.Close()
		server.Close()
	}
}

func TestWowheadPersistedByteBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	w := newWowheadNames(context.Background(), path, "", nil)
	defer w.Close()
	if w.client.Timeout != 10*time.Second {
		t.Fatal("default per-request deadline is not 10 seconds")
	}
	for id := 1; id <= 10000; id++ {
		w.entries[id] = wowheadNameEntry{Name: strings.Repeat("<", 128), FetchedAt: time.Now().Unix()}
	}
	w.save()
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() > 4<<20 {
		t.Fatalf("cache exceeds its read budget: %d", stat.Size())
	}
	reopened := newWowheadNames(context.Background(), path, "", nil)
	defer reopened.Close()
	if len(reopened.entries) != 10000 {
		t.Fatal("valid bounded names must survive reopen")
	}
}

func TestFarmWowheadLinks(t *testing.T) {
	a, _ := nameGame(t, "", `[900001]={n="Test Item",q=3}`, "")
	html := assertFarmName(t, a, "Test Item")
	if strings.Count(html, `href="https://www.wowhead.com/forever/item=900001"`) != 2 {
		t.Fatal("hot and sell names must link to the Forever item page")
	}
	if err := validateExternalURL("https://www.wowhead.com/forever/item=900001"); err != nil {
		t.Fatal(err)
	}
}
