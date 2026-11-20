package death

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(1, afterWipe) }
func afterWipe(c *context) verdict {
	if c.wipeCall.Present && c.at > c.wipeCall.V {
		// Keep priority, but still count other actual matches for confidence.
		return verdict{cause: AfterWipeCall, owner: model.OwnerNobody}
	}
	return verdict{}
}
func wipeCall(v *fightView, p cfg.Death) model.Opt {
	dead := map[string]bool{}
	for _, e := range v.events {
		if _, ok := v.players[e.Dst.ID]; !ok {
			continue
		}
		if e.Kind == model.Resurrect {
			delete(dead, e.Dst.ID)
		}
		if !e.IsDeath() {
			continue
		}
		dead[e.Dst.ID] = true
		allRoleDead := func(role string) bool {
			count := 0
			for _, id := range v.ids {
				if v.f.Roles[id] != role {
					continue
				}
				count++
				if !dead[id] {
					return false
				}
			}
			return count > 0
		}
		if p.WipeDeadShare > 0 && float64(len(dead))/float64(len(v.ids)) >= p.WipeDeadShare || p.WipeAllTanksDead && allRoleDead("tank") || p.WipeAllHealersDead && allRoleDead("healer") {
			return model.Some(e.TimeMs)
		}
	}
	return model.Opt{}
}
func summaries(v *fightView, deaths []model.Death, p cfg.Death) []model.Wipe {
	if !v.f.Boss || v.f.Success != model.Some(0) || p.SummaryWindowMs == nil {
		return nil
	}
	end := v.f.EndMs
	for _, e := range v.events {
		if e.Kind == model.EncounterEnd {
			end = e.TimeMs
			break
		}
	}
	w := model.Wipe{FightID: v.f.ID, FirstDeath: -1}
	if len(deaths) > 0 && deaths[0].TimeMs <= end {
		w.FirstDeath = 0
		for i := 1; i < len(deaths); i++ {
			if deaths[i].TimeMs-deaths[0].TimeMs > *p.SummaryWindowMs {
				break
			}
			w.Following = append(w.Following, i)
		}
	}
	return []model.Wipe{w}
}
