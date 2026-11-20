package death

import (
	"slices"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(3, pulledAggro) }

func bossMelee(c *context) *model.Event {
	killing, _ := candidates(c)
	role := c.view.f.Roles[c.victim.ID]
	if killing == nil || (role != "dps" && role != "healer") || killing.Stream != model.SwingLanded || !slices.Contains(c.view.f.BossUnits, killing.Src.ID) {
		return nil
	}
	return killing
}
func (v *fightView) owner(id string) string {
	if o := v.f.Owners[id]; o != "" {
		return o
	}
	return id
}

// melee includes the parser's spell-less SWING_MISSED representation. Source-
// side damage duplicates are not landed swings and do not establish a turn.
func melee(e model.Event) bool {
	return damage(e) && e.Stream == model.SwingLanded || e.Kind == model.Missed && e.SpellID == 0 && e.SpellName == ""
}

func killingRun(c *context, bossID string) int {
	for i := c.start; i <= c.end; i++ {
		e := c.view.events[i]
		if melee(e) && e.Src.ID == bossID && e.Dst.ID == c.victim.ID {
			return i
		}
	}
	return c.end // bossMelee guarantees a landed killing swing in the window.
}

// pulledAggroInRange is the sole rule-3 reach policy: first damage in this
// fight, with the killing run at most the configured interval after it.
func pulledAggroInRange(c *context, first, turn int64) bool {
	return turn >= first && turn-first <= c.params.PulledAggroMs
}

func pulledAggro(c *context) verdict {
	killing := bossMelee(c)
	if killing == nil {
		return verdict{}
	}
	turn := killingRun(c, killing.Src.ID)
	var firstVictim, firstTank model.Opt
	for _, e := range c.view.events[:c.end+1] {
		if e.TimeMs < c.view.f.StartMs || !damage(e) || e.Dst.ID != killing.Src.ID {
			continue
		}
		owner := c.view.owner(e.Src.ID)
		if owner == c.victim.ID && !firstVictim.Present {
			firstVictim = model.Some(e.TimeMs)
		}
		if c.view.f.Roles[owner] == "tank" && !firstTank.Present {
			firstTank = model.Some(e.TimeMs)
		}
	}
	if firstVictim.Present && (!firstTank.Present || firstVictim.V < firstTank.V) && pulledAggroInRange(c, firstVictim.V, c.view.events[turn].TimeMs) {
		return verdict{cause: PulledAggro, owner: "player", owners: []string{c.victim.Name}}
	}
	return verdict{}
}
