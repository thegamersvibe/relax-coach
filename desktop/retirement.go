package main

// Retirement is deliberately separate from the old installer: no download,
// app placement or recursive account discovery is involved.
import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type retirementResult struct {
	Moved                 []string
	Skipped               []string
	NoGame                bool
	SavedSettingsDeferred bool
	RecoveryBlocked       bool
	journeyRetry          bool // Known-owned save still in place after a failed move.
}

func (r *retirementResult) skip(path string, err error) {
	r.Skipped = append(r.Skipped, fmt.Sprintf("could not move %s, left in place; will retry next start (linked, unreadable or unsafe path: %v)", path, err))
}

// A linked addon, including one containing a link/reparse point, is left
// entirely intact. Check all ancestors too. Moves resolve root-relative paths
// and revalidate their plain parents before the OS no-replace operation.
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
	// Every source (flag, saved config, discovery) must name the Forever game
	// folder itself, by the same check discovery uses; anything else is no game.
	if NormalizeGame(game) != game {
		result.NoGame = true
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
	quarantine, err := newRetirementQuarantine(game)
	if err != nil {
		result.skip(filepath.Join(game, retirementStorage), err)
		return result
	}
	defer quarantine.close()
	if err = quarantine.recover(); err != nil {
		result.RecoveryBlocked = true
		result.Skipped = append(result.Skipped, "a previous move could not be verified; nothing more was moved: "+err.Error())
		return result
	}
	journey := retirementAddons(game, quarantine, &result)
	observer := addonUpdater{running: running}
	open, err := observer.observeGame(ctx)
	if open || err != nil {
		result.SavedSettingsDeferred = true
		return result
	}
	retirementSaves(ctx, game, journey, &observer, quarantine, &result)
	if !result.SavedSettingsDeferred && !result.journeyRetry {
		// Cleanup finished: drop the ownership receipt so a later foreign Journey's
		// settings are never treated as ours.
		retirementRecordJourney(game, false, &result)
	}
	return result
}

func retirementAddons(game string, quarantine *retirementQuarantine, result *retirementResult) bool {
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
		// Fixed literal names only; containment must hold before any move.
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
		quarantine.move(target, filepath.Join("AddOns", name), result)
	}
	return journey
}
