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
	HP, HPMax                  Opt        // of InfoUnit
	InfoUnit                   string     // the unit id the advanced block describes
	X, Y                       Opt        // fixed point, 1/100 yard
	DiedFlag                   Opt        // UNIT_DIED's last field; only V==0 is a death
	ExtraSpellID               Opt        // the interrupted, dispelled or absorbing spell
	Offset, Len                int64      // byte position of the line in the source, and its length with the line ending
	Power, PowerMax            Opt        // of InfoUnit, from the advanced block (model addendum 2)
	PowerType                  Opt        // the power type number as logged; Opt because 0 (mana) is a real type
	Overheal                   Opt        // heal lines only
	MissType                   string     // the miss word as logged (ABSORB, DODGE, ...); "" on other kinds
	AuraType                   string     // BUFF, DEBUFF or ""
	InfoOwnerID                string     // the owner field of the advanced block; "" when not filled
	Enc                        *Encounter // EncounterStart and EncounterEnd only
	Zone                       *Zone      // ZoneChange only
	Info                       *Combatant // CombatantInfo only
	Raw                        []string   // Other, CombatantInfo and Header only
	Caster                     Actor      // Absorb only: the shield\'s caster (Src is the attacker, Dst the shielded unit)
}

// Encounter is what an ENCOUNTER_START or ENCOUNTER_END line carries.
type Encounter struct {
	ID         int64
	Name       string
	Difficulty int64
	GroupSize  int64
	InstanceID Opt // on the start line only
	Success    Opt // on the end line only
	FightMs    Opt // on the end line only: the server's fight length
}

// Zone is what a ZONE_CHANGE line carries.
type Zone struct {
	InstanceID int64 // 0 is the open world
	Name       string
	Difficulty int64
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
	Boss           bool
	InstanceID     int64 // from ENCOUNTER_START; 0 means open world or no start line
	DifficultyID   int64
	DurationMs     Opt // the server's fight length; never EndMs minus StartMs
	Players        []Actor
	Combatants     []Combatant
	Owners         map[string]string // pet or guardian id -> owner id
	Unowned        []string          // pet or guardian ids whose owner is not known
	BossUnits      []string          // unit ids
	Roles          map[string]string // player id -> "tank", "healer" or "dps"
	Specs          map[string]string // player id -> spec id "<CLASS>.<spec>" (PLAN 4A row 16); missing means unknown
	Classes        map[string]string // player id -> class token, e.g. "PALADIN"; missing means unknown. Same for every fight of a run
	Offset, Len    int64             // the fight's byte range in the source log: first line's Offset to last line's end
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
	Player     Actor
	TimeMs     int64
	FightID    string
	Cause      string
	Evidence   []int64
	Confidence string   // "high", "medium" or "low" (PLAN 4A row 3)
	OwnerKind  string   // "player", "healers", "nobody" or "undecided" (4A row 4)
	Owners     []string // names as logged: the one player, or every healer charged; empty otherwise
	// SuggestedCause: for Cause "unresolved", one of the six other causes or "" (written as null); for any
	// other cause always "" (the field is absent from the file). 4A row 5.
	SuggestedCause string
}

// Wipe is R6's wipe summary for one fight (REQ-DTH-06).
type Wipe struct {
	FightID    string
	FirstDeath int   // index into the fight's deaths
	Following  []int // deaths inside the config window
}

// Source is the raw log a night was parsed from (4A row 19).
type Source struct {
	SHA256 string
	Bytes  int64
}

// Evidence is one part's evidence in one fight (4A row 9).
type Evidence struct {
	Text   string
	Counts map[string]int64
	AtMs   []int64
}

// Part is one scored part. Scores are integers so rounding happens once, in score.
type Part struct {
	Name     string
	Tenths   Opt // score times 10; unset means not measured (4A row 7)
	Evidence Evidence
}

// FightScore is R7's output for one player in one fight.
type FightScore struct {
	FightID  string
	PlayerID string
	Role     string
	Tenths   Opt // unset means "not enough data" (4A row 15)
	Coverage int // hundredths, 0 to 100
	Parts    []Part
	Flags    []string // the closed list of 4A row 8
}

// NightScore is R7's output for one player over a night or run.
type NightScore struct {
	PlayerID string
	Tenths   Opt
	Coverage int
	Rank     Opt    // unset for "not enough data" and in a dungeon run (4A rows 6, 13)
	Baseline string // which baseline was used (REQ-OFF-01)
	Fights   []FightScore
}

// PrepReport is one member's prep report (PLAN contract 3, DESIGN 10.1): written by prep, read by score.
// It carries everything the addon sends, because bag contents cannot be recovered after the night.
type PrepReport struct {
	Protocol, Level       int64
	AddonVersion, Name    string
	Class                 string
	PlayedAtLevelS        Opt              // unset means unreadable
	Consumables           []PrepConsumable // in bags; empty with ConsumablesUnreadable set means unreadable
	ConsumablesUnreadable bool
	BetterGear            []PrepGear // better items in bags, one row per slot
	GearChecked           bool       // false: "not checked" (Coach or its data absent)
	GearUnreadable        bool       // checked, but the answer could not be read
	NotScored             []int64    // checked slots whose worn item could not be scored (Forever customs); listed, not blanked
	Reason                string     // "join", "ready" or "encounter"
	SenderClock, Stamped  int64
}

// PrepConsumable is one consumable in bags: its category, item id and count.
type PrepConsumable struct {
	Category      string
	ItemID, Count int64
}

// PrepGear is one better-gear slot: the worn item id and the better item id in bags.
type PrepGear struct {
	Slot, WornID, BagID int64
}

// RaidNight holds runs of kind RunRaid only.
type RaidNight struct {
	Fights     []Fight
	Roster     []Actor
	Partial    bool
	Logger     Actor // the player whose lines carry the "mine" flag; empty if none
	StartMs    int64
	OffsetH    int8
	Build      string // from the header line
	LogVersion int64
	Advanced   bool // advanced logging on, by field count
	Source     Source
}

// DungeonRun is one run of kind RunDungeon.
type DungeonRun struct {
	Fights     []Fight
	Roster     []Actor
	InstanceID int64
	Partial    bool
	Logger     Actor // the player whose lines carry the "mine" flag; empty if none
	StartMs    int64
	OffsetH    int8
	Build      string // from the header line
	LogVersion int64
	Advanced   bool // advanced logging on, by field count
	Source     Source
}
