package main

import "testing"

func TestNativeWowheadOpenerExactShape(t *testing.T) {
	var opened []string
	b := newNativeBridge(&App{}, false, nativeServices{open: func(raw string) error {
		opened = append(opened, raw)
		return nil
	}})
	defer b.Close()
	for _, raw := range []string{
		"https://www.wowhead.com/forever/item=900001",
		"https://github.com/" + Repo + "/releases",
		"https://www.icy-veins.com/wow-classic/warrior-leveling-guide",
	} {
		before := len(opened)
		if _, err := b.Call("openExternal", nativeArgs(raw)); err != nil || len(opened) != before+1 || opened[before] != raw {
			t.Fatalf("valid link did not reach browser service unchanged: %q (%v)", raw, err)
		}
	}
	for _, raw := range []string{
		"https://www.wowhead.com/forever/item=",
		"https://www.wowhead.com/forever/item=abc",
		"https://www.wowhead.com/forever/item=-1",
		"https://www.wowhead.com/forever/item=+1",
		"https://www.wowhead.com/forever/item=９００００１",
		"https://www.wowhead.com/forever/item=%39%30%30%30%30%31",
		"https://www.wowhead.com/forever/item=900001/",
		"https://www.wowhead.com/forever/item=900001/other",
		"https://www.wowhead.com/forever/item=900001%2Fother",
		"https://www.wowhead.com/forever/item=900001?redirect=bad",
		"https://www.wowhead.com/forever/item=900001?",
		"https://www.wowhead.com/forever/item=900001#tooltip",
		"https://www.wowhead.com/forever/item=900001#",
		"https://www.wowhead.com/forever/item=900001\n",
		"https://www.wowhead.com/forever/spell=900001",
		"https://www.wowhead.com/item=900001",
		"https://www.wowhead.com/classic/item=900001",
		"https://www.wowhead.com/classic/guide/classes/warrior/leveling",
		"https://www.wowhead.com/forever/./item=900001",
		"https://www.wowhead.com/forever/../item=900001",
		"https://www.wowhead.com/%66orever/item=900001",
		"http://www.wowhead.com/forever/item=900001",
		"https://wowhead.com/forever/item=900001",
		"https://nether.wowhead.com/forever/item=900001",
		"https://www.wowhead.com:443/forever/item=900001",
		"https://www.wowhead.com.evil.test/forever/item=900001",
		"https://www.wowhead.com@evil.test/forever/item=900001",
		"https://evil.test/forever/item=900001",
		"javascript:alert(1)",
	} {
		t.Run(raw, func(t *testing.T) {
			before := len(opened)
			if _, err := b.Call("openExternal", nativeArgs(raw)); err == nil || len(opened) != before {
				t.Fatalf("non-item URL reached browser service: %q (err=%v)", raw, err)
			}
		})
	}
}
