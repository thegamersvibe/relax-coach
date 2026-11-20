package death

import (
	"slices"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func init() { Register(5, unavoidable) }

func unavoidable(c *context) verdict {
	killing, spells := candidates(c)
	if killing == nil {
		return verdict{}
	}
	if killing.Src.ID == "" {
		if !environmental(*killing) {
			return verdict{cause: Unresolved, owner: model.OwnerUndecided, terminal: true}
		}
		// Environmental deaths with a registry entry need officer review; even
		// a slow fall/fire death must never be charged to the healing team.
		for _, m := range c.mechanics {
			if m.Spell.ID == killing.SpellID {
				return verdict{cause: Unresolved, owner: model.OwnerUndecided, terminal: true}
			}
		}
		return verdict{cause: Unavoidable, owner: model.OwnerNobody, terminal: true}
	}
	if oneShot(c) {
		return verdict{cause: Unavoidable, owner: model.OwnerNobody}
	}
	for _, spell := range spells {
		if raidwide(c, *killing, spell) {
			return verdict{cause: Unavoidable, owner: model.OwnerNobody}
		}
	}
	return verdict{}
}

// logfmt clears the source and places ENVIRONMENTAL_DAMAGE's type in
// SpellName, with no spell ID or swing stream. A missing actor alone is unknown.
func environmental(e model.Event) bool {
	return e.SpellID == 0 && e.Stream == model.StreamNone &&
		slices.Contains([]string{"FALLING", "DROWNING", "FATIGUE", "FIRE", "LAVA", "SLIME"}, e.SpellName)
}

func oneShot(c *context) bool {
	var hpMax, amount int64
	for _, e := range c.window {
		if healthSample(e, c.victim.ID) {
			hpMax = e.HPMax.V
		}
		if damage(e) && e.Dst.ID == c.victim.ID && e.TimeMs >= c.at-c.params.OneShotWindowMs {
			amount += effective(e)
		}
	}
	return hpMax > 0 && amount >= hpMax
}

// Creature identity is independent of names. A pet/guardian is never a raid-
// wide source, even when its GUID is creature-shaped and its owner is known.
func (v *fightView) creature(a model.Actor) bool {
	return (strings.HasPrefix(a.ID, "Creature-") || strings.HasPrefix(a.ID, "Vehicle-")) &&
		v.f.Owners[a.ID] == "" && !slices.Contains(v.f.Unowned, a.ID) && byte(a.Flags>>12)&3 == 0
}

func raidwide(c *context, killing, spell model.Event) bool {
	if spell.Stream != model.StreamNone || !c.view.creature(spell.Src) || spell.Src.Flags&0x40 == 0 {
		return false
	}
	for _, m := range c.mechanics {
		if m.Spell.ID == spell.SpellID && m.Avoidable {
			return false
		}
	}
	from, to := killing.TimeMs-c.params.RaidwideWindowMs, killing.TimeMs+c.params.RaidwideWindowMs
	alive := 0
	for _, id := range c.view.ids {
		if id == c.victim.ID || c.view.alive(id, from) {
			alive++
		}
	}
	hit := map[string]bool{}
	for _, e := range c.view.events {
		if e.TimeMs < from {
			continue
		}
		if e.TimeMs > to {
			continue
		}
		if !damage(e) || e.Stream != model.StreamNone || e.SpellID != spell.SpellID || !c.view.creature(e.Src) {
			continue
		}
		if _, ok := c.view.players[e.Dst.ID]; ok {
			hit[e.Dst.ID] = true
		}
	}
	return alive > 0 && len(hit) >= 3 && float64(len(hit))/float64(alive) >= c.params.RaidwideShare
}
