package fights

// Keep one timestamp and membership bit per real player death, never its event
// or actors. Membership is recorded before index mode discards fight events;
// a boss's inclusive tail belongs to its events, not its display EndMs.
type playerDeathPosition struct {
	at      int64
	inFight bool
}

// Final spans matter: a pending exit may resume, an empty visit emits no run,
// and file-order timestamps need not be monotonic. Classify only after EOF.
func (r *Result) countPlayerDeaths(deaths []playerDeathPosition) {
	for _, death := range deaths {
		inRun := false
		for _, run := range r.Runs {
			if death.at >= run.StartMs && death.at <= run.EndMs {
				inRun = true
				break
			}
		}
		if !inRun {
			r.PlayerDeathsOutsideRuns++
		} else if !death.inFight {
			r.PlayerDeathsInRunsWithoutFight++
		}
	}
}
