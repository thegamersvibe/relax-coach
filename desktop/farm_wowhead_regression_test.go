package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWowheadBatchPersistsOnce(t *testing.T) {
	for _, tt := range []struct {
		name        string
		requests    int
		wantEntries int
	}{
		{"complete", wowheadMaxPerRefresh, wowheadMaxPerRefresh},
		{"backpressure", 3, 2},
		{"cancel", 3, 2},
		{"unchanged", 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache.json")
			// Observe each atomic replacement between requests and at completion.
			// Keep old files open so their inode cannot be recycled during the test.
			var versions []*os.File
			var last os.FileInfo
			var mu sync.Mutex
			observe := func() {
				mu.Lock()
				defer mu.Unlock()
				f, err := os.Open(path)
				if os.IsNotExist(err) {
					return
				}
				if err != nil {
					t.Error(err)
					return
				}
				st, err := f.Stat()
				if err != nil {
					f.Close()
					t.Error(err)
					return
				}
				if last != nil && os.SameFile(last, st) {
					f.Close()
					return
				}
				versions = append(versions, f)
				last = st
			}
			defer func() {
				for _, f := range versions {
					f.Close()
				}
			}()
			var calls atomic.Int32
			cancelReady := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				observe()
				n := int(calls.Add(1))
				if n > tt.wantEntries {
					if tt.name == "cancel" {
						close(cancelReady)
						<-r.Context().Done()
						return
					}
					rw.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				switch n % 3 {
				case 0:
					fmt.Fprint(rw, `{"name":false}`)
				case 1:
					fmt.Fprint(rw, `{"name":"Synthetic response","quality":2}`)
				case 2:
					rw.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			w := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
			defer w.Close()
			fakeWowheadTime(w)
			ids := make([]int, tt.requests)
			for i := range ids {
				ids[i] = 900001 + i
			}
			db := wowheadHistory(ids...)
			w.refresh(db, nil)
			if tt.name == "cancel" {
				select {
				case <-cancelReady:
				case <-time.After(3 * time.Second):
					t.Fatal("cancel fixture never reached the held request")
				}
				w.setEnabled(false)
			}
			waitWowhead(t, w)
			observe()
			wantWrites := 0
			if tt.wantEntries > 0 {
				wantWrites = 1
				data, err := os.ReadFile(path)
				var saved map[int]wowheadNameEntry
				if err != nil || json.Unmarshal(data, &saved) != nil || len(saved) != tt.wantEntries {
					t.Fatalf("batch cache incomplete: entries=%d want=%d err=%v", len(saved), tt.wantEntries, err)
				}
				for i := 0; i < tt.wantEntries; i++ {
					e := saved[ids[i]]
					if e.FetchedAt == 0 || e.Missing != ((i+1)%3 != 1) {
						t.Fatalf("batch lost result for ID %d", ids[i])
					}
				}
			}
			if calls.Load() != int32(tt.requests) || len(versions) != wantWrites {
				t.Fatalf("cache replacements=%d want=%d; requests=%d want=%d", len(versions), wantWrites, calls.Load(), tt.requests)
			}
			w.refresh(db, nil)
			waitWowhead(t, w)
			observe()
			if len(versions) != wantWrites || calls.Load() != int32(tt.requests) {
				t.Fatal("unchanged refresh must not write or request again")
			}
		})
	}
}

func TestWowheadRetryAfterCapped(t *testing.T) {
	for _, tt := range []struct {
		name, header string
		want         time.Duration
	}{
		{"large seconds", "86400", 30 * time.Minute},
		{"duration overflow", "9223372036854775807", 30 * time.Minute},
		{"signed overflow", "9223372036854775808", 30 * time.Minute},
		{"unsigned overflow", "18446744073709551616", 30 * time.Minute},
		{"huge decimal", strings.Repeat("9", 100), 30 * time.Minute},
		{"future date", "Fri, 31 Dec 9999 23:59:59 GMT", 30 * time.Minute},
		{"at cap", "1800", 30 * time.Minute},
		{"zero-padded seconds", strings.Repeat("0", 100) + "120", 2 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				rw.Header().Set("Retry-After", tt.header)
				rw.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()
			w := newWowheadNames(context.Background(), "", server.URL+"/", server.Client())
			defer w.Close()
			c := fakeWowheadTime(w)
			db := wowheadHistory(900001, 900002)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if got := w.retryAt.Sub(c.now()); got != tt.want {
				t.Errorf("Retry-After delay=%v, want %v", got, tt.want)
			}
			if calls.Load() != 1 {
				t.Fatal("server pressure must stop the batch")
			}
			c.advance(tt.want - time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 1 {
				t.Error("Retry-After ended before the bounded delay")
			}
			c.advance(time.Second)
			w.refresh(db, nil)
			waitWowhead(t, w)
			if calls.Load() != 2 {
				t.Error("bounded Retry-After did not admit a refresh at its deadline")
			}
		})
	}
}

func TestWowheadInvalidResponseDoesNotStarveBatch(t *testing.T) {
	for _, tt := range []struct{ name, body string }{
		{"long bytes", `{"name":"` + strings.Repeat("é", 65) + `"}`},
		{"pipe", `{"name":"Synthetic | response"}`},
		{"control", `{"name":"Synthetic\u0000response"}`},
		{"non-printable", `{"name":"Synthetic\u200eresponse"}`},
		{"name number", `{"name":12}`},
		{"name object", `{"name":{}}`},
		{"name null", `{"name":null}`},
		{"quality string", `{"name":"Synthetic response","quality":"2"}`},
		{"quality fraction", `{"name":"Synthetic response","quality":2.5}`},
		{"quality out of range", `{"name":"Synthetic response","quality":9}`},
		{"array", `[]`},
		{"broken", `{"name":`},
		{"trailing", `{"name":"Synthetic response"} {}`},
		{"placeholder", `{"name":"Item 900001"}`},
		{"oversized", `{"name":"Synthetic response","tooltip":"` + strings.Repeat("x", wowheadMaxResponseBytes) + `"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var badCalls, goodCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/900001" {
					badCalls.Add(1)
					fmt.Fprint(rw, tt.body)
					return
				}
				goodCalls.Add(1)
				fmt.Fprint(rw, `{"name":"Synthetic response","quality":2}`)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "cache.json")
			w := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
			defer w.Close()
			c := fakeWowheadTime(w)
			fetched := c.now()
			ids := []int{900001, 900002, 900003}
			w.refresh(wowheadHistory(ids...), nil)
			waitWowhead(t, w)
			if goodCalls.Load() != 2 || badCalls.Load() != 1 {
				t.Fatalf("invalid 200 blocked the next IDs: bad=%d good=%d", badCalls.Load(), goodCalls.Load())
			}
			// Keep adding eligible work. A permanently invalid low ID must not
			// monopolize later refreshes, even after the ordinary backoff window.
			const rounds = 40
			for i := 0; i < rounds; i++ {
				c.advance(time.Minute)
				ids = append(ids, 900004+i)
				got := w.refresh(wowheadHistory(ids...), nil)
				waitWowhead(t, w)
				if got[900001].Name != "" || got[900002].Name != "Synthetic response" || badCalls.Load() != 1 || goodCalls.Load() != int32(3+i) {
					t.Fatalf("refresh %d: invalid ID retried or later IDs starved (bad=%d good=%d)", i, badCalls.Load(), goodCalls.Load())
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var saved map[string]map[string]json.RawMessage
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			marker := saved["900001"]
			if len(marker) != 2 || string(marker["missing"]) != "true" || string(marker["fetched_at"]) != fmt.Sprint(fetched.Unix()) {
				t.Fatal("invalid response must persist only a negative marker and original fetch time")
			}
			reopened := newWowheadNames(context.Background(), path, server.URL+"/", server.Client())
			reopened.now, reopened.wait = c.now, c.wait
			defer reopened.Close()
			c.advance(wowheadNegativeTTL - c.now().Sub(fetched) - time.Second)
			reopened.refresh(wowheadHistory(900001), nil)
			waitWowhead(t, reopened)
			if badCalls.Load() != 1 || goodCalls.Load() != 2+rounds {
				t.Fatal("negative cache must survive restart for the full 24 hours")
			}
			c.advance(time.Second)
			reopened.refresh(wowheadHistory(900001), nil)
			waitWowhead(t, reopened)
			if badCalls.Load() != 2 || goodCalls.Load() != 2+rounds {
				t.Fatal("only the invalid response must be retried at the 24-hour boundary")
			}
		})
	}
}
