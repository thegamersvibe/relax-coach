package main

import (
	"errors"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/prepread"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

type raidPrepResult struct {
	WindowStartS, WindowEndS int64
	Reports                  []model.PrepReport
	Matched                  map[string][]model.PrepReport
	Unmatched                []model.PrepReport
	Diagnostics              prepread.Report
	Scores                   []model.NightScore
}

// raidPrepScores is app glue over caller-selected SavedVariables bytes and one
// run's detailed fights. It performs no I/O, stores nothing and never shifts
// Stamped. File selection and page handlers remain outside this function.
// Fights must come from one run and retain their first decoded event.
func raidPrepScores(data []byte, fights []model.Fight, deaths []model.Death, c cfg.Config) (raidPrepResult, error) {
	const msPerSecond = int64(time.Second / time.Millisecond)
	const msPerHour = int64(time.Hour / time.Millisecond)
	if len(fights) == 0 {
		return raidPrepResult{}, errors.New("prep requires at least one fight")
	}
	var firstUTC, lastUTC int64
	for i, f := range fights {
		if len(f.Events) == 0 {
			return raidPrepResult{}, errors.New("prep requires the first event's clock offset")
		}
		offset := int64(f.Events[0].OffsetH) * msPerHour
		start, end := f.StartMs-offset, f.EndMs-offset
		if start < 0 || end < start {
			return raidPrepResult{}, errors.New("invalid prep fight bounds")
		}
		if i == 0 || start < firstUTC {
			firstUTC = start
		}
		if i == 0 || end > lastUTC {
			lastUTC = end
		}
	}
	firstS := firstUTC / msPerSecond
	lastS := (lastUTC + msPerSecond - 1) / msPerSecond
	// Parse owns the two-hour lookback. Passing WindowStartS here would apply it
	// twice. These arguments yield [floor(first UTC - 2h), ceil(last UTC)].
	reports, diagnostics := prepread.Parse(data, firstS, lastS)
	names := []string{}
	for _, f := range fights {
		for _, p := range f.Players {
			names = append(names, p.Name)
		}
	}
	matched, unmatched := score.MatchPrep(reports, names)
	return raidPrepResult{WindowStartS: firstS - 2*int64(time.Hour/time.Second), WindowEndS: lastS, Reports: reports, Matched: matched, Unmatched: unmatched, Diagnostics: diagnostics, Scores: score.Score(fights, deaths, reports, c)}, nil
}
