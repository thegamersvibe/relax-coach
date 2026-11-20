package fixture

import (
	"fmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func unit(id string) Field {
	if id == "" {
		return Field("0000000000000000")
	}
	return Field(id)
}
func optional(v model.Opt) Field {
	if !v.Present {
		return Nil
	}
	return Number(v.V)
}
func position(v model.Opt) Field {
	if !v.Present {
		return Nil
	}
	// Divide before negating, so the entire int64 domain is representable.
	whole, part := v.V/100, v.V%100
	sign := ""
	if v.V < 0 {
		sign = "-"
		whole = -whole
		part = -part
	}
	return Field(fmt.Sprintf("%s%d.%02d", sign, whole, part))
}

// Actor writes the four common unit fields, with no name normalization.
func Actor(a model.Actor) []Field {
	name := Nil
	if a.Name != "" {
		name = Text(a.Name)
	}
	return []Field{unit(a.ID), name, Field(fmt.Sprintf("0x%x", a.Flags)), "0x0"}
}

// Advanced writes the 19-field advanced block; unspecified numbers are nil.
func Advanced(e model.Event) []Field {
	return []Field{unit(e.InfoUnit), unit(e.InfoOwnerID), optional(e.HP), optional(e.HPMax),
		Nil, Nil, Nil, Nil, Nil, Nil, optional(e.PowerType), optional(e.Power), optional(e.PowerMax), Nil, position(e.X), position(e.Y), Nil, Nil, Nil}
}

// Combatant writes fields after COMBATANT_INFO. Stats are the 24 numeric
// fields (faction through the unclassified field 25). Opaque gear extras remain
// wire tokens: their meaning is deliberately not invented here.
func Combatant(c model.Combatant) []Field {
	fields := []Field{unit(c.PlayerID)}
	for i := 0; i < 24; i++ {
		var v int64
		if i < len(c.Stats) {
			v = c.Stats[i]
		}
		fields = append(fields, Number(v))
	}
	var talents, gear, auras []Field
	for _, t := range c.Talents {
		talents = append(talents, Tuple(Number(t[0]), Number(t[1]), Number(t[2])))
	}
	for _, g := range c.Gear {
		extra := []Field{Tuple(Number(0), Number(0), Number(0)), List(), Tuple()}
		for i, s := range g.Extra {
			if i < len(extra) {
				extra[i] = Field(s)
			}
		}
		gear = append(gear, Tuple(append([]Field{Number(g.ItemID), Number(g.ItemLevel)}, extra...)...))
	}
	for _, a := range c.Auras {
		auras = append(auras, unit(a.CasterID), Number(a.SpellID), Number(a.Stacks))
	}
	return append(fields, List(talents...), Tuple(Number(0), Number(0), Number(0), Number(0)), List(gear...), List(auras...), Number(1), Number(0), Number(0), Number(0))
}
