package fights

import "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"

func indexDetail(f model.Fight, p Params) Detail {
	d := Detail{Timelines: map[string]*Timeline{}, Specs: map[string]string{}}
	if len(f.Events) > 0 {
		first, last := f.Events[0], f.Events[len(f.Events)-1]
		d.Offset = first.Offset
		d.Len = last.Offset + last.Len - first.Offset
	}
	duration := f.DurationMs
	if !f.Boss {
		duration = model.Some(f.EndMs - f.StartMs)
	}
	d.LowConfidence = duration.Present && duration.V < p.LowConfidenceFightMs
	return d
}

func makeDetail(f model.Fight, p Params) Detail {
	d := indexDetail(f, p)
	d.RolesComputed = true
	for _, a := range f.Players {
		d.Timelines[a.ID] = &Timeline{}
	}
	owner := func(id string) string {
		if o := f.Owners[id]; o != "" {
			return o
		}
		return id
	}
	boss := map[string]bool{}
	dead := map[string]bool{}
	hp, maxHP := map[string]model.Opt{}, map[string]model.Opt{}
	for _, id := range f.BossUnits {
		boss[id] = true
	}
	swings := map[string]int64{}
	var total int64
	var casts []model.Event
	for _, e := range f.Events {
		src, dst := owner(e.Src.ID), owner(e.Dst.ID)
		if tl := d.Timelines[src]; tl != nil {
			tl.Events = append(tl.Events, e)
		}
		if tl := d.Timelines[dst]; tl != nil && src != dst {
			tl.Events = append(tl.Events, e)
		}
		if e.Kind == model.Absorb {
			caster := owner(e.Caster.ID)
			if tl := d.Timelines[caster]; tl != nil {
				if caster != src && caster != dst {
					tl.Events = append(tl.Events, e)
				}
				if e.Amount.Present {
					tl.HealingDone += max(int64(0), e.Amount.V)
				}
			}
		}
		power, powerMax, powerType := e.Power, e.PowerMax, e.PowerType
		if (powerMax.Present && powerMax.V == 0) || (powerType.Present && powerType.V == healthCostPowerType) {
			power, powerMax, powerType = model.Opt{}, model.Opt{}, model.Opt{} // no bar, or a health-cost cast
		}
		if tl := d.Timelines[e.InfoUnit]; tl != nil && (e.HP.Present || e.HPMax.Present || e.X.Present || e.Y.Present || power.Present || powerMax.Present || powerType.Present) {
			tl.Samples = append(tl.Samples, Sample{TimeMs: e.TimeMs, HP: e.HP, HPMax: e.HPMax, X: e.X, Y: e.Y, Power: power, PowerMax: powerMax, PowerType: powerType})
		}
		if e.IsDeath() {
			if tl := d.Timelines[e.Dst.ID]; tl != nil {
				tl.Deaths = append(tl.Deaths, e)
			}
		}
		if e.Kind == model.CastSuccess {
			casts = append(casts, e)
		}
		if e.Kind == model.Damage && e.Stream != model.SwingSource && e.Amount.Present {
			amount := max(int64(0), e.Amount.V)
			if e.Overkill.Present && e.Overkill.V > 0 {
				amount = max(int64(0), amount-e.Overkill.V)
			}
			if tl := d.Timelines[src]; tl != nil && !charmedFriendly(e) {
				tl.DamageDone += amount
			}
			if tl := d.Timelines[e.Dst.ID]; tl != nil {
				tl.DamageTaken += amount
			}
		}
		if e.Kind == model.Heal && e.Amount.Present {
			over := int64(0)
			if e.Overheal.Present {
				over = max(int64(0), e.Overheal.V)
			}
			effective := max(int64(0), e.Amount.V-over)
			if tl := d.Timelines[src]; tl != nil {
				tl.HealingDone += effective
				tl.OverhealDone += over
			}
			if tl := d.Timelines[e.Dst.ID]; tl != nil {
				tl.HealingReceived += effective
				tl.OverhealReceived += over
			}
		}
		if e.Kind == model.Damage && e.Stream == model.SwingLanded && (boss[e.Src.ID] || !f.Boss && creature(e.Src.ID) && !pet(e.Src)) {
			total++
			swings[e.Dst.ID]++
		}
		if boss[e.InfoUnit] {
			if e.HP.Present && !dead[e.InfoUnit] {
				hp[e.InfoUnit] = e.HP
			}
			if e.HPMax.Present {
				maxHP[e.InfoUnit] = e.HPMax
			}
		}
		if e.IsDeath() && boss[e.Dst.ID] {
			hp[e.Dst.ID] = model.Some(0)
			dead[e.Dst.ID] = true
		}
	}
	var top int64
	for _, a := range f.Players {
		top = max(top, swings[a.ID])
	}
	// f shares the map with its caller but never modifies any caller-owned event.
	for _, a := range f.Players {
		// A boss without landed boss swings cannot distinguish tanks from DPS
		// or healers. Positive output alone must not turn an unknown tank into DPS.
		if f.Boss && total == 0 {
			continue
		}
		tl := d.Timelines[a.ID]
		tank := total > 0 && swings[a.ID] > 0 && float64(swings[a.ID])/float64(total) >= p.TankMinBossSwingShare &&
			float64(swings[a.ID]) >= float64(top)*p.TankMinTopRatio
		ratio := float64(tl.DamageDone) * p.HealerMinHealingDamageRatio
		healer := float64(tl.HealingDone) > ratio
		if !p.HealerStrictlyGreater {
			healer = float64(tl.HealingDone) >= ratio
		}
		role := ""
		if tl.DamageDone > 0 {
			role = "dps"
		}
		if tank {
			role = "tank"
		}
		if tl.HealingDone > 0 && healer && (!p.HealerRequiresNotTank || !tank) {
			role = "healer"
		}
		if role != "" {
			f.Roles[a.ID] = role
		}
	}
	if p.InferSpecs != nil && len(p.Profiles) > 0 {
		allowed := map[string]bool{}
		for _, pr := range p.Profiles {
			allowed[pr.ID] = len(pr.CastSignatures) > 0
		}
		for id, spec := range p.InferSpecs(casts, p.Profiles) {
			if d.Timelines[id] != nil && allowed[spec] {
				d.Specs[id] = spec
			}
		}
	}
	if len(f.BossUnits) > 0 {
		current, maximum := int64(0), int64(0)
		known := true
		for _, id := range f.BossUnits {
			if !hp[id].Present || !maxHP[id].Present {
				known = false
			}
			current += hp[id].V
			maximum += maxHP[id].V
		}
		if known {
			d.RemainingHP, d.MaximumHP = model.Some(current), model.Some(maximum)
		}
	}
	return d
}

// healthCostPowerType is the type the log writes on a health-cost cast; its current and maximum repeat the
// health fields, so it is not a power sample.
const healthCostPowerType = -2

// Review round 1 / PROBES: 0x1248, 0x11248 and 0x1218 are Player GUIDs
// with CONTROL_NPC (0x200). REACTION_FRIENDLY is 0x10; TYPE_PET is bit 12,
// TYPE_GUARDIAN bit 13. Do not use reaction or pet bits to change identity.
func charmedFriendly(e model.Event) bool {
	return player(e.Src.ID) && e.Src.Flags&0x200 != 0 && e.Dst.Flags&0x10 != 0
}
