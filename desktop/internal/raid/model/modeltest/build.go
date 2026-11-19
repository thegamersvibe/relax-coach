// Package modeltest builds model values for tests, so engines are tested without a log.
// Ids are derived from the name with a fixed hash: the same name gives the same id in every test.
package modeltest

import (
	"fmt"
	"hash/fnv"
	"reflect"

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

// Absorb makes a SPELL_ABSORBED event: Src attacks Dst, caster's shield (shieldID) absorbs amount.
func (b *EventBuilder) Absorb(caster model.Actor, shieldID, amount int64) *EventBuilder {
	b.e.Kind, b.e.Caster = model.Absorb, caster
	b.e.ExtraSpellID, b.e.Amount = model.Some(shieldID), model.Some(amount)
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

// Class sets a player's class token, e.g. "PALADIN".
func (b *FightBuilder) Class(p model.Actor, class string) *FightBuilder {
	if b.f.Classes == nil {
		b.f.Classes = map[string]string{}
	}
	b.f.Classes[p.ID] = class
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

// Wipe builds a wipe summary: the first death's index and the following deaths inside the window.
func Wipe(fightID string, first int, following ...int) model.Wipe {
	return model.Wipe{FightID: fightID, FirstDeath: first, Following: following}
}

// Evidence builds one part's evidence; counts are given as name, value pairs with any integer type.
// A key without a value, a non-string key or a non-integer value panics: a test mistake is never dropped.
func Evidence(text string, atMs []int64, counts ...any) model.Evidence {
	if len(counts)%2 != 0 {
		panic(fmt.Sprintf("modeltest.Evidence: key %v has no value", counts[len(counts)-1]))
	}
	ev := model.Evidence{Text: text, AtMs: atMs}
	for i := 0; i < len(counts); i += 2 {
		key, ok := counts[i].(string)
		if !ok {
			panic(fmt.Sprintf("modeltest.Evidence: key %v is not a string", counts[i]))
		}
		if ev.Counts == nil {
			ev.Counts = map[string]int64{}
		}
		ev.Counts[key] = toInt64(counts[i+1])
	}
	return ev
}

func toInt64(v any) int64 {
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return r.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(r.Uint())
	}
	panic(fmt.Sprintf("modeltest.Evidence: value %v (%T) is not an integer", v, v))
}

// Part builds a measured part (score in tenths).
func Part(name string, tenths int64, ev model.Evidence) model.Part {
	return model.Part{Name: name, Tenths: model.Some(tenths), Evidence: ev}
}

// Unmeasured builds a part that could not be measured (Tenths unset).
func Unmeasured(name string) model.Part { return model.Part{Name: name} }

// FightScoreBuilder builds one player's score in one fight.
type FightScoreBuilder struct{ s model.FightScore }

// FightScore starts a fight score; without Score it is "not enough data".
func FightScore(fightID string, p model.Actor, role string) *FightScoreBuilder {
	return &FightScoreBuilder{s: model.FightScore{FightID: fightID, PlayerID: p.ID, Role: role}}
}

// Score sets the score in tenths.
func (b *FightScoreBuilder) Score(tenths int64) *FightScoreBuilder {
	b.s.Tenths = model.Some(tenths)
	return b
}

// Coverage sets coverage in hundredths (0 to 100).
func (b *FightScoreBuilder) Coverage(h int) *FightScoreBuilder { b.s.Coverage = h; return b }

// Parts appends parts.
func (b *FightScoreBuilder) Parts(p ...model.Part) *FightScoreBuilder {
	b.s.Parts = append(b.s.Parts, p...)
	return b
}

// Flags appends flags from 4A row 8.
func (b *FightScoreBuilder) Flags(f ...string) *FightScoreBuilder {
	b.s.Flags = append(b.s.Flags, f...)
	return b
}

// Build returns the fight score.
func (b *FightScoreBuilder) Build() model.FightScore { return b.s }

// NightScoreBuilder builds one player's night (or run) score.
type NightScoreBuilder struct{ s model.NightScore }

// NightScore starts a night score; without Score it is "not enough data", without Rank it has no rank.
func NightScore(p model.Actor, baseline string) *NightScoreBuilder {
	return &NightScoreBuilder{s: model.NightScore{PlayerID: p.ID, Baseline: baseline}}
}

// Score sets the score in tenths.
func (b *NightScoreBuilder) Score(tenths int64) *NightScoreBuilder {
	b.s.Tenths = model.Some(tenths)
	return b
}

// Coverage sets coverage in hundredths.
func (b *NightScoreBuilder) Coverage(h int) *NightScoreBuilder { b.s.Coverage = h; return b }

// Rank sets the rank (raid nights only).
func (b *NightScoreBuilder) Rank(r int64) *NightScoreBuilder { b.s.Rank = model.Some(r); return b }

// Fights appends the player's fight scores.
func (b *NightScoreBuilder) Fights(f ...model.FightScore) *NightScoreBuilder {
	b.s.Fights = append(b.s.Fights, f...)
	return b
}

// Build returns the night score.
func (b *NightScoreBuilder) Build() model.NightScore { return b.s }

// PrepBuilder builds a prep report.
type PrepBuilder struct{ r model.PrepReport }

// Prep starts a prep report from a sender; gear is "not checked" until Gear is called.
func Prep(name, class string, level, protocol int64, addonVersion string) *PrepBuilder {
	return &PrepBuilder{r: model.PrepReport{Name: name, Class: class, Level: level, Protocol: protocol, AddonVersion: addonVersion}}
}

// Played sets the time played at this level in seconds.
func (b *PrepBuilder) Played(s int64) *PrepBuilder { b.r.PlayedAtLevelS = model.Some(s); return b }

// Consumable appends a consumable in bags.
func (b *PrepBuilder) Consumable(category string, itemID, count int64) *PrepBuilder {
	b.r.Consumables = append(b.r.Consumables, model.PrepConsumable{Category: category, ItemID: itemID, Count: count})
	return b
}

// ConsumablesUnreadable marks the bag read as failed.
func (b *PrepBuilder) ConsumablesUnreadable() *PrepBuilder {
	b.r.ConsumablesUnreadable = true
	return b
}

// Gear marks gear checked and appends better items in bags.
func (b *PrepBuilder) Gear(rows ...model.PrepGear) *PrepBuilder {
	b.r.GearChecked, b.r.BetterGear = true, append(b.r.BetterGear, rows...)
	return b
}

// GearUnreadable marks gear checked but unreadable.
func (b *PrepBuilder) GearUnreadable() *PrepBuilder {
	b.r.GearChecked, b.r.GearUnreadable = true, true
	return b
}

// NotScored marks gear checked and lists slots whose worn item could not be scored.
func (b *PrepBuilder) NotScored(slots ...int64) *PrepBuilder {
	b.r.GearChecked, b.r.NotScored = true, append(b.r.NotScored, slots...)
	return b
}

// Reason sets the send reason: "join", "ready" or "encounter".
func (b *PrepBuilder) Reason(r string) *PrepBuilder { b.r.Reason = r; return b }

// Clock sets the sender's clock and the time the report was stamped on receipt.
func (b *PrepBuilder) Clock(sender, stamped int64) *PrepBuilder {
	b.r.SenderClock, b.r.Stamped = sender, stamped
	return b
}

// Build returns the report.
func (b *PrepBuilder) Build() model.PrepReport { return b.r }
