// Package fights splits decoded model events without knowing the log grammar.
// Callers supply all tuning and adapt their decoder/config at the application seam.
package fights

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"iter"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Params mirrors cfg.Scoring.Fights and Confidence without a dependency on cfg.
// No defaults are compiled into this package.
type Params struct {
	PullGapMs, TailMs, LowConfidenceFightMs int64
	ReentryGapMs                            int64
	TankMinBossSwingShare                   float64
	TankMinTopRatio                         float64
	HealerRequiresNotTank                   bool
	HealerMinHealingDamageRatio             float64
	HealerStrictlyGreater                   bool
	BossUnitSelection                       string
	InstanceKind                            func(instanceID, groupSize int64) (model.RunKind, bool)
	DetailsFor                              func(model.Fight) bool // nil keeps index only; explicitly select fights for detail
	BossUnits                               func(encounterID int64) []CreatureSelector
	Profiles                                []SpecProfile
	InferSpecs                              func(casts []model.Event, profiles []SpecProfile) map[string]string
}

// Result retains per-file metadata beside runs; it never names a local path.
type Result struct {
	Runs                   []Run
	Source                 Source
	DuplicateEncounterEnds int64
}

// Run is a segmentation result, not a night-score input. Conversion to the two
// model types is checked by kind. Unknown runs remain visible but unscored.
type Run struct {
	Kind                   model.RunKind
	InstanceID             int64
	Listed                 bool
	NightID                string
	StartMs, EndMs         int64
	OffsetH                int8
	Partial                bool
	Logger                 model.Actor
	Roster                 []model.Actor
	Fights                 []model.Fight
	Details                []Detail // same order as Fights; model intentionally unchanged
	Build                  string
	LogVersion             int64
	Advanced               bool
	Source                 model.Source
	DuplicateEncounterEnds int64
}

// Build consumes decoded events in file order. Wall time is used for windows,
// never for complete boss identity or the server's duration.
func Build(events []model.Event, p Params) (Result, error) {
	return collect(slices.Values(events), p)
}

