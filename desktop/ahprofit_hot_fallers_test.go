package main

import (
	"reflect"
	"strings"
	"testing"
)

func hotFallIDs(hh FarmHotHouse) []int {
	var ids []int
	for _, r := range hh.Fallers {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestHotFallersFilters(t *testing.T) {
	for _, c := range []struct {
		name       string
		keep, drop []ahpHistRow
	}{
		{"listed-now-10", hotWithSupply(800, 10, 20, 20, 20, 20, 20), hotWithSupply(800, 9, 20, 20, 20, 20, 20)},
		{"previous-drop-20", series(hotToday, 20, 2000, 2000, 2000, 2000, 1000, 800), series(hotToday, 20, 2000, 2000, 2000, 2000, 1000, 810)},
		{"usual-drop-20", series(hotToday, 20, 1000, 1000, 1000, 1000, 2000, 800), series(hotToday, 20, 1000, 1000, 1000, 1000, 2000, 810)},
		{"usual-5-silver", series(hotToday, 20, 500, 500, 500, 500, 500, 400), series(hotToday, 20, 499, 499, 499, 499, 499, 390)},
		{"usual-listed-5", hotWithSupply(800, 10, 5, 5, 5, 5, 5), hotWithSupply(800, 10, 4, 4, 4, 4, 4)},
		{"earlier-days-5", series(hotToday, 20, 1000, 1000, 1000, 1000, 1000, 800), series(hotToday, 20, 1000, 1000, 1000, 1000, 800)},
	} {
		t.Run(c.name, func(t *testing.T) {
			hh := hotHouse(t, hotSave(map[int][]ahpHistRow{1: c.keep, 2: c.drop}))
			if got := hotFallIDs(hh); !reflect.DeepEqual(got, []int{1}) || hh.Checked != 2 || hh.FallDropped != 1 {
				t.Fatalf("keep boundary, reject below it: ids %v checked %d filtered %d", got, hh.Checked, hh.FallDropped)
			}
		})
	}
}

func TestHotFallersAllowThinSupply(t *testing.T) {
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{
		1: hotWithSupply(800, 49, 100, 100, 100, 100, 100),
		2: hotWithSupply(800, 50, 100, 100, 100, 100, 100),
	}))
	if got := hotFallIDs(hh); !reflect.DeepEqual(got, []int{1, 2}) || hh.ThinSupply != 0 || hh.FallDropped != 0 {
		t.Fatalf("fallers ignore the riser-only thin filter: %+v", hh)
	}
}

func TestHotFallersRankSmallerDrop(t *testing.T) {
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{
		1: series(hotToday, 20, 1000, 1000, 1000, 1000, 500, 400),  // drops 20/60
		2: series(hotToday, 20, 1000, 1000, 1000, 1000, 1250, 500), // drops 60/50
		3: series(hotToday, 20, 1000, 1000, 1000, 1000, 3000, 600), // drops 80/40
		4: series(hotToday, 20, 1000, 1000, 1000, 1000, 3000, 600), // tie: lower id first
	}))
	if got := hotFallIDs(hh); !reflect.DeepEqual(got, []int{2, 3, 4, 1}) {
		t.Fatalf("smaller drop, descending; ascending id tie: %v", got)
	}
	if r := hh.Fallers[0]; r.ChangePct != -60 || r.AgainstPct != -50 {
		t.Fatalf("signed changes round symmetrically: %+v", r)
	}
}

func TestHotFallersDumpBoundary(t *testing.T) {
	for _, earlier := range [][]int{{100, 100, 100, 100, 100}, {100}} {
		hh := hotHouse(t, hotSave(map[int][]ahpHistRow{
			1: hotWithSupply(800, 199, earlier...),
			2: hotWithSupply(800, 200, earlier...),
		}))
		if len(hh.Fallers) != 2 {
			t.Fatalf("expected both fallers, got %v", hotFallIDs(hh))
		}
		if hh.Fallers[0].Dump || !hh.Fallers[1].Dump || hh.Fallers[1].Spike {
			t.Fatalf("199%% is not a dump; 200%% is, also with only earlier available days: %+v", hh.Fallers)
		}
	}
}

// The decoder also rejects zero; exercise ranking directly so its own guard
// cannot be masked by that independent boundary. A 1c fall has no 10x cap.
func TestHotFallersMinCopper(t *testing.T) {
	earlier := series(hotToday-1, 20, 1000, 1000, 1000, 1000, 1000)
	for _, price := range []int{-1, 0, 1} {
		it := hotItem{now: price, listed: 20, nowDay: hotToday, earlier: earlier, prev: earlier[len(earlier)-1]}
		row, excluded := hotRank(it, false, true)
		if got, want := excluded == hotKept, price >= 1; got != want {
			t.Errorf("Now %d: kept %t, want %t; row %+v", price, got, want, row)
		}
	}
	hh := hotHouse(t, hotSave(map[int][]ahpHistRow{1: steady(1)}))
	if !reflect.DeepEqual(hotFallIDs(hh), []int{1}) {
		t.Error("1c accepted through the real decoder; no reciprocal 10x cap")
	}
}

