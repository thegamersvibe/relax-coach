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

// Build returns the event.
func (b *EventBuilder) Build() model.Event { return b.e }
