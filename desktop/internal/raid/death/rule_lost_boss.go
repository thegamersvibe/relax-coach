package death

import (
	"slices"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(4, lostBoss) }

func lostBoss(c *context) verdict {
	killing := bossMelee(c)
	if killing == nil || pulledAggro(c).cause != "" {
		return verdict{}
	}
	evidence := []int64{}
	for _, e := range c.window {
		if damage(e) && e.Stream == model.SwingLanded && slices.Contains(c.view.f.BossUnits, e.Src.ID) && e.Dst.ID == c.victim.ID {
			evidence = append(evidence, e.TimeMs)
		}
	}
	// An unresolved aggro recap is a terminal fallback, not proof of a cause.
	unknown := verdict{cause: Unresolved, owner: model.OwnerUndecided, evidence: evidence, terminal: true}
	turn := killingRun(c, killing.Src.ID)
	at := c.view.events[turn].TimeMs
	holder := ""
	for i := turn - 1; i >= 0; i-- {
		e := c.view.events[i]
		if at-e.TimeMs > c.params.WindowMs {
			break
		}
		if melee(e) && e.Src.ID == killing.Src.ID {
			holder = e.Dst.ID
			break
		}
	}
	if c.params.LostBossGapMs == nil || c.view.f.Roles[holder] != "tank" || !c.view.aliveBefore(holder, turn) {
		return unknown
	}
	var last model.Opt
	for _, e := range c.view.events[:c.end+1] {
		// The holder's damage at the turn millisecond counts as engagement;
		// no later timestamp (or line after this death) repairs the gap.
		if e.TimeMs > at {
			break
		}
		if damage(e) && e.Dst.ID == killing.Src.ID && c.view.owner(e.Src.ID) == holder {
			last = model.Some(e.TimeMs)
		}
	}
	if !last.Present || at-last.V > *c.params.LostBossGapMs {
		return verdict{cause: TankLostBoss, owner: model.OwnerPlayer, owners: []string{c.view.players[holder].Name}, evidence: evidence}
	}
	return unknown
}
