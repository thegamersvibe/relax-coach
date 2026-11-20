package death_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"testing"
)

func review(t *testing.T, f model.Fight, end int64, c cfg.Config) ([]model.Death, []model.Wipe, int) {
	t.Helper()
	ds, ws, x, err := death.Review(f, end, c)
	if err != nil {
		t.Fatal(err)
	}
	return ds, ws, x
}

// Legacy time-predicate fixtures append earlier events after constructing a
// death. Make their intended monotonic log explicit without modifying the fixture
// or same-ms line order. Positional/rollback tests use raw Fight.Events instead.
func chronological(f model.Fight) model.Fight {
	f.Events = sorted(append([]model.Event(nil), f.Events...)...)
	return f
}
func forChronologicalVictim(t *testing.T, f model.Fight, c cfg.Config) model.Death {
	t.Helper()
	return forVictim(t, chronological(f), c)
}
