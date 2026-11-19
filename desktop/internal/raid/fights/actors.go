package fights

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"sort"
	"strconv"
	"strings"
)

func player(id string) bool  { return strings.HasPrefix(id, "Player-") }
func pet(a model.Actor) bool { return strings.HasPrefix(a.ID, "Pet-") || byte(a.Flags>>12)&3 != 0 }
func creature(id string) bool {
	return strings.HasPrefix(id, "Creature-") || strings.HasPrefix(id, "Vehicle-")
}

// Ownership evidence is collected for the whole run before projecting fights:
// late owner blocks fix earlier events, but never cross a header/run boundary.
func describe(r *Run, events []model.Event, p Params) {
	actors := map[string]model.Actor{}
	owners, priority := map[string]string{}, map[string]int{}
	pets := map[string]bool{}
	putOwner := func(id, owner string, rank int) {
		if id != "" && owner != "" && owner != "0" && id != owner && rank > priority[id] {
			owners[id] = owner
			priority[id] = rank
			pets[id] = true
		}
	}
	for _, e := range events {
		if e.InfoUnit != "" && (e.HP.Present || e.HPMax.Present || e.Power.Present || e.PowerMax.Present || e.PowerType.Present || e.X.Present || e.Y.Present) {
			r.Advanced = true
		}
		for _, a := range []model.Actor{e.Src, e.Dst} {
			if a.ID == "" {
				continue
			}
			old := actors[a.ID]
			if a.Name == "" {
				a.Name = old.Name
			}
			actors[a.ID] = a
			if pet(a) {
				pets[a.ID] = true
			}
			putOwner(a.ID, a.OwnerID, 2)
			if player(a.ID) && a.Flags&1 != 0 {
				r.Logger = a
			}
		}
		if e.Kind == model.Summon {
			putOwner(e.Dst.ID, e.Src.ID, 1)
			pets[e.Dst.ID] = true
		}
		putOwner(e.InfoUnit, e.InfoOwnerID, 3)
	}
	resolved := map[string]string{}
	for id := range owners {
		seen := map[string]bool{id: true}
		owner := owners[id]
		for owner != "" && !player(owner) && !seen[owner] {
			seen[owner] = true
			owner = owners[owner]
		}
		if player(owner) {
			resolved[id] = owner
		}
	}
	roster := map[string]bool{}
	for i := range r.Fights {
		f := &r.Fights[i]
		f.Owners = map[string]string{}
		participants := map[string]bool{}
		touched := map[string]bool{}
		for _, c := range f.Combatants {
			if player(c.PlayerID) {
				participants[c.PlayerID] = true
			}
		}
		for _, e := range f.Events {
			for _, id := range []string{e.Src.ID, e.Dst.ID, e.InfoUnit} {
				if id == "" {
					continue
				}
				touched[id] = true
				if player(id) {
					participants[id] = true
				}
				if owner := resolved[id]; owner != "" {
					f.Owners[id] = owner
					participants[owner] = true
				}
			}
		}
		for id := range touched {
			if pets[id] && resolved[id] == "" {
				f.Unowned = append(f.Unowned, id)
			}
		}
		sort.Strings(f.Unowned)
		for _, id := range keys(participants) {
			a := actors[id]
			a.ID = id
			f.Players = append(f.Players, a)
			roster[id] = true
		}
		f.Roles = map[string]string{}
		selectBosses(f, p)
		d := indexDetail(*f, p)
		if p.DetailsFor == nil || p.DetailsFor(*f) {
			d = makeDetail(*f, p)
		} else {
			f.Events = nil
			d.UnscoredReason = "detailNotSelected"
		}
		if f.Partial {
			d.UnscoredReason = "partialFight"
		}
		if r.Kind == model.RunUnknown {
			d.UnscoredReason = "unknownInstance"
		}
		f.Offset, f.Len, f.Specs = d.Offset, d.Len, d.Specs
		r.Details = append(r.Details, d)
	}
	for _, id := range keys(roster) {
		a := actors[id]
		a.ID = id
		r.Roster = append(r.Roster, a)
	}
}
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func selectBosses(f *model.Fight, p Params) {
	if !f.Boss {
		return
	}
	candidates := map[string]model.Actor{}
	hp := map[string]int64{}
	for _, e := range f.Events {
		if e.Kind == model.Damage && creature(e.Dst.ID) && (player(e.Src.ID) || f.Owners[e.Src.ID] != "") {
			candidates[e.Dst.ID] = e.Dst
		}
		if e.HPMax.Present && e.HPMax.V > hp[e.InfoUnit] {
			hp[e.InfoUnit] = e.HPMax.V
		}
	}
	var rules []CreatureSelector
	if p.BossUnits != nil {
		rules = p.BossUnits(f.EncounterID)
	}
	if len(rules) > 0 {
		// An override can name a boss with no player damage (e.g. a separate phase).
		for _, e := range f.Events {
			for _, a := range []model.Actor{e.Src, e.Dst} {
				if creature(a.ID) {
					candidates[a.ID] = a
				}
			}
		}
		for id, a := range candidates {
			for _, rule := range rules {
				if matches(a, rule) {
					f.BossUnits = append(f.BossUnits, id)
					break
				}
			}
		}
		sort.Strings(f.BossUnits)
		return
	}
	if p.BossUnitSelection != "highest_max_health_damaged_creature" {
		return
	}
	best := ""
	for id := range candidates {
		if hp[id] > 0 && (best == "" || hp[id] > hp[best] || hp[id] == hp[best] && id < best) {
			best = id
		}
	}
	if best != "" {
		f.BossUnits = []string{best}
	}
}
func matches(a model.Actor, r CreatureSelector) bool {
	if r.ID == 0 && r.Name == "" {
		return false
	}
	if r.Name != "" && r.Name != a.Name {
		return false
	}
	if r.ID != 0 {
		parts := strings.Split(a.ID, "-")
		if len(parts) < 7 {
			return false
		}
		id, err := strconv.ParseInt(parts[5], 10, 64)
		if err != nil || id != r.ID {
			return false
		}
	}
	return true
}
