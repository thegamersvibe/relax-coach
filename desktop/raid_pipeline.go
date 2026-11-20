package main

import (
	"errors"
	"io"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// The caller owns config deployment and file selection. All engine inputs here
// come from one read. No stored night, history, upload, or inferred defaults.
func readRaidPages(raw io.Reader, save []byte, c cfg.Config) ([]raidPageModel, error) {
	return readRaidPagesSelected(raw, save, c, "", -1)
}

func readRaidPagesSelected(raw io.Reader, save []byte, c cfg.Config, officerPath string, selected int) ([]raidPageModel, error) {
	p := c.Scoring.Fights
	params := fights.Params{PullGapMs: p.PullGapMs, TailMs: p.TailMs, LowConfidenceFightMs: c.Scoring.Confidence.LowConfidenceFightMs, ReentryGapMs: p.ReentryGapMs, TankMinBossSwingShare: p.TankMinBossSwingShare, TankMinTopRatio: p.TankMinTopRatio, HealerRequiresNotTank: p.HealerRequiresNotTank, HealerMinHealingDamageRatio: p.HealerMinHealingDamageRatio, HealerStrictlyGreater: p.HealerStrictlyGreater, BossUnitSelection: p.BossUnitSelection, InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true }, BossUnits: func(id int64) []fights.CreatureSelector {
		rule, _ := c.Encounter(id)
		out := []fights.CreatureSelector{}
		for _, u := range rule.BossUnits {
			out = append(out, fights.CreatureSelector{ID: u.ID, Name: u.Name, Source: u.Source})
		}
		return out
	}}
	var parsed logfmt.ParseReport
	result, err := fights.Read(raw, func(r io.Reader, emit func(model.Event)) error {
		for e := range logfmt.Decode(r, &parsed) {
			emit(e)
		}
		return parsed.ReadError
	}, params)
	if err != nil {
		return nil, err
	}
	pages := []raidPageModel{}
	for index, run := range result.Runs {
		prediction := ""
		if index == selected {
			prediction = officerPath
		}
		page, err := buildRaidPageWithOfficer(run, parsed, save, c, prediction)
		if err != nil {
			return nil, err
		}
		page.OutsideRuns = result.PlayerDeathsOutsideRuns
		page.MissingPulls = result.PlayerDeathsInRunsWithoutFight
		pages = append(pages, page)
	}
	return pages, nil
}

// Always reads this exact chosen path again. Neither path nor prediction is
// retained by App, written into SavedVariables, or stored in a night file.
func raidComparisonFile(path string, scores []model.NightScore, run fights.Run, c cfg.Config) (compare.Report, error) {
	if path == "" {
		return compare.Report{}, errors.New("choose an officer file path")
	}
	file, err := openRaidLog("", path)
	if err != nil {
		return compare.Report{}, err
	}
	defer file.Close()
	const maxBytes = 64 << 10
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return compare.Report{}, err
	}
	if len(data) > maxBytes {
		return compare.Report{}, errors.New("officer file exceeds input budget")
	}
	officer, err := compare.ParseOfficer(data)
	if err != nil {
		return compare.Report{}, err
	}
	return compare.Compare(scores, run.Roster, c, officer, run.Fights)
}
