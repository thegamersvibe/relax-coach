package logfmt

import "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"

// Format knowledge is data in one table. Widths include the event name, but
// not the timestamp. false/true means absent/present advanced block, determined
// per line, never from a header. Nonadvanced variants remove exactly 19 fields.
type format struct {
	kind                 model.Kind
	stream               model.Stream
	spell, environmental bool
	widths               map[int]bool
}

func damageFormat(full int, stream model.Stream, spell, environmental bool) format {
	return format{model.Damage, stream, spell, environmental, map[int]bool{full: true, full - 19: false}}
}

func plain(kind model.Kind, spell bool, widths ...int) format {
	w := make(map[int]bool, len(widths))
	for _, n := range widths {
		w[n] = false
	}
	return format{kind: kind, spell: spell, widths: w}
}
func withAdvanced(kind model.Kind, full int) format {
	return format{kind: kind, spell: true, widths: map[int]bool{full: true, full - 19: false}}
}

var formats = map[string]format{
	"COMBAT_LOG_VERSION":      plain(model.Header, false, 8),
	"ENCOUNTER_START":         plain(model.EncounterStart, false, 6),
	"ENCOUNTER_END":           plain(model.EncounterEnd, false, 7),
	"ZONE_CHANGE":             plain(model.ZoneChange, false, 4),
	"COMBATANT_INFO":          plain(model.CombatantInfo, false, 34),
	"SWING_MISSED":            plain(model.Missed, false, 11, 12, 14),
	"SPELL_MISSED":            plain(model.Missed, true, 15, 16, 18),
	"SPELL_PERIODIC_MISSED":   plain(model.Missed, true, 15, 16, 18),
	"RANGE_MISSED":            plain(model.Missed, true, 14, 17),
	"DAMAGE_SHIELD_MISSED":    plain(model.Missed, true, 15, 16),
	"SPELL_HEAL":              withAdvanced(model.Heal, 36),
	"SPELL_PERIODIC_HEAL":     withAdvanced(model.Heal, 36),
	"SPELL_ABSORBED":          plain(model.Absorb, false, 19, 22),
	"SPELL_ENERGIZE":          withAdvanced(model.Energize, 35),
	"SPELL_PERIODIC_ENERGIZE": withAdvanced(model.Energize, 35),
	"SPELL_PERIODIC_LEECH":    withAdvanced(model.Energize, 35),
	"SPELL_DRAIN":             withAdvanced(model.Energize, 35),
	"SPELL_PERIODIC_DRAIN":    withAdvanced(model.Energize, 35),
	"SPELL_CAST_START":        plain(model.CastStart, true, 12),
	"SPELL_CAST_SUCCESS":      withAdvanced(model.CastSuccess, 31),
	"SPELL_CAST_FAILED":       plain(model.CastFailed, true, 13),
	"SPELL_AURA_APPLIED":      plain(model.AuraApplied, true, 13, 14),
	"SPELL_AURA_REMOVED":      plain(model.AuraRemoved, true, 13, 14),
	"SPELL_AURA_REFRESH":      plain(model.AuraRefresh, true, 13, 14),
	"SPELL_AURA_APPLIED_DOSE": plain(model.AuraDose, true, 14),
	"SPELL_AURA_REMOVED_DOSE": plain(model.AuraDose, true, 14),
	"SPELL_AURA_BROKEN":       plain(model.AuraBroken, true, 13),
	"SPELL_AURA_BROKEN_SPELL": plain(model.AuraBroken, true, 16),
	"SPELL_INTERRUPT":         plain(model.Interrupt, true, 15),
	"SPELL_DISPEL":            plain(model.Dispel, true, 16),
	"SPELL_SUMMON":            plain(model.Summon, true, 12),
	"SPELL_RESURRECT":         plain(model.Resurrect, true, 12),
	"UNIT_DIED":               plain(model.UnitDied, false, 10),
	"PARTY_KILL":              plain(model.PartyKill, false, 10),
	"SPELL_EXTRA_ATTACKS":     plain(model.Other, false, 13),
	"SPELL_INSTAKILL":         plain(model.Other, false, 13),
	"SPELL_CREATE":            plain(model.Other, false, 12),
	"ENCHANT_APPLIED":         plain(model.Other, false, 12),
	"ENCHANT_REMOVED":         plain(model.Other, false, 12),
	"MAP_CHANGE":              plain(model.Other, false, 7),
	"SWING_DAMAGE":            damageFormat(38, model.SwingSource, false, false),
	"SWING_DAMAGE_LANDED":     damageFormat(38, model.SwingLanded, false, false),
	"SPELL_DAMAGE":            damageFormat(42, model.StreamNone, true, false),
	"SPELL_PERIODIC_DAMAGE":   damageFormat(42, model.StreamNone, true, false),
	"RANGE_DAMAGE":            damageFormat(42, model.StreamNone, true, false),
	"DAMAGE_SHIELD":           damageFormat(42, model.StreamNone, true, false),
	"DAMAGE_SPLIT":            damageFormat(42, model.StreamNone, true, false),
	"ENVIRONMENTAL_DAMAGE":    damageFormat(39, model.StreamNone, false, true),
}
