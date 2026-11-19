// Package modeltest builds model values for tests, so engines are tested without a log.
// Ids are derived from the name with a fixed hash: the same name gives the same id in every test.
package modeltest

import (
	"fmt"
	"hash/fnv"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func nameHash(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	return fmt.Sprintf("%08X", h.Sum32())
}

// Player is a player actor; Name is kept byte for byte.
func Player(name string) model.Actor {
	return model.Actor{ID: "Player-0-" + nameHash(name), Name: name}
}

// Creature is an NPC actor with its npc id in the GUID.
func Creature(name string, npcID int64) model.Actor {
	return model.Actor{ID: fmt.Sprintf("Creature-0-0-0-0-%d-%s", npcID, nameHash(name)), Name: name}
}

// Pet is a pet actor owned by owner.
func Pet(name string, owner model.Actor) model.Actor {
	return model.Actor{ID: "Pet-0-" + nameHash(owner.ID+"/"+name), Name: name, OwnerID: owner.ID}
}

// EventBuilder builds one event.
type EventBuilder struct{ e model.Event }

// At starts an event at the given local time in ms.
func At(ms int64) *EventBuilder { return &EventBuilder{e: model.Event{TimeMs: ms, Kind: model.Other}} }

// Src sets the source actor.
func (b *EventBuilder) Src(a model.Actor) *EventBuilder { b.e.Src = a; return b }

// Dst sets the destination actor.
func (b *EventBuilder) Dst(a model.Actor) *EventBuilder { b.e.Dst = a; return b }

// Damage makes a damage event; overkill -1 is the log's "not a killing hit".
func (b *EventBuilder) Damage(amount, overkill int64) *EventBuilder {
	b.e.Kind = model.Damage
	b.e.Amount, b.e.Overkill = model.Some(amount), model.Some(overkill)
	return b
}

// Landed marks a swing in the landed stream (the one totals read).
func (b *EventBuilder) Landed() *EventBuilder { b.e.Stream = model.SwingLanded; return b }

// Died makes a UNIT_DIED event with the given last field (0 is a death, 1 is feign death).
func (b *EventBuilder) Died(flag int64) *EventBuilder {
	b.e.Kind = model.UnitDied
	b.e.DiedFlag = model.Some(flag)
	return b
}

// Spell sets the spell id and name.
func (b *EventBuilder) Spell(id int64, name string) *EventBuilder {
	b.e.SpellID, b.e.SpellName = id, name
	return b
}

// Pos sets the line's byte position and length (with its line ending).
func (b *EventBuilder) Pos(offset, length int64) *EventBuilder {
	b.e.Offset, b.e.Len = offset, length
	return b
}

// Heal makes a heal event.
func (b *EventBuilder) Heal(amount, overheal int64) *EventBuilder {
	b.e.Kind = model.Heal
	b.e.Amount, b.e.Overheal = model.Some(amount), model.Some(overheal)
	return b
}

// Miss makes a missed event with the miss word as logged (e.g. "DODGE", "ABSORB").
func (b *EventBuilder) Miss(missType string) *EventBuilder {
	b.e.Kind, b.e.MissType = model.Missed, missType
	return b
}

// Aura makes an aura event of kind k (AuraApplied, AuraRemoved, ...) with auraType "BUFF" or "DEBUFF".
func (b *EventBuilder) Aura(k model.Kind, auraType string) *EventBuilder {
	b.e.Kind, b.e.AuraType = k, auraType
	return b
}

// Owner sets the advanced block's owner field.
func (b *EventBuilder) Owner(id string) *EventBuilder { b.e.InfoOwnerID = id; return b }

// Zone makes a ZONE_CHANGE event.
func (b *EventBuilder) Zone(instanceID int64, name string, difficulty int64) *EventBuilder {
	b.e.Kind = model.ZoneChange
	b.e.Zone = &model.Zone{InstanceID: instanceID, Name: name, Difficulty: difficulty}
	return b
}

// Info makes a COMBATANT_INFO event.
func (b *EventBuilder) Info(c model.Combatant) *EventBuilder {
	b.e.Kind = model.CombatantInfo
	b.e.Info = &c
	return b
}

// Build returns the event.
func (b *EventBuilder) Build() model.Event { return b.e }

// Encounter returns an ENCOUNTER_START at startMs and an ENCOUNTER_END at endMs for enc, each carrying only
// its own line's fields: the instance id on the start line; success and the fight length on the end line.
func Encounter(enc model.Encounter, startMs, endMs int64) (start, end model.Event) {
	s, e := enc, enc
	s.Success, s.FightMs = model.Opt{}, model.Opt{}
	e.InstanceID = model.Opt{}
	return model.Event{TimeMs: startMs, Kind: model.EncounterStart, Enc: &s},
		model.Event{TimeMs: endMs, Kind: model.EncounterEnd, Enc: &e}
}

// Fight builds a fight from enc and its events: start and end from the first and last event, the boss fields
// from enc (an encounter id above 0 is a boss fight), DurationMs from the server's fight length only.
func Fight(id string, enc model.Encounter, events ...model.Event) model.Fight {
	f := model.Fight{ID: id, EncounterID: enc.ID, Name: enc.Name, Boss: enc.ID > 0,
		InstanceID: enc.InstanceID.V, DifficultyID: enc.Difficulty, GroupSize: int(enc.GroupSize),
		Success: enc.Success, DurationMs: enc.FightMs, Events: events}
	if len(events) > 0 {
		f.StartMs, f.EndMs = events[0].TimeMs, events[len(events)-1].TimeMs
	}
	return f
}
