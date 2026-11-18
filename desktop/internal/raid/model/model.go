// Package model holds the plain types every Relax Raid package shares. It imports nothing outside the
// standard library and nothing from the raid tree, so engines (fights, death, score) can be tested with
// built values and never depend on the log format.
//
// Rules carried by these types (PLAN.md section 4, contracts 10 to 15):
//   - An actor's name is the string the log wrote, whole. Nothing in this package splits it.
//   - An optional number is an Opt; an absent value is never zero.
//   - Only a UNIT_DIED whose last field is 0 is a death (feign death writes 1).
//   - A swing names its stream; totals read the landed stream only.
//   - RaidNight and DungeonRun are unrelated types, so a night scorer cannot take a dungeon run.
package model

// Actor is a unit as the log writes it.
type Actor struct {
	ID      string // as logged, e.g. "Player-6940-0001ABCD"
	Name    string // as logged, whole. Never split. May be "".
	Flags   uint32
	OwnerID string // "" when unknown
}

// Kind is one value per event family.
type Kind uint8

const (
	Other Kind = iota
	Damage
	Missed
	Heal
	Absorb
	CastStart
	CastSuccess
	CastFailed
	AuraApplied
	AuraRemoved
	AuraRefresh
	AuraDose
	AuraBroken
	Interrupt
	Dispel
	Energize
	Summon
	Resurrect
	UnitDied
	PartyKill
	EncounterStart
	EncounterEnd
	ZoneChange
	CombatantInfo
	Header
	kindCount // not a kind: the number of kinds
)

var kindNames = [...]string{
	Other: "Other", Damage: "Damage", Missed: "Missed", Heal: "Heal", Absorb: "Absorb",
	CastStart: "CastStart", CastSuccess: "CastSuccess", CastFailed: "CastFailed",
	AuraApplied: "AuraApplied", AuraRemoved: "AuraRemoved", AuraRefresh: "AuraRefresh", AuraDose: "AuraDose",
	AuraBroken: "AuraBroken", Interrupt: "Interrupt", Dispel: "Dispel", Energize: "Energize", Summon: "Summon",
	Resurrect: "Resurrect", UnitDied: "UnitDied", PartyKill: "PartyKill", EncounterStart: "EncounterStart",
	EncounterEnd: "EncounterEnd", ZoneChange: "ZoneChange", CombatantInfo: "CombatantInfo", Header: "Header",
}

// Kinds lists every kind, in order.
func Kinds() []Kind {
	out := make([]Kind, 0, int(kindCount))
	for k := Kind(0); k < kindCount; k++ {
		out = append(out, k)
	}
	return out
}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "Kind(?)"
}

// Stream separates the two swing streams the log writes; they are not copies of each other.
type Stream uint8

const (
	StreamNone Stream = iota
	SwingSource
	SwingLanded
)

// Opt is an optional number; absent is never zero.
type Opt struct {
	V       int64
	Present bool
}

// Some returns a present value.
func Some(v int64) Opt { return Opt{V: v, Present: true} }

// Event is one decoded log line.
type Event struct {
	TimeMs                     int64 // local wall time in ms since the Unix epoch, as written
	OffsetH                    int8  // the writer's UTC offset in whole hours
	Line                       int64 // line number in the source file
	Kind                       Kind
	Stream                     Stream
	Src, Dst                   Actor
	SpellID                    int64
	SpellName                  string
	Amount, Overkill, Absorbed Opt
	Crit                       bool
	HP, HPMax                  Opt    // of InfoUnit
	InfoUnit                   string // the unit id the advanced block describes
	X, Y                       Opt    // fixed point, 1/100 yard
	DiedFlag                   Opt    // UNIT_DIED's last field; only V==0 is a death
	ExtraSpellID               Opt    // the interrupted, dispelled or absorbing spell
	Raw                        []string
}

// IsDeath reports a real death: UNIT_DIED with its last field present and 0.
func (e Event) IsDeath() bool {
	return e.Kind == UnitDied && e.DiedFlag.Present && e.DiedFlag.V == 0
}

// RunKind is the kind of a run.
type RunKind uint8

const (
	RunUnknown RunKind = iota
	RunRaid
	RunDungeon
)

// Fight is one pull, bounded by its encounter lines when it has them.
type Fight struct {
	ID             string // hash of content, set by fights; never a counter
	EncounterID    int64  // 0 for a trash pull
	Name           string
	StartMs, EndMs int64
	Success        Opt // from ENCOUNTER_END only
	Partial        bool
	GroupSize      int
	Events         []Event
}

// Combatant is a decoded COMBATANT_INFO line; fields not understood are kept raw.
type Combatant struct {
	PlayerID string
	Stats    []int64
	Talents  [][3]int64
	Gear     []GearSlot
	Auras    []Aura
	Raw      []string
}

// GearSlot is one worn item.
type GearSlot struct {
	ItemID, ItemLevel int64
	Extra             []string
}

// Aura is one aura present at the pull.
type Aura struct {
	CasterID        string
	SpellID, Stacks int64
}

// Death is one classified death.
type Death struct {
	Player   Actor
	TimeMs   int64
	FightID  string
	Cause    string
	Evidence []int64
}

// RaidNight holds runs of kind RunRaid only.
type RaidNight struct {
	Fights []Fight
	Roster []Actor
}

// DungeonRun is one run of kind RunDungeon.
type DungeonRun struct {
	Fights     []Fight
	Roster     []Actor
	InstanceID int64
}
