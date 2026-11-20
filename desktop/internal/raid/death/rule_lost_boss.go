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
	// First swing onto the victim in this review window is the observed turn.
	turn := killing.TimeMs
	evidence := []int64{}
	for _, e := range c.window {
		if damage(e) && e.Stream == model.SwingLanded && slices.Contains(c.view.f.BossUnits, e.Src.ID) && e.Dst.ID == c.victim.ID {
			if e.Src.ID == killing.Src.ID && e.TimeMs < turn {
				turn = e.TimeMs
			}
			evidence = append(evidence, e.TimeMs)
		}
	}
	var last model.Opt
	owner := ""
	tied := false
	for _, e := range c.view.events {
		if e.TimeMs >= turn {
			break
		}
		id := c.view.owner(e.Src.ID)
		if !damage(e) || effective(e) == 0 || e.Dst.ID != killing.Src.ID || c.view.f.Roles[id] != "tank" || !c.view.alive(id, killing.TimeMs) {
			continue
		}
		if !last.Present || e.TimeMs > last.V {
			last = model.Some(e.TimeMs)
			owner = id
			tied = false
		} else if id != owner {
			tied = true
		}
	}
	if c.params.LostBossGapMs != nil && last.Present && !tied && turn-last.V > *c.params.LostBossGapMs {
		return verdict{cause: TankLostBoss, owner: "player", owners: []string{c.view.players[owner].Name}, evidence: evidence}
	}
	// No threat table: do not invent a model cause or a healer charge. R8 can
	// label this evidence "aggro: puller or tank" without a seventh suggestion.
	return verdict{cause: Unresolved, owner: "undecided", evidence: evidence, terminal: true}
}
