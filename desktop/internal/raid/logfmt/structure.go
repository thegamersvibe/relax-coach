package logfmt

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"strings"
)

func structural(kind model.Kind, f []string) (model.Event, SkipReason) {
	e := model.Event{Kind: kind}
	switch kind {
	case model.EncounterStart, model.EncounterEnd:
		id, difficulty, size := number(f[1]), number(f[3]), number(f[4])
		if !id.Present || !difficulty.Present || !size.Present {
			return model.Event{}, InvalidValue
		}
		e.Enc = &model.Encounter{ID: id.V, Name: strings.Clone(f[2]), Difficulty: difficulty.V, GroupSize: size.V}
		if kind == model.EncounterStart {
			e.Enc.InstanceID = number(f[5])
		} else {
			e.Enc.Success, e.Enc.FightMs = number(f[5]), number(f[6])
		}
	case model.ZoneChange:
		id, difficulty := number(f[1]), number(f[3])
		if !id.Present || !difficulty.Present {
			return model.Event{}, InvalidValue
		}
		e.Zone = &model.Zone{InstanceID: id.V, Name: strings.Clone(f[2]), Difficulty: difficulty.V}
	case model.CombatantInfo:
		info, ok := combatant(f)
		if !ok {
			return model.Event{}, InvalidValue
		}
		e.Info, e.Raw = info, f
	}
	return e, ""
}

func collection(s string) ([]string, bool) {
	if len(s) < 2 {
		return nil, false
	}
	if !((s[0] == '[' && s[len(s)-1] == ']') || (s[0] == '(' && s[len(s)-1] == ')')) {
		return nil, false
	}
	s = s[1 : len(s)-1]
	if strings.TrimSpace(s) == "" {
		return nil, true
	}
	return split(s)
}

func combatant(f []string) (*model.Combatant, bool) {
	c := &model.Combatant{PlayerID: absent(f[1]), Raw: f}
	if c.PlayerID == "" {
		return nil, false
	}
	// The numbers' positions are known, not their class/spec semantics.
	for _, s := range f[2:26] {
		n := number(s)
		if !n.Present {
			return nil, false
		}
		c.Stats = append(c.Stats, n.V)
	}
	talents, ok := collection(f[26])
	if !ok {
		return nil, false
	}
	for _, s := range talents {
		parts, ok := collection(s)
		if !ok || len(parts) != 3 {
			return nil, false
		}
		var t [3]int64
		for i, p := range parts {
			n := number(p)
			if !n.Present {
				return nil, false
			}
			t[i] = n.V
		}
		c.Talents = append(c.Talents, t)
	}
	gear, ok := collection(f[28])
	if !ok {
		return nil, false
	}
	for _, s := range gear {
		parts, ok := collection(s)
		if !ok || len(parts) != 5 {
			return nil, false
		}
		id, level := number(parts[0]), number(parts[1])
		if !id.Present || !level.Present {
			return nil, false
		}
		c.Gear = append(c.Gear, model.GearSlot{ItemID: id.V, ItemLevel: level.V, Extra: parts[2:]})
	}
	auras, ok := collection(f[29])
	if !ok || len(auras)%3 != 0 {
		return nil, false
	}
	for i := 0; i < len(auras); i += 3 {
		id, stacks := number(auras[i+1]), number(auras[i+2])
		if !id.Present || !stacks.Present {
			return nil, false
		}
		c.Auras = append(c.Auras, model.Aura{CasterID: absent(auras[i]), SpellID: id.V, Stacks: stacks.V})
	}
	return c, true
}
