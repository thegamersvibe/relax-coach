package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"testing"
)

func TestIncompleteFightMarksRunPartial(t *testing.T) {
	es := append(boss(7, 1000)[:4], modeltest.At(2000).Zone(0, "Outside", 0).Build())
	r := only(t, build(t, es, params()))
	if !r.Partial || !r.Fights[0].Partial {
		t.Fatal("truncated boss not reflected on run")
	}
	es = append(boss(7, 1000), hit(100000), model.Event{Kind: model.Header, TimeMs: 100001})
	r = only(t, build(t, es, params()))
	if !r.Fights[1].Partial || r.Details[1].UnscoredReason != "partialFight" {
		t.Fatal("header-truncated trash treated as complete")
	}
}
func TestDeadBossRemainsZeroForWipeProgress(t *testing.T) {
	es := boss(7, 1000)
	more := []model.Event{modeltest.At(1010).Dst(giant).Died(0).Build(), {TimeMs: 1011, InfoUnit: giant.ID, HP: model.Some(50), HPMax: model.Some(900)}}
	es = append(es[:4], append(more, es[4:]...)...)
	if only(t, build(t, es, params())).Details[0].RemainingHP != model.Some(0) {
		t.Fatal("dead boss revived by stale health snapshot")
	}
}
