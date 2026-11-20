package score

import (
	"fmt"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// ValidatePart refuses missing or invalid metadata on a measured part. A file
// writer must not turn a forgotten coverage/baseline into a valid-looking zero.
func ValidatePart(p model.Part) error {
	if !p.Tenths.Present {
		return nil
	}
	if p.Coverage < 1 || p.Coverage > 100 {
		return fmt.Errorf("score: measured %s coverage outside 1..100", p.Name)
	}
	switch p.Baseline {
	case "", "own", "guild", "role":
	default:
		return fmt.Errorf("score: unknown %s baseline", p.Name)
	}
	if (p.Name == "output" || p.Name == "activity") && p.Baseline == "" {
		return fmt.Errorf("score: measured %s requires a baseline", p.Name)
	}
	return nil
}

// Validate is the output boundary until the result writer exists. Score invokes
// it before returning; future writers may use the same guard before publication.
func Validate(scores []model.NightScore) error {
	for _, n := range scores {
		for _, f := range n.Fights {
			for _, p := range f.Parts {
				if err := ValidatePart(p); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
