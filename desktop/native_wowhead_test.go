package main

import (
	"html"
	"strings"
	"testing"
)

func TestNativeWowheadTalentGuideLinks(t *testing.T) {
	var opened []string
	b := newNativeBridge(&App{}, false, nativeServices{open: func(raw string) error {
		opened = append(opened, raw)
		return nil
	}})
	defer b.Close()
	// Main allowed both Wowhead hosts, default HTTPS ports, escaped paths,
	// query strings and fragments for documentation. Keep those guide links.
	for _, raw := range []string{
		"https://www.wowhead.com/forever/guide/classes/warrior/arms/dps-leveling-tips",
		"https://www.wowhead.com/classic/guide/classes/warrior/leveling",
		"https://wowhead.com/guide/classes/warrior/leveling",
		"https://www.wowhead.com:443/forever/guide/classes/warrior/leveling",
		"https://wowhead.com:443/classic/guide/classes/warrior/leveling?view=talents&mode=full#build",
		"https://www.wowhead.com/forever/guide/classes/warrior/leveling?",
		"https://www.wowhead.com/forever/guide/classes/warrior/leveling#",
		"https://www.wowhead.com/%66orever/%67uide/classes/warrior/leveling",
	} {
		t.Run(raw, func(t *testing.T) {
			cv := CharView{Name: "Synthetic", Class: "WARRIOR", Level: 20}
			enrich(&cv, map[string]any{"talentPlan": map[string]any{
				"source": "Wowhead", "url": raw,
				"timeline": map[string]any{"1": map[string]any{"level": float64(20), "name": "Synthetic talent", "state": "next"}},
			}}, nil, nil, nil)
			body, err := renderReportBody(View{Chars: []CharView{cv}})
			if err != nil || cv.TalentURL != raw || !strings.Contains(html.UnescapeString(body), `href="`+raw+`"`) {
				t.Fatalf("talent guide did not survive enrich/render: %v", err)
			}
			before := len(opened)
			if _, err := b.Call("openExternal", nativeArgs(cv.TalentURL)); err != nil || len(opened) != before+1 || opened[before] != raw {
				t.Fatalf("rendered talent guide refused by browser service: %q (%v)", raw, err)
			}
		})
	}
}

func TestNativeWowheadOpenerExactShape(t *testing.T) {
	var opened []string
	b := newNativeBridge(&App{}, false, nativeServices{open: func(raw string) error {
		opened = append(opened, raw)
		return nil
	}})
	defer b.Close()
	for _, raw := range []string{
		"https://www.wowhead.com/forever/item=900001",
		"https://www.wowhead.com/classic/guide/classes/warrior/leveling",
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
		"https://www.wowhead.com/forever/item=900001/guide/talents",
		"http://www.wowhead.com/forever/guide/classes/warrior/leveling",
		"https://www.wowhead.com:444/forever/guide/classes/warrior/leveling",
		"https://nether.wowhead.com/forever/guide/classes/warrior/leveling",
		"https://www.wowhead.com.evil.test/forever/guide/classes/warrior/leveling",
		"https://user@www.wowhead.com/forever/guide/classes/warrior/leveling",
		"https://www.wowhead.com/forever/guide/../item=900001",
		"https://www.wowhead.com/forever/guide/%2e%2e/item=900001",
		"https://www.wowhead.com/forever/guide/./classes/warrior/leveling",
		"https://www.wowhead.com/forever/guide/classes/warrior/leveling\n",
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
