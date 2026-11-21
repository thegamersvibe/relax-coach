package main

import "testing"

// Every report reply (refresh, preference save, account and folder changes)
// carries a strictly increasing Seq assigned under the bridge lock.
func TestRaidLockReportSeq(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	accountSave(t, game, "B#2", "Beta")
	var last uint64
	calls := []struct{ method, arg string }{
		{"reportRefresh", ""}, {"setWowheadNames", "true"}, {"selectAccount", "B#2"},
		{"setGame", game}, {"reportRefresh", ""}, {"setWowheadNames", "false"},
	}
	for _, c := range calls {
		var got any
		var err error
		if c.arg == "" {
			got, err = b.Call(c.method, nil)
		} else {
			got, err = b.Call(c.method, nativeArgs(c.arg))
		}
		if err != nil {
			t.Fatalf("%s: %v", c.method, err)
		}
		r, ok := got.(nativeReport)
		if !ok || r.Seq <= last {
			t.Fatalf("%s: Seq %v not above %d", c.method, got, last)
		}
		last = r.Seq
	}
}
