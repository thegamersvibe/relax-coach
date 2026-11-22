package main

// Retirement is deliberately separate from the old installer: no download,
// backup, restore, app placement or recursive account discovery is involved.
import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type retirementResult struct {
	Removed               []string
	Skipped               []string
	NoGame                bool
	SavedSettingsDeferred bool
}

func (r *retirementResult) skip(path string, err error) {
	r.Skipped = append(r.Skipped, fmt.Sprintf("skipped %s (linked, unreadable or unsafe path: %v)", path, err))
}

// A linked addon, including one containing a link/reparse point, is left
// entirely intact. Check all ancestors too. OpenRoot then bounds deletion to
// the already-open AddOns directory rather than trusting a later path lookup.
func retirementTree(path string) error {
	if err := appPlainDirectory(path); err != nil {
		return err
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || (!d.IsDir() && !d.Type().IsRegular()) {
			return fmt.Errorf("linked or special entry: %s", p)
		}
		return appRejectReparse(p)
	})
}

func retireGame(ctx context.Context, game string, running func(context.Context) (bool, error)) retirementResult {
	result := retirementResult{NoGame: game == ""}
	if game == "" {
		return result
	}
	game, err := filepath.Abs(game)
	if err != nil {
		result.skip(game, err)
		return result
	}
	if err = ctx.Err(); err != nil {
		result.skip(game, err)
		return result
	}
	if err = appPlainDirectory(game); err != nil {
		result.skip(game, err)
		return result
	}
	journey := retirementAddons(game, &result)
	observer := addonUpdater{running: running}
	open, err := observer.observeGame(ctx)
	if open || err != nil {
		result.SavedSettingsDeferred = true
		return result
	}
	retirementSaves(ctx, game, journey, &observer, &result)
	return result
}

func retirementAddons(game string, result *retirementResult) bool {
	addons := filepath.Join(game, "Interface", "AddOns")
	if err := appPlainDirectory(filepath.Dir(addons)); err != nil {
		if os.IsNotExist(err) {
			return retirementKnownJourney(game, result)
		}
		result.skip(filepath.Dir(addons), err)
		return false
	}
	if err := appPlainDirectory(addons); err != nil {
		if os.IsNotExist(err) {
			return retirementKnownJourney(game, result)
		}
		result.skip(addons, err)
		return false
	}
	root, err := os.OpenRoot(addons)
	if err != nil {
		result.skip(addons, err)
		return false
	}
	defer root.Close()
	journey := false
	for _, name := range []string{"ForeverCoach", "RelaxRaid", "RelaxQuestPlanner", "Journey"} {
		target := filepath.Join(addons, name)
		// Fixed literal names only; containment must hold before any deletion.
		if filepath.Dir(target) != addons || filepath.Base(target) != name {
			result.skip(target, fmt.Errorf("not a direct addon child"))
			continue
		}
		if _, err := root.Lstat(name); os.IsNotExist(err) {
			if name == "Journey" {
				journey = retirementKnownJourney(game, result)
			}
			continue
		}
		if err := retirementTree(target); err != nil {
			if name == "Journey" {
				retirementRecordJourney(game, false, result)
			}
			result.skip(target, err)
			continue
		}
		if name == "Journey" {
			ours, readable := addonJourneyTitle(target)
			if !readable || !ours {
				retirementRecordJourney(game, false, result)
				result.Skipped = append(result.Skipped, "skipped Journey (not identified as Relax Journey)")
				continue
			}
			journey = true
			retirementRecordJourney(game, true, result)
		}
		if err := root.RemoveAll(name); err != nil {
			result.skip(target, err)
			continue
		}
		result.Removed = append(result.Removed, filepath.Join(addons, name))
	}
	return journey
}
