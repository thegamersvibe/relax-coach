package death

import "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"

func init() { Register(6, unhealed) }

func healthSample(e model.Event, id string) bool {
	return e.InfoUnit == id && e.HPMax.Present && e.HPMax.V > 0
}
func unhealed(c *context) verdict {
	latestFull, samples := -1, 0
	for i, e := range c.window {
		if !healthSample(e, c.victim.ID) {
			continue
		}
		samples++
		if e.HP.Present && float64(e.HP.V) >= float64(e.HPMax.V)*c.params.FullHealthFraction {
			latestFull = i
		}
	}
	start := -1
	for i, e := range c.window {
		if i > latestFull && damage(e) && e.Dst.ID == c.victim.ID {
			start = i
			break
		}
	}
	if start < 0 || c.at-c.window[start].TimeMs < c.params.UnhealedMinMs {
		return verdict{}
	}
	from := c.window[start].TimeMs
	for _, e := range c.window {
		if e.TimeMs < from || e.Dst.ID != c.victim.ID {
			continue
		}
		if e.Kind == model.Heal && effective(e) > 0 || e.Kind == model.Absorb && nonnegative(e.Amount) > 0 || damage(e) && nonnegative(e.Absorbed) > 0 {
			return verdict{}
		}
	}
	whole := false
	var owners []string
	for _, id := range c.view.ids {
		if c.view.f.Roles[id] != "healer" {
			continue
		}
		if c.aliveThroughout(id, from, c.at) {
			whole = true
		}
		if c.alive(id, c.at) {
			owners = append(owners, c.view.players[id].Name)
		}
	}
	if !whole {
		return verdict{}
	}
	return verdict{cause: Unhealed, owner: "healers", owners: owners, low: samples < c.params.MinHealthSamples}
}
