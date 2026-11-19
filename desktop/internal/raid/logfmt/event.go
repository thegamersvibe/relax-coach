package logfmt

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"math"
	"strconv"
	"strings"
)

func absent(s string) string {
	if s == "nil" || s == "0000000000000000" {
		return ""
	}
	// A substring would keep the entire line alive in a retained typed event.
	return strings.Clone(s)
}
func number(s string) model.Opt {
	// A decimal spelling of an integer is used by energize; fractional values
	// are unrepresentable in Opt and stay unknown, not rounded to zero.
	if whole, fraction, ok := strings.Cut(s, "."); ok {
		if fraction == "" || strings.Trim(fraction, "0") != "" {
			return model.Opt{}
		}
		s = whole
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return model.Opt{}
	}
	return model.Some(v)
}
func position(s string) model.Opt {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return model.Opt{}
	}
	v = math.Round(v * 100)
	if v >= float64(math.MaxInt64) || v < float64(math.MinInt64) {
		return model.Opt{}
	}
	return model.Some(int64(v))
}
func actor(f []string) model.Actor {
	flags, _ := strconv.ParseUint(f[2], 0, 32)
	return model.Actor{ID: absent(f[0]), Name: absent(f[1]), Flags: uint32(flags)}
}

func event(f []string) (model.Event, SkipReason, bool) {
	schema, known := formats[f[0]]
	if !known {
		return model.Event{Kind: model.Other, Raw: f}, "", false
	}
	advanced, ok := schema.widths[len(f)]
	if !ok {
		return model.Event{}, UnknownWidth, true
	}
	if schema.kind == model.Header {
		if !number(f[1]).Present || f[2] != "ADVANCED_LOG_ENABLED" || f[4] != "BUILD_VERSION" || f[6] != "PROJECT_ID" {
			return model.Event{}, InvalidValue, true
		}
		return model.Event{Kind: model.Header}, "", true
	}
	if schema.kind == model.Other {
		return model.Event{Kind: model.Other, Raw: f}, "", true
	}
	if schema.kind == model.EncounterStart || schema.kind == model.EncounterEnd || schema.kind == model.ZoneChange || schema.kind == model.CombatantInfo {
		e, reason := structural(schema.kind, f)
		return e, reason, true
	}
	e := model.Event{Kind: schema.kind, Stream: schema.stream, Src: actor(f[1:5]), Dst: actor(f[5:9])}
	p := 9
	if schema.spell || (schema.kind == model.Absorb && len(f) == 22) {
		id := number(f[p])
		if !id.Present {
			return model.Event{}, InvalidValue, true
		}
		e.SpellID, e.SpellName = id.V, strings.Clone(f[p+1])
		p += 3
	}
	if advanced {
		e.InfoUnit, e.InfoOwnerID = absent(f[p]), absent(f[p+1])
		e.HP, e.HPMax = number(f[p+2]), number(f[p+3])
		// V22 offsets: wowcoach.gg/docs/combat-log/advanced-logging (PLAN M2).
		// Zero is a real power type (mana), never the unknown sentinel.
		e.PowerType = number(f[p+10])
		e.Power, e.PowerMax = number(f[p+11]), number(f[p+12])
		e.X, e.Y = position(f[p+14]), position(f[p+15])
		p += 19
	}
	if schema.environmental {
		e.Src = model.Actor{}
		e.SpellName = strings.Clone(f[p])
		p++
	}
	switch e.Kind {
	case model.Damage:
		e.Amount, e.Overkill, e.Absorbed = number(f[p]), number(f[p+2]), number(f[p+6])
		e.Crit = f[p+7] == "1"
	case model.Heal:
		e.Amount, e.Overheal, e.Absorbed = number(f[p]), number(f[p+2]), number(f[p+3])
		e.Crit = f[p+4] == "1"
	case model.Energize:
		e.Amount = number(f[p])
	case model.Missed:
		e.MissType = strings.Clone(f[p])
		if e.MissType == "ABSORB" && len(f) > p+2 {
			e.Absorbed = number(f[p+2])
		}
	case model.Absorb:
		// Src/Dst remain the common logged actors. The current model has no
		// separate shield-caster actor; do not mislabel it as an advanced owner.
		e.ExtraSpellID = number(f[p+4])
		e.Amount = number(f[p+7])
		e.Crit = f[p+9] == "1"
	case model.UnitDied:
		e.DiedFlag = number(f[p])
	case model.AuraApplied, model.AuraRemoved, model.AuraRefresh, model.AuraDose:
		e.AuraType = strings.Clone(f[p])
		if len(f) > p+1 {
			e.Amount = number(f[p+1])
		}
	case model.AuraBroken:
		if len(f) == 16 {
			e.ExtraSpellID = number(f[p])
			p += 3
		}
		e.AuraType = strings.Clone(f[p])
	case model.Interrupt, model.Dispel:
		e.ExtraSpellID = number(f[p])
		if e.Kind == model.Dispel {
			e.AuraType = strings.Clone(f[p+3])
		}
	}
	return e, "", true
}
