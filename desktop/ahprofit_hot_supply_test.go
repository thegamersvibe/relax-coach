package main

import (
	"reflect"
	"strings"
	"testing"
)

func hotWithSupply(nowPrice, nowListed int, earlierListed ...int) []ahpHistRow {
	var rows []ahpHistRow
	for i, listed := range earlierListed {
		rows = append(rows, ahpHistRow{Day: hotToday - len(earlierListed) + i, High: 1000, Listed: listed})
	}
	return append(rows, ahpHistRow{Day: hotToday, High: nowPrice, Listed: nowListed})
}

func hotHTML(t *testing.T, h FarmHot) string {
	t.Helper()
	html, err := renderFarmBody(View{Farm: FarmView{Present: true, Houses: hotHouses, Hot: h}})
	if err != nil {
		t.Fatal(err)
	}
	return html
}

// Counts are disjoint: old filters first, then thin supply among otherwise
// qualifying risers. Usual supply excludes Now and uses the same earlier days
// as the price median, including the few days available in change-only mode.
func TestHotThinSupply(t *testing.T) {
	for _, c := range []struct {
		name       string
		earlier    []int
		changeOnly bool
	}{
		{"baseline", []int{80, 100, 120, 100, 200}, false},
		{"change-only", []int{80, 100, 200}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			db := hotSave(map[int][]ahpHistRow{
				1: hotWithSupply(1500, 49, c.earlier...),
				2: hotWithSupply(1500, 50, c.earlier...),
				3: hotWithSupply(1500, 9, c.earlier...),  // old listed minimum, not counted again as thin
				4: hotWithSupply(1000, 10, c.earlier...), // flat, not an otherwise qualifying riser
				5: hotWithSupply(1500, 10),               // no earlier days: no Previous, no thin filter
			})
			h := buildHot(db, 3, true, hotHouses, nil, nil, hotNow)
			hh := h.Houses[0]
			if got := hotIDs(hh); !reflect.DeepEqual(got, []int{2}) || hh.Checked != 4 || hh.Dropped != 2 || hh.ChangeOnly != c.changeOnly {
				t.Errorf("49%% excluded, 50%% kept; counts and mode: %+v", hh)
			}
			html := hotHTML(t, h)
			for _, want := range []string{"4 items checked, 2 left out by the filters, 1 for thin supply", "listed (scan day) / usual", ">50 / 100</td>"} {
				if !strings.Contains(html, want) {
					t.Errorf("source/supply lacks %q", want)
				}
			}
		})
	}
}

func TestHotSupplyUsesUnroundedMedian(t *testing.T) {
	h := buildHot(hotSave(map[int][]ahpHistRow{
		1: hotWithSupply(1500, 50, 100, 101, 100, 101, 100, 101), // 50 < 50% of 100.5
		2: hotWithSupply(1500, 51, 100, 101, 100, 101, 100, 101),
	}), 3, true, hotHouses, nil, nil, hotNow)
	if got := hotIDs(h.Houses[0]); !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("filter with the exact median, not its integer display: %v", got)
	}
	if !strings.Contains(hotHTML(t, h), ">51 / 101</td>") {
		t.Error("display rounds usual supply to a whole unit")
	}
}

func TestHotNoEarlierSupplyIsNotThin(t *testing.T) {
	h := buildHot(hotSave(map[int][]ahpHistRow{1: hotWithSupply(1500, 10)}), 3, true, hotHouses, nil, nil, hotNow)
	if hh := h.Houses[0]; !hh.ChangeOnly || hh.Checked != 0 || hh.Dropped != 0 || len(hh.Rows) != 0 {
		t.Fatalf("without Previous, neither compared nor filtered: %+v", hh)
	}
	if !strings.Contains(hotHTML(t, h), "0 items checked, 0 left out by the filters, 0 for thin supply") {
		t.Error("no earlier supply must not count as a thin-supply rejection")
	}
}
