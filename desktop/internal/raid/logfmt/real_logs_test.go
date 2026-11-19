package logfmt_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// This test never embeds or prints filenames, actors, identifiers or raw lines.
// CI must not set RELAX_RAID_REAL_LOGS. The folder is supplied by the local user.
func TestRealLogsOptInCountsOnly(t *testing.T) {
	dir := os.Getenv("RELAX_RAID_REAL_LOGS")
	if dir == "" {
		t.Skip("private real logs are opt-in")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil || len(paths) != 2 {
		t.Fatalf("expected 2 log files; found %d", len(paths))
	}
	sort.Strings(paths)
	expected := []map[string]int64{
		{"lines": 85552, "headers": 1, "ENCOUNTER_START": 13, "ENCOUNTER_END": 13, "success": 13, "COMBATANT_INFO": 65, "SWING_DAMAGE": 6597, "SWING_DAMAGE_LANDED": 6851, "SPELL_ABSORBED": 1458, "SPELL_CAST_FAILED": 2670, "UNIT_DIED": 421, "ZONE_CHANGE": 1, "MAP_CHANGE": 0, "ENVIRONMENTAL_DAMAGE": 0, "skipped": 0},
		{"lines": 95614, "headers": 1, "ENCOUNTER_START": 13, "ENCOUNTER_END": 13, "success": 13, "COMBATANT_INFO": 65, "SWING_DAMAGE": 6427, "SWING_DAMAGE_LANDED": 6622, "SPELL_ABSORBED": 1423, "SPELL_CAST_FAILED": 8176, "UNIT_DIED": 435, "ZONE_CHANGE": 3, "MAP_CHANGE": 4, "ENVIRONMENTAL_DAMAGE": 12, "skipped": 0},
	}
	for i, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("log %d open failed", i+1)
		}
		var r logfmt.ParseReport
		var success, rawViolations, absorbs, casters int64
		for e := range logfmt.Decode(f, &r) {
			if e.Kind == model.Absorb {
				absorbs++
				if e.Caster.ID != "" && e.Caster.Name != "" {
					casters++
				}
			}
			if e.Kind == model.EncounterEnd && e.Enc != nil && e.Enc.Success == model.Some(1) {
				success++
			}
			if e.Kind != model.Other && e.Kind != model.CombatantInfo && e.Kind != model.Header && len(e.Raw) != 0 {
				rawViolations++
			}
		}
		closeErr := f.Close()
		if r.ReadError != nil || closeErr != nil {
			t.Errorf("log %d read/close failed", i+1)
		}
		sum := r.Parsed + r.HeadersSeen
		var skipped int64
		for _, n := range r.Skipped {
			skipped += n
		}
		sum += skipped
		if sum != r.LinesRead || rawViolations != 0 {
			t.Errorf("log %d accounting/raw violations=%d", i+1, rawViolations)
		}
		var unknownLines int64
		for _, n := range r.UnknownEvents {
			unknownLines += n
		}
		absorbLines := r.EventCounts["SPELL_ABSORBED"]
		fmt.Printf("REAL_LOG %d absorbCasterSet=%d absorbDecoded=%d absorbLines=%d invalidFlagsSkipped=%d nameNULBytes=%d invalidUTF8Names=%d unknownBuckets=%d unknownOther=%d unknownLines=%d\n", i+1, casters, absorbs, absorbLines, r.Skipped[logfmt.InvalidFlags], r.NameNULBytes, r.InvalidUTF8Names, len(r.UnknownEvents), r.UnknownEvents["other"], unknownLines)
		if casters != absorbLines || absorbs != absorbLines {
			t.Errorf("log %d absorb caster count differs: set=%d decoded=%d total=%d", i+1, casters, absorbs, absorbLines)
		}
		if n := r.Skipped[logfmt.InvalidFlags]; n != 0 {
			t.Errorf("log %d skipped=%d reason=%s", i+1, n, logfmt.InvalidFlags)
		}
		if r.NameNULBytes != 0 || r.InvalidUTF8Names != 0 || unknownLines != 0 {
			t.Errorf("log %d name/unknown counts differ from clean reference", i+1)
		}
		measured := r.EventCounts
		measured["lines"] = r.LinesRead
		measured["headers"] = r.HeadersSeen
		measured["success"] = success
		measured["skipped"] = skipped
		var keys []string
		for key := range expected[i] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Printf("REAL_LOG %d %s measured=%d expected=%d\n", i+1, key, measured[key], expected[i][key])
			if measured[key] != expected[i][key] {
				t.Errorf("log %d %s count differs: %d != %d", i+1, key, measured[key], expected[i][key])
			}
		}
		var diagnostics []string
		for key, n := range r.SkippedLayouts {
			diagnostics = append(diagnostics, fmt.Sprintf("SKIPPED %s fields=%d count=%d reason=%s", key.Name, key.Fields, n, key.Reason))
		}
		sort.Strings(diagnostics)
		for _, s := range diagnostics {
			fmt.Println(s)
		}
	}
}
