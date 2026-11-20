package fights

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestBossRolesRequireTankEvidence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		knownBoss bool
		landed    bool
		wantRoles int
	}{
		{"no-boss-unit-damaging-swing-taker", false, true, 0},
		{"known-boss-no-landed-swings", true, false, 0},
		{"known-boss-landed-swings", true, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			es := boss(7, 1000)
			if tc.knownBoss {
				es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
			}
			swing := modeltest.At(1010).Src(giant).Dst(alia).Damage(10, -1).Landed().Build()
			if !tc.landed {
				swing.Stream = model.SwingSource
			}
			es = append(es[:4], swing,
				modeltest.At(1011).Src(giant).Dst(alia).Miss("DODGE").Build(),
				modeltest.At(1012).Src(bram).Dst(alia).Heal(30, 0).Build(), es[4])
			r := only(t, build(t, es, params()))
			f, d := r.Fights[0], r.Details[0]
			if (len(f.BossUnits) > 0) != tc.knownBoss || d.Timelines[alia.ID].DamageDone != 100 || d.Timelines[bram.ID].HealingDone != 30 {
				t.Fatal("fixture lost boss selection or positive output")
			}
			if len(f.Roles) != tc.wantRoles {
				t.Fatalf("roles=%v: without boss tank evidence no role may be assigned", f.Roles)
			}
			if tc.wantRoles > 0 && (f.Roles[alia.ID] != "tank" || f.Roles[bram.ID] != "healer") {
				t.Fatal("positive tank evidence did not enable roles")
			}
		})
	}
}
