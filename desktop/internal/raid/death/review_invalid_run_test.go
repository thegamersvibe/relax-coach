package death_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"reflect"
	"testing"
)

func TestReviewRequiresPositiveRunEnd(t *testing.T) {
	for _, end := range []int64{0, -1} {
		for _, f := range []model.Fight{{}, fight(died(end, victim))} {
			// Reflection lets the old three-result API fail behaviorally first.
			out := reflect.ValueOf(death.Review).Call([]reflect.Value{reflect.ValueOf(f), reflect.ValueOf(end), reflect.ValueOf(config())})
			if len(out) != 4 {
				t.Fatalf("runEnd=%d: Review must return an error, got %d results", end, len(out))
			}
			err, ok := out[3].Interface().(error)
			if !ok || err == nil || !out[0].IsNil() || !out[1].IsNil() || out[2].Int() != 0 {
				t.Fatalf("runEnd=%d: want error with no partial results", end)
			}
		}
	}
}
