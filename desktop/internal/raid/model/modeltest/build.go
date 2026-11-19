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

// Power sets InfoUnit's current and maximum power and the power type as logged.
func (b *EventBuilder) Power(cur, max, powerType int64) *EventBuilder {
	b.e.Power, b.e.PowerMax, b.e.PowerType = model.Some(cur), model.Some(max), model.Some(powerType)
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
// For the fight's other fields use NewFight.
func Fight(id string, enc model.Encounter, events ...model.Event) model.Fight {
	f := model.Fight{ID: id, EncounterID: enc.ID, Name: enc.Name, Boss: enc.ID > 0,
		InstanceID: enc.InstanceID.V, DifficultyID: enc.Difficulty, GroupSize: int(enc.GroupSize),
		Success: enc.Success, DurationMs: enc.FightMs, Events: events}
	if len(events) > 0 {
		f.StartMs, f.EndMs = events[0].TimeMs, events[len(events)-1].TimeMs
	}
	return f
}

// FightBuilder sets the fields the fights engine derives (players, owners, roles, ...).
type FightBuilder struct{ f model.Fight }

// NewFight starts from Fight(id, enc, events...).
func NewFight(id string, enc model.Encounter, events ...model.Event) *FightBuilder {
	return &FightBuilder{f: Fight(id, enc, events...)}
}

// Players appends players.
func (b *FightBuilder) Players(a ...model.Actor) *FightBuilder {
	b.f.Players = append(b.f.Players, a...)
	return b
}

// Combatants appends COMBATANT_INFO values.
func (b *FightBuilder) Combatants(c ...model.Combatant) *FightBuilder {
	b.f.Combatants = append(b.f.Combatants, c...)
	return b
}

// Owner records that pet (or guardian) belongs to owner.
func (b *FightBuilder) Owner(pet, owner model.Actor) *FightBuilder {
	if b.f.Owners == nil {
		b.f.Owners = map[string]string{}
	}
	b.f.Owners[pet.ID] = owner.ID
	return b
}

// Unowned appends pet or guardian ids whose owner is not known.
func (b *FightBuilder) Unowned(ids ...string) *FightBuilder {
	b.f.Unowned = append(b.f.Unowned, ids...)
	return b
}

// BossUnits appends boss unit ids.
func (b *FightBuilder) BossUnits(a ...model.Actor) *FightBuilder {
	for _, u := range a {
		b.f.BossUnits = append(b.f.BossUnits, u.ID)
	}
	return b
}

// Role sets a player's role: "tank", "healer" or "dps".
func (b *FightBuilder) Role(p model.Actor, role string) *FightBuilder {
	if b.f.Roles == nil {
		b.f.Roles = map[string]string{}
	}
	b.f.Roles[p.ID] = role
	return b
}

// Spec sets a player's spec id, e.g. "PALADIN.holy".
func (b *FightBuilder) Spec(p model.Actor, spec string) *FightBuilder {
	if b.f.Specs == nil {
		b.f.Specs = map[string]string{}
	}
	b.f.Specs[p.ID] = spec
	return b
}

// Range sets the fight's byte range from its first and last events (Offset to the last line's end).
func (b *FightBuilder) Range() *FightBuilder {
	if n := len(b.f.Events); n > 0 {
		first, last := b.f.Events[0], b.f.Events[n-1]
		b.f.Offset, b.f.Len = first.Offset, last.Offset+last.Len-first.Offset
	}
	return b
}

// Partial marks the fight partial.
func (b *FightBuilder) Partial() *FightBuilder { b.f.Partial = true; return b }

// Build returns the fight.
func (b *FightBuilder) Build() model.Fight { return b.f }

// Run holds the fields RaidNight and DungeonRun share; Night and Dungeon turn it into either type.
type Run struct {
	fights   []model.Fight
	roster   []model.Actor
	partial  bool
	logger   model.Actor
	startMs  int64
	offsetH  int8
	build    string
	version  int64
	advanced bool
	source   model.Source
}

// NewRun starts a run from its fights; the start is the first fight's start.
func NewRun(fights ...model.Fight) *Run {
	r := &Run{fights: fights}
	if len(fights) > 0 {
		r.startMs = fights[0].StartMs
	}
	return r
}

// Roster appends players to the roster.
func (r *Run) Roster(a ...model.Actor) *Run { r.roster = append(r.roster, a...); return r }

// Partial marks the run partial.
func (r *Run) Partial() *Run { r.partial = true; return r }

// Logger sets the logging player.
func (r *Run) Logger(a model.Actor) *Run { r.logger = a; return r }

// Start sets the start time and the writer's UTC offset in hours.
func (r *Run) Start(ms int64, offsetH int8) *Run { r.startMs, r.offsetH = ms, offsetH; return r }

// Header sets what the header line carries: build, log version, advanced logging.
func (r *Run) Header(build string, logVersion int64, advanced bool) *Run {
	r.build, r.version, r.advanced = build, logVersion, advanced
	return r
}

// Source sets the raw log's hash and size.
func (r *Run) Source(sha256 string, bytes int64) *Run {
	r.source = model.Source{SHA256: sha256, Bytes: bytes}
	return r
}

// Night returns the run as a raid night.
func (r *Run) Night() model.RaidNight {
	return model.RaidNight{Fights: r.fights, Roster: r.roster, Partial: r.partial, Logger: r.logger,
		StartMs: r.startMs, OffsetH: r.offsetH, Build: r.build, LogVersion: r.version, Advanced: r.advanced, Source: r.source}
}

// Dungeon returns the run as a dungeon run in instanceID.
func (r *Run) Dungeon(instanceID int64) model.DungeonRun {
	return model.DungeonRun{Fights: r.fights, Roster: r.roster, InstanceID: instanceID, Partial: r.partial, Logger: r.logger,
		StartMs: r.startMs, OffsetH: r.offsetH, Build: r.build, LogVersion: r.version, Advanced: r.advanced, Source: r.source}
}

// DeathBuilder builds a classified death.
type DeathBuilder struct{ d model.Death }

// DeathOf starts a death of p at ms in fight with cause.
func DeathOf(p model.Actor, fightID string, ms int64, cause string) *DeathBuilder {
	return &DeathBuilder{d: model.Death{Player: p, FightID: fightID, TimeMs: ms, Cause: cause}}
}

// Evidence appends evidence times.
func (b *DeathBuilder) Evidence(ms ...int64) *DeathBuilder {
	b.d.Evidence = append(b.d.Evidence, ms...)
	return b
}

// Confidence sets "high", "medium" or "low".
func (b *DeathBuilder) Confidence(c string) *DeathBuilder { b.d.Confidence = c; return b }

// Owner sets the owner kind and the names charged.
func (b *DeathBuilder) Owner(kind string, names ...string) *DeathBuilder {
	b.d.OwnerKind, b.d.Owners = kind, names
	return b
}

// Suggest sets the suggested cause of an unresolved death.
func (b *DeathBuilder) Suggest(cause string) *DeathBuilder { b.d.SuggestedCause = cause; return b }

// Build returns the death.
func (b *DeathBuilder) Build() model.Death { return b.d }
