package compare

import (
	"fmt"
	"sort"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Report is transient and belongs only to this comparison, never the night file.
type Report struct {
	Officer               string
	Ranked                int
	NoReportReason        string
	Top, Bottom           []Entry
	ScoreTop, ScoreBottom []Entry
}

// Entry retains the officer's spelling alongside the resolved logged identity.
// Verdict is empty for an unranked/unmatched player or a score-only entry.
type Entry struct {
	Name, LoggedName, PlayerID string
	Rank                       model.Opt
	Coverage                   int
	Status, Verdict            string
	Part                       *PartComparison
	PartReason                 string
}

// Compare takes the scores, roster and config of one run plus the officer's
// prediction and the original fights. It never re-ranks or edits those inputs.
func Compare(scores []model.NightScore, roster []model.Actor, c cfg.Config, officer OfficerFile, fights []model.Fight) (Report, error) {
	if err := officer.validate(); err != nil {
		return Report{}, err
	}
	out := Report{Officer: officer.Officer}
	for _, n := range scores {
		if n.Rank.Present {
			out.Ranked++
		}
	}
	if out.Ranked < 4 {
		out.NoReportReason = "fewer than 4 ranked players"
		return out, nil
	}
	byID := map[string]model.NightScore{}
	names := []string{}
	actors := map[string]model.Actor{}
	ambiguous := map[string]bool{}
	logged := map[string]string{}
	for _, a := range roster {
		if old, ok := actors[a.Name]; ok && old.ID != a.ID {
			ambiguous[a.Name] = true
		}
		names = append(names, a.Name)
		actors[a.Name], logged[a.ID] = a, a.Name
	}
	var ranked []Entry
	for _, n := range scores {
		byID[n.PlayerID] = n
		if n.Rank.Present {
			ranked = append(ranked, entry(n, logged[n.PlayerID]))
		}
	}
	summaries := map[string]score.NightSummary{}
	for _, n := range scores {
		if !n.Rank.Present {
			continue
		}
		summary, err := score.NightRoleParts(fights, n, c)
		if err != nil {
			return Report{}, err
		}
		summaries[n.PlayerID] = summary
	}
	named := map[string]bool{}
	rows := func(list []string, top bool) []Entry {
		result := []Entry{}
		for _, name := range list {
			e := Entry{Name: name, Status: "not in this night"}
			matches, _ := score.MatchPrep([]model.PrepReport{{Name: name}}, names)
			for match := range matches {
				a := actors[match]
				if ambiguous[match] {
					continue
				}
				n := byID[a.ID]
				e = entry(n, a.Name)
				e.Name, e.PlayerID = name, a.ID
				named[a.ID] = true
				if !e.Rank.Present {
					e.Status = fmt.Sprintf("no rank: %d%% measured", e.Coverage)
					continue
				}
				e.Verdict = "agreement"
				upper := e.Rank.V <= int64((out.Ranked+1)/2)
				if top != upper {
					e.Verdict = "disagreement"
					e.Part, e.PartReason = explain(summaries[a.ID], summaries, c, top)
				}
			}
			result = append(result, e)
		}
		return result
	}
	out.Top, out.Bottom = rows(officer.Top, true), rows(officer.Bottom, false)
	out.ScoreTop = ownFive(ranked, named, true)
	out.ScoreBottom = ownFive(ranked, named, false)
	return out, nil
}

func entry(n model.NightScore, name string) Entry {
	return Entry{Name: name, LoggedName: name, PlayerID: n.PlayerID, Rank: n.Rank, Coverage: n.Coverage}
}

func ownFive(ranked []Entry, named map[string]bool, top bool) []Entry {
	// Sort only our entries, never the caller's scores. Equal ranks are stable by
	// logged name then ID, independent of the input slice's order.
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.Rank.V != b.Rank.V {
			if top {
				return a.Rank.V < b.Rank.V
			}
			return a.Rank.V > b.Rank.V
		}
		if a.LoggedName != b.LoggedName {
			return a.LoggedName < b.LoggedName
		}
		return a.PlayerID < b.PlayerID
	})
	out := []Entry{}
	for _, e := range ranked[:min(5, len(ranked))] {
		if !named[e.PlayerID] {
			out = append(out, e)
		}
	}
	return out
}
