package score_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func prepCutoffFight(offset int8, fractionalMs int) (model.Fight, int64) {
	f := fixture("tank")
	// Invented wall time just after midnight. Compute expected UTC independently
	// with a zone, rather than copying the scorer's offset subtraction.
	wall := time.Date(2026, 6, 8, 0, 0, 20, fractionalMs*1000000, time.UTC)
	utc := time.Date(2026, 6, 8, 0, 0, 20, fractionalMs*1000000, time.FixedZone("test", int(offset)*3600))
	shift := wall.UnixMilli() - f.EndMs
	f.StartMs, f.EndMs = f.StartMs+shift, f.EndMs+shift
	for i := range f.Events {
		f.Events[i].TimeMs += shift
		f.Events[i].OffsetH = offset
	}
	return f, utc.Unix()
}

func TestPrepOnlyAfterEndIsUnmeasured(t *testing.T) {
	c := config(t)
	for _, offset := range []int8{3, -4, 0, 14, -12, 23, -23} {
		for _, fraction := range []int{0, 750} {
			t.Run(fmt.Sprintf("offset_%d_fraction_%d", offset, fraction), func(t *testing.T) {
				f, end := prepCutoffFight(offset, fraction)
				r := stock(f.Players[0])
				r.Stamped, r.SenderClock = end+1, end-1 // sender clock cannot admit it
				r.BetterGear = []model.PrepGear{{Slot: 1}}
				later := r
				later.Stamped++
				want := score.Score([]model.Fight{f}, nil, nil, c)
				for _, reports := range [][]model.PrepReport{{r}, {later, r}, {r, later}} {
					got := score.Score([]model.Fight{f}, nil, reports, c)
					p := part(t, got[0], "preparation")
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("future-only must equal no reports: prep=%v prep_coverage=%d night_coverage=%d want=%d selected=%d", p.Tenths, p.Coverage, got[0].Coverage, want[0].Coverage, p.Evidence.Counts["prep_stamped_s"])
					}
				}
			})
		}
	}
}

func TestPrepFutureStockCannotCreditEarlierFight(t *testing.T) {
	c := config(t)
	for _, offset := range []int8{3, -4, 0} {
		for _, unreadable := range []bool{false, true} {
			t.Run(fmt.Sprintf("offset_%d_unreadable_%v", offset, unreadable), func(t *testing.T) {
				f, end := prepCutoffFight(offset, 750)
				before, future := stock(f.Players[0]), stock(f.Players[0])
				before.Stamped, future.Stamped = end, end+1
				before.Consumables = nil
				before.ConsumablesUnreadable = unreadable
				future.BetterGear = []model.PrepGear{{Slot: 1}}
				want := score.Score([]model.Fight{f}, nil, []model.PrepReport{before}, c)
				for _, reports := range [][]model.PrepReport{{before, future}, {future, before}} {
					got := score.Score([]model.Fight{f}, nil, reports, c)
					p, base := part(t, got[0], "preparation"), part(t, want[0], "preparation")
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("future stock changed earlier fight: prep=%v want=%v coverage=%d want=%d stock_measured=%d want=%d", p.Tenths, base.Tenths, p.Coverage, base.Coverage, p.Evidence.Counts["measured.stock"], base.Evidence.Counts["measured.stock"])
					}
				}
			})
		}
	}
}

func TestPrepCutoffInclusiveAndLatest(t *testing.T) {
	c := config(t)
	for _, offset := range []int8{3, -4, 0, 23, -23} {
		for _, fraction := range []int{0, 750} {
			f, end := prepCutoffFight(offset, fraction)
			early, latest, atEnd, after := stock(f.Players[0]), stock(f.Players[0]), stock(f.Players[0]), stock(f.Players[0])
			early.Stamped, latest.Stamped, atEnd.Stamped, after.Stamped = end-2, end-1, end, end+1
			early.BetterGear = []model.PrepGear{{Slot: 1}}
			after.BetterGear = []model.PrepGear{{Slot: 1}, {Slot: 2}}
			for _, tc := range []struct {
				name    string
				reports []model.PrepReport
				stamp   int64
			}{
				{"at_end", []model.PrepReport{atEnd}, end},
				{"mixed_at_end", []model.PrepReport{after, early, atEnd, latest}, end},
				{"mixed_before_end", []model.PrepReport{after, early, latest}, end - 1},
			} {
				t.Run(fmt.Sprintf("offset_%d_fraction_%d_%s", offset, fraction, tc.name), func(t *testing.T) {
					for order := 0; order < 2; order++ {
						p := part(t, score.Score([]model.Fight{f}, nil, tc.reports, c)[0], "preparation")
						if p.Tenths != model.Some(1000) || p.Coverage != 60 || p.Evidence.Counts["prep_stamped_s"] != tc.stamp || p.Evidence.Counts["fight_end_utc_s"] != end {
							t.Fatalf("latest at/before end %d: %+v", end, p)
						}
						for i, j := 0, len(tc.reports)-1; i < j; i, j = i+1, j-1 {
							tc.reports[i], tc.reports[j] = tc.reports[j], tc.reports[i]
						}
					}
				})
			}
		}
	}
}
