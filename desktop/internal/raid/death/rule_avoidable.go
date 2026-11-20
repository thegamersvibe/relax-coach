package death

import (
	"slices"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(2, avoidable) }

// candidates keeps the last landed damage and the strictly-majority spell.
// Iterating the window, not a map, gives a stable representative source.
func candidates(c *context) (killing *model.Event, spells []model.Event) {
	amounts := map[int64]int64{}
	var total int64
	for i, e := range c.window {
		if !damage(e) || e.Dst.ID != c.victim.ID {
			continue
		}
		killing = &c.window[i]
		n := effective(e)
		amounts[e.SpellID] += n
		total += n
	}
	if killing == nil {
		return nil, nil
	}
	spells = append(spells, *killing)
	for _, e := range c.window {
		if !damage(e) || e.Dst.ID != c.victim.ID || e.SpellID == killing.SpellID {
			continue
		}
		if amounts[e.SpellID] > total-amounts[e.SpellID] {
			spells = append(spells, e)
			break
		}
	}
	return killing, spells
}
func avoidable(c *context) verdict {
	killing, spells := candidates(c)
	if killing == nil || killing.Src.ID == "" {
		return verdict{}
	}
	for _, e := range spells {
		// Environmental damage is never charged to the player or healers.
		if e.Src.ID == "" {
			continue
		}
		for _, m := range c.mechanics {
			if m.Spell.ID == e.SpellID && m.Avoidable && (len(m.Roles) == 0 || slices.Contains(m.Roles, c.view.f.Roles[c.victim.ID])) {
				return verdict{cause: Avoidable, owner: model.OwnerPlayer, owners: []string{c.victim.Name}}
			}
		}
	}
	return verdict{}
}
