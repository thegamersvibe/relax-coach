package death

import (
	"slices"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(3, pulledAggro) }

func bossMelee(c *context) *model.Event {
	killing, _ := candidates(c)
	if killing == nil || c.view.f.Roles[c.victim.ID] == "tank" || killing.Stream != model.SwingLanded || !slices.Contains(c.view.f.BossUnits, killing.Src.ID) {
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
func pulledAggro(c *context) verdict {
	killing := bossMelee(c)
	if killing == nil {
		return verdict{}
	}
	tankAlive := false
	for _, id := range c.view.ids {
		if c.view.f.Roles[id] == "tank" && c.view.alive(id, killing.TimeMs) {
			tankAlive = true
		}
	}
	var firstVictim, firstTank model.Opt
	for _, e := range c.view.events {
		if e.TimeMs > killing.TimeMs {
			break
		}
		if !damage(e) || e.Dst.ID != killing.Src.ID || effective(e) == 0 {
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
	if !tankAlive || firstVictim.Present && (!firstTank.Present || firstVictim.V < firstTank.V) {
		return verdict{cause: PulledAggro, owner: "player", owners: []string{c.victim.Name}}
	}
	return verdict{}
}
