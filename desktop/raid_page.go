package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Page-only projection. Never written as a night file or used as score input.
type raidPageModel struct {
	Parsed                                                  logfmt.ParseReport
	PrepSkipped                                             map[string]int
	Comparison                                              *compare.Report
	ComparisonError                                         string
	OutsideRuns, MissingPulls                               int64
	Deaths                                                  []raidDeathView
	Wipes                                                   []raidWipeView
	Excluded                                                int
	Unmatched                                               []model.PrepReport
	Warnings                                                []string
	Title, Date, Logger, Duration, ConfigVersion, LogStatus string
	Dungeon                                                 bool
	Bosses, WipePulls, Kills, Trash, PrepPlayers, Players   int
	Ranked, Provisional, Unscored                           []raidPlayerView
}
type raidPlayerView struct {
	Key                     int
	Name, Role, Score, Rank string
	Coverage                int
	Fights                  []raidFightView
	NightParts              []raidNightPartView
	NightReason             string
	Flags                   []string
}

func raidTenths(v model.Opt, absent string) string {
	if !v.Present {
		return absent
	}
	return fmt.Sprintf("%d.%d", v.V/10, v.V%10)
}
func buildRaidPage(run fights.Run, parsed logfmt.ParseReport, save []byte, c cfg.Config) (raidPageModel, error) {
	return buildRaidPageWithOfficer(run, parsed, save, c, "")
}

func buildRaidPageWithOfficer(run fights.Run, parsed logfmt.ParseReport, save []byte, c cfg.Config, officerPath string) (raidPageModel, error) {
	p := raidPageModel{Parsed: parsed, Title: fmt.Sprintf("Unlisted instance %d", run.InstanceID), Date: time.UnixMilli(run.StartMs).UTC().Format("2 Jan 2006"), Logger: run.Logger.Name, Duration: (time.Duration(run.EndMs-run.StartMs) * time.Millisecond).String(), ConfigVersion: c.Scoring.Version, LogStatus: "complete", Dungeon: run.Kind == model.RunDungeon, Players: len(run.Roster)}
	for _, i := range c.Instances.Instances {
		if i.ID == run.InstanceID {
			p.Title = i.Name
		}
	}
	if run.Partial {
		p.LogStatus = "partial"
	}
	for _, n := range parsed.Skipped {
		if n > 0 {
			p.LogStatus = "incomplete: skipped log lines"
		}
	}
	var deaths []model.Death
	for _, f := range run.Fights {
		if f.Boss {
			p.Bosses++
			if f.Success.Present && f.Success.V == 1 {
				p.Kills++
			}
			if f.Success.Present && f.Success.V == 0 {
				p.WipePulls++
			}
		} else {
			p.Trash++
		}
		ds, wipes, excluded, err := death.Review(f, run.EndMs, c)
		if err != nil {
			return raidPageModel{}, err
		}
		deaths = append(deaths, ds...)
		rows := raidDeathRows(f, ds, c)
		p.Deaths = append(p.Deaths, rows...)
		p.Excluded += excluded
		for _, wipe := range wipes {
			v := raidWipeView{Number: len(p.Wipes) + 1, Fight: f.Name, Duration: "unknown duration"}
			if f.DurationMs.Present {
				v.Duration = (time.Duration(f.DurationMs.V) * time.Millisecond).String()
			}
			if wipe.FirstDeath >= 0 && wipe.FirstDeath < len(rows) {
				v.First = rows[wipe.FirstDeath]
				v.HasFirst = true
			}
			if c.Scoring.Death.SummaryWindowMs != nil {
				v.Window = (time.Duration(*c.Scoring.Death.SummaryWindowMs) * time.Millisecond).String()
			}
			for _, i := range wipe.Following {
				v.Following = append(v.Following, rows[i])
			}
			p.Wipes = append(p.Wipes, v)
		}
	}
	prep, err := raidPrepScores(save, run.Fights, deaths, c)
	if err != nil {
		return raidPageModel{}, err
	}
	if officerPath != "" {
		report, err := raidComparisonFile(officerPath, prep.Scores, run, c)
		if err != nil {
			p.ComparisonError = err.Error()
		} else {
			p.Comparison = &report
		}
	}
	p.PrepPlayers = len(prep.Matched)
	p.PrepSkipped = prep.Diagnostics.Skipped
	p.Unmatched = prep.Unmatched
	p.Warnings = prep.Diagnostics.Warnings
	byID := map[string]model.NightScore{}
	if run.Kind != model.RunUnknown {
		for _, s := range prep.Scores {
			byID[s.PlayerID] = s
		}
	}
	for i, a := range run.Roster {
		s := byID[a.ID]
		summary, err := score.NightRoleParts(run.Fights, s, c)
		if err != nil {
			return raidPageModel{}, err
		}
		role := summary.Role
		if role == "" {
			role = "see fights"
		}
		if summary.Split {
			role = "split evenly"
		}
		row := raidPlayerView{Key: i, Name: a.Name, Role: role, Score: raidTenths(s.Tenths, "not enough data"), Coverage: s.Coverage}
		row.NightParts, row.NightReason = raidNightParts(summary, c)
		row.Fights, row.Flags = raidPlayerEvidence(s, run.Fights, c)
		if !s.Tenths.Present {
			p.Unscored = append(p.Unscored, row)
		} else if s.Rank.Present && run.Kind == model.RunRaid {
			row.Rank = fmt.Sprint(s.Rank.V)
			p.Ranked = append(p.Ranked, row)
		} else if run.Kind == model.RunDungeon && float64(s.Coverage) >= c.Scoring.Confidence.MinCoverage*100 {
			p.Ranked = append(p.Ranked, row)
		} else {
			p.Provisional = append(p.Provisional, row)
		}
	}
	// The engine owns ranks. Sort by its numeric ranks, not formatted score text.
	sort.SliceStable(p.Ranked, func(i, j int) bool {
		a, b := p.Ranked[i], p.Ranked[j]
		sa, sb := byID[run.Roster[a.Key].ID], byID[run.Roster[b.Key].ID]
		if sa.Rank.V != sb.Rank.V {
			return sa.Rank.V < sb.Rank.V
		}
		return a.Name < b.Name
	})
	return p, nil
}
