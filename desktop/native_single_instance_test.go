package main

import (
	"errors"
	"testing"
	"time"
)

func TestNativeSingleInstance(t *testing.T) {
	for _, c := range []struct {
		name     string
		mode     string
		ok       bool
		err      error
		run      bool
		focused  bool
		wantWait time.Duration
	}{
		{"first copy", "", true, nil, true, false, nativeInstanceWait},
		{"second copy focuses the first and stops", "", false, nil, false, true, nativeInstanceWait},
		{"lock error: run anyway", "", false, errors.New("no lock"), true, false, nativeInstanceWait},
		{"new app of an app update always runs", "start", false, nil, true, false, 0},
		{"new app of an app update takes a free lock", "start", true, nil, true, false, 0},
	} {
		var waited time.Duration = -1
		focused := false
		run := nativeSingleInstance(c.mode, func(w time.Duration) (bool, error) { waited = w; return c.ok, c.err }, func() { focused = true })
		if run != c.run || focused != c.focused || waited != c.wantWait {
			t.Errorf("%s: run %v focused %v waited %v", c.name, run, focused, waited)
		}
	}
	if nativeInstanceWait < 2*time.Second {
		t.Fatal("a copy that is just closing (Open the installed Relax Coach) needs time to let go")
	}
}
