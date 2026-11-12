package main

// Review round 1 of the Farming page (S1, S2 bounds, S3, S4, N1), from the
// reviewer's probes. These use only the API of the first Farming commits.

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func reviewHouse(n int, t int64) string {
	var p, q, tt strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&p, "[%d]=%d,", i, 10000+i)
		fmt.Fprintf(&q, "[%d]=%d,", i, 3)
		fmt.Fprintf(&tt, "[%d]=%d,", i, t)
	}
	return fmt.Sprintf("{p={%s},q={%s},t={%s},full=%d}", p.String(), q.String(), tt.String(), t)
}

// S1: without a Coach character of the same name and realm the house is
// unknown, even when only one house of that realm was ever scanned.
func TestFarmNoCoachMatchMeansNoHouse(t *testing.T) {
	now := time.Unix(farmNow, 0)
	sv := fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]={p={[7]=500},q={[7]=1},t={[7]=%d},full=1}},bank={["Orc-R"]={t=1,items={{7,2,0}}}}}`, farmNow)
	for _, chars := range [][]CharView{nil, {{Name: "Elf", Realm: "R", Faction: "Alliance", Updated: now}}} {
		fv, err := buildFarm([]byte(sv), chars, now, farmRef{})
		if err != nil {
			t.Fatal(err)
		}
		if len(fv.Sell) != 1 || fv.Sell[0].House != "" || !fv.Sell[0].NoPrice || fv.Sell[0].Price != 0 {
			t.Fatalf("chars %v: Orc-R must be unpriced with no house: %+v", chars, fv.Sell)
		}
	}
	// With the Coach character, the house is known and priced.
	fv, _ := buildFarm([]byte(sv), []CharView{{Name: "Orc", Realm: "R", Faction: "Alliance", Updated: now}}, now, farmRef{})
	if fv.Sell[0].House != "R|Alliance" || fv.Sell[0].Price != 500 {
		t.Fatalf("Coach match prices it: %+v", fv.Sell[0])
	}
}

// S2: the addon's bounds on received prices: older than 12 hours or more
// than 300 seconds ahead is ignored (nothing, not an old price).
func TestFarmGuildPriceBounds(t *testing.T) {
	now := time.Unix(farmNow, 0)
	elf := []CharView{{Name: "Elf", Realm: "R", Faction: "Alliance", Updated: now}}
	sv := func(ownT, g7, g8, g9 int64) string {
		return fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]={p={[7]=500},q={[7]=2},t={[7]=%d},full=0}},shared={["R|Alliance"]={p={[7]=1,[8]=9,[9]=4},q={[7]=1,[8]=1,[9]=1},t={[7]=%d,[8]=%d,[9]=%d},full=0,from={[%d]="Evil",[%d]="Old",[%d]="Ok"}}},bank={["Elf-R"]={t=1,items={{7,2,0},{8,1,0},{9,1,0}}}}}`,
			ownT, g7, g8, g9, g7, g8, g9)
	}
	// own 3 h old; guild item 7 claimed 10 years ahead, item 8 30 days old, item 9 at +300 s (the limit)
	fv, err := buildFarm([]byte(sv(farmNow-3*3600, farmNow+10*365*86400, farmNow-30*86400, farmNow+300)), elf, now, farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	if r := farmRow(fv, "Elf-R", 7); r.Price != 500 || r.Source != "scan" {
		t.Fatalf("a guild price far ahead is ignored, own scan shown: %+v", r)
	}
	if r := farmRow(fv, "Elf-R", 8); !r.NoPrice || r.Price != 0 {
		t.Fatalf("a guild price older than 12 h shows nothing: %+v", r)
	}
	if r := farmRow(fv, "Elf-R", 9); r.Price != 4 || r.Source != "guild" || r.From != "Ok" {
		t.Fatalf("a guild price at +300 s is still accepted: %+v", r)
	}
	// boundaries: 12 h exactly is kept, one second more is not; +301 s is not
	fv, _ = buildFarm([]byte(sv(farmNow-3*3600, farmNow+301, farmNow-43201, farmNow-43200)), elf, now, farmRef{})
	if r := farmRow(fv, "Elf-R", 7); r.Source != "scan" {
		t.Fatalf("+301 s ignored: %+v", r)
	}
	if r := farmRow(fv, "Elf-R", 8); !r.NoPrice {
		t.Fatalf("12 h + 1 s ignored: %+v", r)
	}
	if r := farmRow(fv, "Elf-R", 9); r.Source != "guild" {
		t.Fatalf("exactly 12 h kept: %+v", r)
	}
}