func collect(events iter.Seq[model.Event], p Params) (Result, error) {
	if err := validate(p); err != nil {
		return Result{}, err
	}
	var result Result
	repeats := map[string]int{}
	var header []string
	instance, difficulty := int64(0), int64(0)
	enabled, knownZone, inferred := true, false, false
	pendingExit := false
	var exitMs, lastMs, lastCombat, encounterEndMs int64
	var r Run
	var f *model.Fight
	var facts []fightFacts
	actors := newActorIndex()
	initialized, ended := false, false
	seenEnds := map[int64]bool{}
	anchor, position := "start", 0
	finish := func() {
		if f == nil {
			return
		}
		if f.Boss && !ended {
			f.Partial = true
		}
		if f.Boss && !f.Partial {
			text := bossText(*f)
			repeats[text]++
			if repeats[text] > 1 {
				text += "\nrepeat " + strconv.Itoa(repeats[text])
			}
			f.ID = digest(text)
			anchor, position = f.ID, 0
		} else {
			position++
			f.ID = anchor + ":" + strconv.Itoa(position)
		}
		fact := collectFightFacts(*f)
		actors.project(f, fact.touched, actors.resolved())
		selectBosses(f, p)
		d := indexDetail(*f, p)
		fact.selected = p.DetailsFor != nil && p.DetailsFor(*f)
		if fact.selected {
			d = makeDetail(*f, p)
		} else {
			fact.bossEvidence = bossEvidence(*f)
			f.Events = nil
			d.UnscoredReason = "detailNotSelected"
		}
		f.Offset, f.Len, f.Specs = d.Offset, d.Len, d.Specs
		r.Fights = append(r.Fights, *f)
		r.Details = append(r.Details, d)
		facts = append(facts, fact)
		f, ended = nil, false
	}
	flush := func(end int64, cut bool) {
		if pendingExit {
			// Later outside lines (including headers/EOF) cannot move the exit.
			end, cut = exitMs, false
			pendingExit = false
		}
		if !initialized {
			return
		}
		if f != nil && !f.Boss && cut && end-lastCombat <= p.PullGapMs {
			f.Partial = true
		}
		finish()
		r.InstanceID, r.EndMs = instance, max(r.StartMs, end)
		var size int64
		for _, f := range r.Fights {
			if int64(f.GroupSize) > size {
				size = int64(f.GroupSize)
			}
		}
		r.Kind, r.Listed = p.InstanceKind(instance, size)
		r.NightID = nightID(r.Fights)
		readHeader(&r, header)
		actors.finish(&r, facts, p)
		result.DuplicateEncounterEnds += r.DuplicateEncounterEnds
		if len(r.Fights) > 0 {
			result.Runs = append(result.Runs, r)
		}
		r, facts, actors = Run{}, nil, newActorIndex()
		initialized, ended = false, false
		seenEnds = map[int64]bool{}
		anchor, position = "start", 0
	}
	for e := range events {
		if e.Kind == model.Header {
			flush(e.TimeMs, true)
			header = e.Raw
			inferred = false
			instance, difficulty, enabled, knownZone = 0, 0, true, false
			continue
		}
		if e.Kind == model.ZoneChange && e.Zone != nil {
			next := e.Zone.InstanceID
			if !enabled && knownZone && next == 0 {
				continue // repeated outside zones do not restart the re-entry clock
			}
			if pendingExit && next == instance && e.TimeMs-exitMs < p.ReentryGapMs {
				pendingExit, enabled = false, true
				continue
			}
			if enabled && knownZone && next == instance {
				continue
			}
			if enabled && instance != 0 && next == 0 && initialized {
				pendingExit, exitMs = true, e.TimeMs
				finish() // re-entry joins runs, never fights across an exit
			} else {
				flush(e.TimeMs, false)
				instance, difficulty = next, e.Zone.Difficulty
			}
			knownZone = true
			enabled = next != 0
			continue
		}
		if !enabled {
			continue
		}
		if !knownZone && !inferred && e.Kind == model.EncounterStart && e.Enc != nil && e.Enc.InstanceID.Present {
			instance = e.Enc.InstanceID.V
			inferred = true
		}
		if !initialized {
			r.StartMs, r.OffsetH, initialized = e.TimeMs, e.OffsetH, true
		}
		start := e.Kind == model.EncounterStart && e.Enc != nil
		stop := e.Kind == model.EncounterEnd && e.Enc != nil
		if start {
			delete(seenEnds, e.Enc.ID)
		}
		if stop {
			if seenEnds[e.Enc.ID] {
				r.DuplicateEncounterEnds++
				continue
			}
			seenEnds[e.Enc.ID] = true
		}
		lastMs = e.TimeMs
		actors.observe(e)
		if start {
			finish()
		}
		if f != nil && ended && e.TimeMs-encounterEndMs > p.TailMs {
			finish()
		}
		if f != nil && !f.Boss && e.TimeMs-lastCombat > p.PullGapMs {
			finish()
		}
		if start {
			f = &model.Fight{Boss: true, EncounterID: e.Enc.ID, Name: e.Enc.Name, InstanceID: e.Enc.InstanceID.V, DifficultyID: e.Enc.Difficulty, GroupSize: int(e.Enc.GroupSize), StartMs: e.TimeMs}
		}
		if stop && (f == nil || !f.Boss || f.EncounterID != e.Enc.ID) {
			finish()
			if len(r.Fights) == 0 {
				r.Partial = true
			}
			f = &model.Fight{Boss: true, Partial: true, EncounterID: e.Enc.ID, Name: e.Enc.Name, InstanceID: e.Enc.InstanceID.V, DifficultyID: e.Enc.Difficulty, GroupSize: int(e.Enc.GroupSize), StartMs: e.TimeMs}
		}
		if f == nil && combat(e) {
			partial := len(r.Fights) == 0 && e.TimeMs-r.StartMs <= p.PullGapMs
			f = &model.Fight{StartMs: e.TimeMs, InstanceID: instance, DifficultyID: difficulty, Partial: partial, Name: e.Dst.Name}
			if partial {
				r.Partial = true
			}
		}
		if f == nil {
			continue
		}
		f.Events = append(f.Events, e)
		if !ended {
			f.EndMs = max(f.StartMs, f.EndMs, e.TimeMs)
		}
		if combat(e) {
			lastCombat = e.TimeMs
		}
		if e.Kind == model.CombatantInfo && e.Info != nil {
			f.Combatants = append(f.Combatants, *e.Info)
		}
		if stop {
			// The END marker owns the tail; an earlier clock spike must not
			// extend it. Clamp display bounds only when END predates START.
			encounterEndMs = e.TimeMs
			f.EndMs = max(f.StartMs, encounterEndMs)
			f.Success, f.DurationMs = e.Enc.Success, e.Enc.FightMs
			ended = true
		}
	}
	flush(lastMs, true)
	return result, nil
}

func combat(e model.Event) bool {
	return e.Kind == model.Damage || e.Kind == model.Missed || e.Kind == model.Interrupt || e.Kind == model.Dispel
}

func validate(p Params) error {
	if p.PullGapMs < 0 || p.TailMs < 0 || p.LowConfidenceFightMs < 0 || p.ReentryGapMs <= 0 || p.InstanceKind == nil || p.BossUnitSelection != "highest_max_health_damaged_creature" {
		return errors.New("fights: invalid window, instance classifier or boss selection")
	}
	for _, ratio := range []float64{p.TankMinBossSwingShare, p.TankMinTopRatio} {
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0 || ratio > 1 {
			return errors.New("fights: invalid tank ratio")
		}
	}
	if math.IsNaN(p.HealerMinHealingDamageRatio) || math.IsInf(p.HealerMinHealingDamageRatio, 0) || p.HealerMinHealingDamageRatio < 0 {
		return errors.New("fights: invalid role ratio")
	}
	return nil
}

func digest(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
func number(v int64) string  { return strconv.FormatInt(v, 10) }
func bossText(f model.Fight) string {
	fields := []string{"boss", number(f.InstanceID), number(f.EncounterID), number(f.DifficultyID), number(int64(f.GroupSize)), optional(f.Success), optional(f.DurationMs)}
	ids := make(map[string]bool)
	for _, c := range f.Combatants {
		ids[c.PlayerID] = true
	}
	players := make([]string, 0, len(ids))
	for id := range ids {
		players = append(players, id)
	}
	sort.Strings(players)
	return strings.Join(append(fields, players...), "\n")
}
func optional(n model.Opt) string {
	if !n.Present {
		return "unknown"
	}
	return number(n.V)
}
func nightID(fs []model.Fight) string {
	var ids []string
	for _, f := range fs {
		if f.Boss && !f.Partial {
			ids = append(ids, f.ID)
		}
	}
	sort.Strings(ids)
	return digest(strings.Join(ids, "\n"))
}
