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
	TankMinBossSwingShare                   float64
	HealerRequiresNotTank                   bool
	HealerMinHealingDamageRatio             float64
	HealerStrictlyGreater                   bool
	BossUnitSelection                       string
	InstanceKind                            func(instanceID, groupSize int64) (model.RunKind, bool)
	DetailsFor                              func(model.Fight) bool // nil selects all; false keeps only index metadata
	BossUnits                               func(encounterID int64) []CreatureSelector
	Profiles                                []SpecProfile
	InferSpecs                              func(casts []model.Event, profiles []SpecProfile) map[string]string
}

// Result retains per-file metadata beside runs; it never names a local path.
type Result struct {
	Runs   []Run
	Source Source
}

// Run is a segmentation result, not a night-score input. Conversion to the two
// model types is checked by kind. Unknown runs remain visible but unscored.
type Run struct {
	Kind           model.RunKind
	InstanceID     int64
	Listed         bool
	NightID        string
	StartMs, EndMs int64
	OffsetH        int8
	Partial        bool
	Logger         model.Actor
	Roster         []model.Actor
	Fights         []model.Fight
	Details        []Detail // same order as Fights; model intentionally unchanged
	Build          string
	LogVersion     int64
	Advanced       bool
	Source         model.Source
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
	var segment []model.Event
	var header []string
	instance, difficulty := int64(0), int64(0)
	enabled, knownZone := true, false
	inferred := false
	flush := func(end int64, cut bool) {
		if len(segment) == 0 {
			return
		}
		r := splitRun(segment, instance, difficulty, end, cut, p)
		readHeader(&r, header)
		describe(&r, segment, p)
		if len(r.Fights) > 0 {
			result.Runs = append(result.Runs, r)
		}
		segment = nil
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
			if !knownZone || e.Zone.InstanceID != instance {
				flush(e.TimeMs, false)
				instance, difficulty = e.Zone.InstanceID, e.Zone.Difficulty
				knownZone = true
				enabled = instance != 0
			}
			continue
		}
		if !enabled {
			continue
		}
		if !knownZone && !inferred && e.Kind == model.EncounterStart && e.Enc != nil && e.Enc.InstanceID.Present {
			instance = e.Enc.InstanceID.V
			inferred = true
		}
		segment = append(segment, e)
	}
	if len(segment) > 0 {
		flush(segment[len(segment)-1].TimeMs, true)
	}
	return result, nil
}

func splitRun(events []model.Event, instance, difficulty, end int64, cut bool, p Params) Run {
	r := Run{InstanceID: instance, StartMs: events[0].TimeMs, OffsetH: events[0].OffsetH, EndMs: end, Partial: cut}
	var f *model.Fight
	ended := false
	lastCombat := int64(0)
	anchor, position := "start", 0
	finish := func() {
		if f == nil {
			return
		}
		if f.Boss && !ended {
			f.Partial = true
		}
		if f.Boss && !f.Partial {
			f.ID = bossID(*f)
			anchor, position = f.ID, 0
		} else {
			position++
			f.ID = anchor + ":" + strconv.Itoa(position)
		}
		if f.Partial {
			r.Partial = true
		}
		r.Fights = append(r.Fights, *f)
		f = nil
		ended = false
	}
	for _, e := range events {
		start := e.Kind == model.EncounterStart && e.Enc != nil
		stop := e.Kind == model.EncounterEnd && e.Enc != nil
		if start {
			finish()
		}
		if f != nil && ended && e.TimeMs-f.EndMs > p.TailMs {
			finish()
		}
		if f != nil && !f.Boss && e.TimeMs-lastCombat > p.PullGapMs {
			finish()
		}
		if start {
			f = &model.Fight{Boss: true, EncounterID: e.Enc.ID, Name: e.Enc.Name, InstanceID: e.Enc.InstanceID.V, DifficultyID: e.Enc.Difficulty, GroupSize: int(e.Enc.GroupSize), StartMs: e.TimeMs}
		}
		if stop && (f == nil || !f.Boss || f.EncounterID != e.Enc.ID) {
			// Preserve an orphan end as an incomplete boss, never infer a start.
			finish()
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
			f.EndMs = e.TimeMs
		}
		if combat(e) {
			lastCombat = e.TimeMs
		}
		if e.Kind == model.CombatantInfo && e.Info != nil {
			f.Combatants = append(f.Combatants, *e.Info)
		}
		if stop {
			f.Success, f.DurationMs = e.Enc.Success, e.Enc.FightMs
			ended = true
		}
	}
	if f != nil && !f.Boss && cut && end-lastCombat <= p.PullGapMs {
		f.Partial = true
	}
	finish()
	var size int64
	for _, f := range r.Fights {
		if int64(f.GroupSize) > size {
			size = int64(f.GroupSize)
		}
	}
	if p.InstanceKind != nil {
		r.Kind, r.Listed = p.InstanceKind(instance, size)
	}
	r.NightID = nightID(r.Fights)
	return r
}

func combat(e model.Event) bool {
	return e.Kind == model.Damage || e.Kind == model.Missed || e.Kind == model.Interrupt || e.Kind == model.Dispel
}

func validate(p Params) error {
	if p.PullGapMs < 0 || p.TailMs < 0 || p.LowConfidenceFightMs < 0 || p.InstanceKind == nil || p.BossUnitSelection != "highest_max_health_damaged_creature" {
		return errors.New("fights: invalid window, instance classifier or boss selection")
	}
	if math.IsNaN(p.TankMinBossSwingShare) || math.IsInf(p.TankMinBossSwingShare, 0) || p.TankMinBossSwingShare < 0 || p.TankMinBossSwingShare > 1 || math.IsNaN(p.HealerMinHealingDamageRatio) || math.IsInf(p.HealerMinHealingDamageRatio, 0) || p.HealerMinHealingDamageRatio < 0 {
		return errors.New("fights: invalid role ratio")
	}
	return nil
}

func digest(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
func number(v int64) string  { return strconv.FormatInt(v, 10) }
func bossID(f model.Fight) string {
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
	return digest(strings.Join(append(fields, players...), "\n"))
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