// S3: at most 150 rows per house, so one rich house cannot hide another.
func TestFarmRowCapPerHouse(t *testing.T) {
	now := time.Unix(farmNow, 0)
	var a, b strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&a, "{%d,1,0},", i)
	}
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, "{%d,1,0},", i)
	}
	sv := fmt.Sprintf(`AHProfitDB={v=1,live={["R|Alliance"]=%s,["R|Horde"]={p={[1]=5,[2]=5,[3]=5,[4]=5,[5]=5},q={},t={[1]=%d,[2]=%d,[3]=%d,[4]=%d,[5]=%d},full=1}},
 bank={["Rich-R"]={t=1,items={%s}},["Poor-R"]={t=1,items={%s}},["Lost-R"]={t=1,items={{1,1,0}}}}}`,
		reviewHouse(200, farmNow-10), farmNow, farmNow, farmNow, farmNow, farmNow, a.String(), b.String())
	chars := []CharView{{Name: "Rich", Realm: "R", Faction: "Alliance", Updated: now}, {Name: "Poor", Realm: "R", Faction: "Horde", Updated: now.Add(-time.Hour)}}
	fv, err := buildFarm([]byte(sv), chars, now, farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderFarmBody(View{Farm: fv})
	if err != nil {
		t.Fatal(err)
	}
	al, ho, un := strings.Count(html, `data-house="R|Alliance"`), strings.Count(html, `data-house="R|Horde"`), strings.Count(html, `data-house=""`)
	if al != 150 || ho != 5 || un != 1 {
		t.Fatalf("rendered rows: Alliance=%d Horde=%d unknown=%d", al, ho, un)
	}
	// the 150 kept are the most valuable of that house
	if r := farmRow(fv, "Rich-R", 200); r == nil {
		t.Fatal("the most valuable Alliance row is kept")
	}
	if r := farmRow(fv, "Rich-R", 1); r != nil {
		t.Fatal("the least valuable Alliance rows beyond 150 are dropped")
	}
}

// S4: a 60 MB file of tiny values fails fast instead of allocating GBs.
func TestFarmHugeSaveFailsFast(t *testing.T) {
	data := []byte("AHProfitDB={x={" + strings.Repeat("1,", 30<<20) + "}}")
	start := time.Now()
	_, err := buildFarm(data, nil, time.Unix(farmNow, 0), farmRef{})
	if err == nil {
		t.Fatal("a 60 MB save is refused")
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("refused only after %v", el)
	}
	// under the byte cap but with too many values
	data = []byte("AHProfitDB={x={" + strings.Repeat("1,", 600000) + "}}")
	start = time.Now()
	if _, err := buildFarm(data, nil, time.Unix(farmNow, 0), farmRef{}); err == nil || !strings.Contains(err.Error(), "values") {
		t.Fatalf("too many values refused: %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("value cap hit only after %v", el)
	}
	if ahpMaxBytes != 16<<20 {
		t.Fatalf("byte cap %d", ahpMaxBytes)
	}
	// over 16 MB but few values (a long comment): refused by size alone
	data = []byte("-- " + strings.Repeat("x", 16<<20) + "\nAHProfitDB={}")
	if _, err := buildFarm(data, nil, time.Unix(farmNow, 0), farmRef{}); err == nil || !strings.Contains(err.Error(), "16 MB") {
		t.Fatalf("over 16 MB refused by size: %v", err)
	}
}

// N1: listing sums cannot overflow into negative gold.
func TestFarmListingSumsClamped(t *testing.T) {
	var l strings.Builder
	for i := 0; i < 10; i++ {
		l.WriteString("{1,1000000,1000000000000,0},")
	}
	sv := `AHProfitDB={v=1,auc={own={["A-R"]={t=1,list={` + l.String() + `}}}},bank={["A-R"]={t=1,items={{1,1000000,0}}}},live={["R|Alliance"]={p={[1]=1000000000000},q={[1]=1},t={[1]=` + fmt.Sprint(farmNow) + `},full=0}}}`
	fv, err := buildFarm([]byte(sv), []CharView{{Name: "A", Realm: "R", Faction: "Alliance"}}, time.Unix(farmNow, 0), farmRef{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fv.Listings) != 1 || fv.Listings[0].ActiveAfterCut != ahpMaxCopper || fv.Listings[0].Active != 10 {
		t.Fatalf("listing total clamped, never negative: %+v", fv.Listings)
	}
	if r := farmRow(fv, "A-R", 1); r == nil || r.Value != ahpMaxCopper {
		t.Fatalf("row value clamped: %+v", r)
	}
}