func TestHotFallersCapNotPadded(t *testing.T) {
	hist := map[int][]ahpHistRow{}
	for i := 1; i <= 12; i++ {
		hist[i] = steady(810 - 10*i)
	}
	if got := hotFallIDs(hotHouse(t, hotSave(hist))); !reflect.DeepEqual(got, []int{12, 11, 10, 9, 8, 7, 6, 5, 4, 3}) {
		t.Errorf("top ten only: %v", got)
	}
	if got := hotFallIDs(hotHouse(t, hotSave(map[int][]ahpHistRow{1: steady(800)}))); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("one qualifier, not padded: %v", got)
	}
}

func TestHotFallersChangeOnly(t *testing.T) {
	db := hotSave(map[int][]ahpHistRow{
		1: series(hotToday, 20, 500, 500, 2000, 1000), // down 50%, ABOVE usual: change-only keeps
		2: series(hotToday, 20, 1000, 800),
		3: series(hotToday, 20, 1000, 810),              // under 20% daily
		4: series(hotToday, 20, 499, 100),               // under 5s usual
		5: series(hotToday, 20, 500),                    // no Previous; no supply inference
		6: series(hotToday, 20, 500, 500, 20000, 10000), // down 50%, 20x usual: no 10x cap
	})
	hh := hotHouse(t, db)
	if !hh.ChangeOnly || !reflect.DeepEqual(hotFallIDs(hh), []int{1, 6, 2}) || hh.Checked != 5 || hh.FallDropped != 2 {
		t.Fatalf("change-only ranks daily drops without against-usual guard: %+v", hh)
	}
	M(M(db["hist"])["Realm|Alliance"])["9"] = ahpHistEncode(steady(1000))
	if hh := hotHouse(t, db); hh.ChangeOnly || len(hh.Fallers) != 0 {
		t.Fatalf("one mature item enables the baseline rules for both lists: %+v", hh)
	}
}

func TestHotFallersHouseDayNamesAndLabels(t *testing.T) {
	for _, daysAgo := range []int{0, 1, 3} {
		db := hotSave(map[int][]ahpHistRow{
			1: series(hotToday-daysAgo, 20, 1000, 1000, 1000, 1000, 1000, 800),
			2: series(hotToday-daysAgo, 20, 1000, 1000, 1000, 1000, 1000, 700),
		})
		items := map[int]farmItemRef{1: {Name: "<b>Sample Down</b>", QualityClass: "q3"}}
		h := buildHot(db, 3, true, hotHouses, items, map[int]bool{1: true}, hotNow)
		hh := h.Houses[0]
		if len(hh.Fallers) != 2 || hh.DaysAgo != daysAgo || hh.Fallers[1].Name != "<b>Sample Down</b>" || hh.Fallers[1].QualityClass != "q3" || !hh.Fallers[1].Gatherable || hh.Fallers[0].Name != "item 2" || hh.Fallers[0].Chart.DotX != hotChartW-2 {
			t.Fatalf("shared chosen day, names, quality, gather and chart: %+v", hh)
		}
		html := hotHTML(t, h)
		for _, want := range []string{`id="farm-fallers"`, "Down", "&lt;b&gt;Sample Down&lt;/b&gt;", `class="q3"`, "-20%", ">20 / 20</td>"} {
			if !strings.Contains(html, want) {
				t.Errorf("page lacks %q", want)
			}
		}
		if strings.Contains(html, "<b>Sample Down</b>") {
			t.Error("name must stay escaped")
		}
		start := strings.Index(html, `id="farm-fallers"`)
		if start < 0 {
			t.Fatal("fallers card missing")
		}
		fallHTML := html[start:]
		label := "Down since yesterday"
		if daysAgo == 1 {
			label = "From yesterday's scan (" + hh.NowDay.Format("2 Jan") + "), against the day before"
		}
		if daysAgo > 1 {
			label = "From the scan of " + hh.NowDay.Format("2 Jan") + ", against the day before"
		}
		if !strings.Contains(fallHTML, label) {
			t.Errorf("fallers lack day label %q", label)
		}
	}
	h := buildHot(hotSave(map[int][]ahpHistRow{1: steady(1000)}), 3, true, hotHouses, nil, nil, hotNow)
	for _, want := range []string{"Nothing is clearly up.", "Nothing is clearly down."} {
		if !strings.Contains(hotHTML(t, h), want) {
			t.Errorf("empty lists lack %q", want)
		}
	}
}
