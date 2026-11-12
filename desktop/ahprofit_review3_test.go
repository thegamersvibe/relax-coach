package main

// Final review of the Farming page (S7 and notes), from the reviewer's probe.

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func snippet(html, marker string, n int) string {
	i := strings.Index(html, marker)
	if i < 0 {
		return "(no " + marker + ")"
	}
	return html[i:min(len(html), i+n)]
}

func review3Render(t *testing.T, sv string, now time.Time) (FarmView, string) {
	t.Helper()
	elf := []CharView{{Name: "Elf", Realm: "R", Faction: "Alliance", Updated: now}}
	fv, err := buildFarm([]byte(sv), elf, now, farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil {
		t.Fatal(err)
	}
	return fv, html
}

// S7: a bid-only auction (buyout 0) has unknown proceeds: counted apart,
// never as "0c after the cut".
func TestFarmBidOnlyListings(t *testing.T) {
	now := time.Unix(farmNow, 0)
	sv := fmt.Sprintf(`AHProfitDB={v=2,auc={own={["Elf-R"]={t=%d,list={{7,2,0,0},{8,1,0,0},{9,1,100,0}}}}}}`, farmNow)
	_, html := review3Render(t, sv, now)
	if !strings.Contains(html, "1 listed, 95c after the cut if all sell · 2 bid only, not counted") {
		t.Fatalf("bid-only counted apart: %s", snippet(html, "farm-listings", 300))
	}
	sv = fmt.Sprintf(`AHProfitDB={v=2,auc={own={["Elf-R"]={t=%d,list={{7,2,0,0},{8,1,0,0}}}}}}`, farmNow)
	_, html = review3Render(t, sv, now)
	if strings.Contains(html, "0c after the cut") || !strings.Contains(html, "2 bid only, not counted") {
		t.Fatalf("only bid-only: no 0c total: %s", snippet(html, "farm-listings", 300))
	}
}

// A total at the overflow cap is shown as a lower bound.
func TestFarmCappedTotalsSayMoreThan(t *testing.T) {
	now := time.Unix(farmNow, 0)
	sv := fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]={p={},q={},t={},full=0}},shared={["R|Alliance"]={p={[7]=1000000000000},q={[7]=1},t={[7]=%d},full=0,from={[%d]="P"}}},bank={["Elf-R"]={t=1,items={{7,2000,0}}}},auc={own={["Elf-R"]={t=%d,list={{7,1000000,1000000000000,0},{7,1000000,1000000000000,0}}}}}}`, farmNow, farmNow, farmNow)
	_, html := review3Render(t, sv, now)
	if strings.Count(html, "more than 100,000,000,000g 0s 0c") != 2 {
		t.Fatalf("capped row value and listing total say more than: %d", strings.Count(html, "more than 100,000,000,000g"))
	}
}

// A price without a listed count does not claim "0 listed".
func TestFarmUnknownListedCountOmitted(t *testing.T) {
	now := time.Unix(farmNow, 0)
	sv := fmt.Sprintf(`AHProfitDB={v=2,live={["R|Alliance"]={p={[7]=500},q={},t={[7]=%d},full=0}},bank={["Elf-R"]={t=1,items={{7,2,0}}}}}`, farmNow)
	_, html := review3Render(t, sv, now)
	if strings.Contains(html, "0 listed") || !strings.Contains(html, "your scan, ") {
		t.Fatalf("no listed count: %s", snippet(html, "your scan", 80))
	}
}

// The page's clock is whole seconds, like the game's GetServerTime.
func TestFarmClockWholeSeconds(t *testing.T) {
	now := time.Unix(farmNow, 500000000) // half a second past
	sv := fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]={p={[7]=111},q={[7]=1},t={[7]=%d},full=0}},shared={["R|Alliance"]={p={[7]=222},q={[7]=1},t={[7]=%d},full=0,from={[%d]="P"}}},bank={["Elf-R"]={t=1,items={{7,1,0}}}}}`, farmNow-7200, farmNow-100, farmNow-100)
	fv, _ := review3Render(t, sv, now)
	if r := farmRow(fv, "Elf-R", 7); r.Price != 111 || r.Source != "scan" {
		t.Fatalf("own scan exactly 2 h old still wins at T+0.5 s: %+v", r)
	}
	sv = fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]={p={},q={},t={},full=0}},shared={["R|Alliance"]={p={[7]=222},q={[7]=1},t={[7]=%d},full=0,from={[%d]="P"}}},bank={["Elf-R"]={t=1,items={{7,1,0}}}}}`, farmNow-43200, farmNow-43200)
	fv, _ = review3Render(t, sv, now)
	if r := farmRow(fv, "Elf-R", 7); r.Price != 222 {
		t.Fatalf("guild price exactly 12 h old still kept at T+0.5 s: %+v", r)
	}
}

// Guild prices only from a save version that has them (1, 2 or 3).
func TestFarmGuildPricesNeedAKnownVersion(t *testing.T) {
	now := time.Unix(farmNow, 0)
	for v, want := range map[string]int{"": 0, "v=0,": 0, "v=1,": 222, "v=2,": 222, "v=3,": 222, "v=4,": 0, `v="2",`: 0} {
		sv := fmt.Sprintf(`AHProfitDB={%slive={["R|Alliance"]={p={},q={},t={},full=0}},shared={["R|Alliance"]={p={[7]=222},q={[7]=1},t={[7]=%d},full=0,from={}}},bank={["Elf-R"]={t=1,items={{7,1,0}}}}}`, v, farmNow-60)
		fv, _ := review3Render(t, sv, now)
		if r := farmRow(fv, "Elf-R", 7); r.Price != want {
			t.Fatalf("%q: guild price %d, want %d", v, r.Price, want)
		}
	}
}
