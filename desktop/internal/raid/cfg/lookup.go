package cfg

import (
	"strconv"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// InstanceKind uses the listed kind first, then only the encounter's group
// size. listed=false MUST be reported by the caller, including for inferred
// raids; a five-player group does not establish an unknown instance as dungeon.
func (c *Config) InstanceKind(instanceID, groupSize int64) (kind model.RunKind, listed bool) {
	for _, i := range c.Instances.Instances {
		if i.ID == instanceID {
			if i.Kind == "raid" {
				return model.RunRaid, true
			}
			return model.RunDungeon, true
		}
	}
	if groupSize > c.Instances.RaidGroupSizeAbove {
		return model.RunRaid, false
	}
	return model.RunUnknown, false
}

// Profile looks up a canonical CLASS.spec id. Unknown/ambiguous ids stay unknown.
func (c *Config) Profile(id string) (Profile, bool) {
	for _, p := range c.Specs.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Encounter consults enabled rules only, never research candidates.
func (c *Config) Encounter(id int64) (EncounterRule, bool) {
	for _, e := range c.Encounters.Encounters {
		if e.ID == id {
			return e, true
		}
	}
	return EncounterRule{}, false
}

// OutputTargets defaults to encounter boss units, even for an unknown boss.
// This policy is independent of BossUnits, which identifies tanking exposure.
func (c *Config) OutputTargets(encounterID int64) TargetRule {
	if e, ok := c.Encounter(encounterID); ok && e.TargetRule != nil {
		return *e.TargetRule
	}
	return TargetRule{Mode: c.Encounters.DefaultTargetRule, Units: []CreatureSelector{}}
}

// ItemLabel is the no-database fallback. itemdata does not exist yet; item
// names/slots/levels must come through that shared provider when it is added,
// never from a duplicate raid loot/item catalogue. No item-dependent measure
// can be computed from this label alone.
func ItemLabel(id int64) string { return strconv.FormatInt(id, 10) }
