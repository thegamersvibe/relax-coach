package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var classColors = map[string]string{
	"WARRIOR": "#C69B6D", "PALADIN": "#F48CBA", "HUNTER": "#AAD372", "ROGUE": "#FFF468", "PRIEST": "#E6E6E6",
	"SHAMAN": "#0070DD", "MAGE": "#3FC7EB", "WARLOCK": "#8788EE", "DRUID": "#FF7C0A",
}

// Paper-doll order: armor, jewelry, weapons.
var slotOrder = []int{1, 2, 3, 15, 5, 9, 10, 6, 7, 8, 11, 12, 13, 14, 16, 17, 18}
var slotNames = map[int]string{1: "Head", 2: "Neck", 3: "Shoulder", 15: "Back", 5: "Chest", 9: "Wrist", 10: "Hands",
	6: "Waist", 7: "Legs", 8: "Feet", 11: "Ring 1", 12: "Ring 2", 13: "Trinket 1", 14: "Trinket 2", 16: "Main Hand", 17: "Off Hand", 18: "Ranged"}

const maxLevel = 60

type TalentStep struct {
	Level       int
	Name, State string // done | todo | unknown | later
}

type DungeonView struct {
	Name, Level, Location string
	OnRoute               bool
	Quests                []DungeonQuest
}

type DungeonQuest struct {
	Name, Pickup string
	InLog, Later bool
	ClassOnly    string
}

type OwnedView struct {
	Name, Slot string
	Gain       int
	// GainText is the shown percentage ("+12%", ">100%"), empty when too few slots are scored to state one.
	GainText string
}

// pctText is the addon's one display rule for a gain percentage (Gear.lua G.PctText): above 100 it reads ">100%".
func pctText(n int) string {
	if n > 100 {
		return ">100%"
	}
	return fmt.Sprintf("%d%%", n)
}

func money(c float64) string {
	g, s, cp := int(c)/10000, int(c)/100%100, int(c)%100
	if g > 0 {
		return fmt.Sprintf("%dg %ds %dc", g, s, cp)
	}
	if s > 0 {
		return fmt.Sprintf("%ds %dc", s, cp)
	}
	return fmt.Sprintf("%dc", cp)
}

func enrich(cv *CharView, c, s, g map[string]any, sess []any) {
	cv.ClassColor = classColors[cv.Class]
	if cv.ClassColor == "" {
		cv.ClassColor = "#c9a227"
	}
	cv.MaxLevel = cv.Level >= maxLevel
	cv.Money = money(N(s["money"]))
	cv.Counts = map[string]int{}
	for _, f := range cv.Findings {
		cv.Counts[f.Area]++
	}
	if z := M(s["zone"]); z != nil {
		cv.Zone = S(z["zone"])
	}
	if j := M(s["journey"]); j != nil {
		cv.Guide = S(j["zone"])
		if cv.Guide == "" {
			cv.Guide = S(j["guide"])
		}
		if from, to := int(N(j["from"])), int(N(j["to"])); from > 0 {
			cv.Guide = fmt.Sprintf("%s (%d-%d)", cv.Guide, from, to)
		}
		cv.GuideStep = S(j["step"])
	}
	if le := M(c["lastError"]); le != nil && S(le["version"]) == S(c["addonVersion"]) {
		cv.AddonError = S(le["where"]) + ": " + S(le["err"])
	}
	for k := range M(c["stale"]) {
		cv.Stale = append(cv.Stale, k)
	}
	sort.Strings(cv.Stale)

	// Gear paper-doll: only assess what's known; unknown items are shown as such, never as 0%.
	cv.GearStatus = S(g["status"])
	slots := M(g["slots"])
	eq := M(s["equipped"])
	states := M(s["equipmentState"])
	unreadable := 0
	unassessed := map[string]bool{}
	for _, inv := range slotOrder {
		k := strconv.Itoa(inv)
		e := M(slots[k])
		sv := SlotView{Name: slotNames[inv], GapPct: -1, Weapon: inv >= 16, Status: S(e["scoreStatus"]), Note: S(e["note"])}
		if eq[k] != nil {
			sv.Equipped = true
			item := M(eq[k])
			link := S(item["link"])
			sv.Item = itemName(link)
			if link == "" && N(item["id"]) > 0 {
				sv.Item = fmt.Sprintf("Item #%d (name pending)", int(N(item["id"])))
			}
		}
		if !sv.Equipped {
			if S(states[k]) == "empty" {
				sv.Status = "empty"
			} else {
				sv.Status = "unreadable"
				sv.Note = "Equipment was not captured for this slot; it is not confirmed empty. Log in with the updated addon, then /reload and Refresh."
				unreadable++
			}
		} else if sv.Status == "" || sv.Status == "empty" || sv.Status == "unreadable" || sv.Status == "estimated" {
			// Legacy estimates used stat weights, not comparable pipeline units.
			sv.Status = "unknown"
		}
		if sv.Status == "unknown" || sv.Status == "unreadable" {
			unassessed[sv.Name] = true
		}
		if b := M(e["best"]); b != nil {
			sv.BestName, sv.BestHow, sv.BestSource, sv.BestWhere = S(b["name"]), S(b["how"]), S(b["source"]), S(b["where"])
			_, hasGap := e["gapPct"]
			gap := max(0, int(N(e["gapPct"])))
			// pctHidden: too few slots are scored to state a number; the pick still counts as an upgrade.
			if hasGap && sv.Status != "unknown" && sv.Status != "unreadable" && e["pctHidden"] != true {
				sv.GapPct = min(100, gap) // colour scale
				sv.GapText = pctText(gap)
			}
			if sv.Equipped && hasGap && sv.Status != "unknown" && sv.Status != "unreadable" && gap >= 12 {
				cv.GearUpgrades++
			}
		}
		switch sv.Status {
		case "known", "estimated":
			cv.GearAssessed++
		case "unknown", "unreadable":
			cv.GearUnknown++
		case "empty":
			if sv.BestName != "" {
				cv.GearEmpty++
			}
		}
		cv.Slots = append(cv.Slots, sv)
	}
	switch cv.GearStatus {
	case "missing":
		cv.GearNote = "Gear checks are off: install GearQuest Forever."
	case "layout":
		cv.GearNote = "Gear checks are off: GearQuest changed its data format. Update Relax Coach."
	case "classdata", "nodata":
		cv.GearNote = "Gear checks are off: enable your class's GearQuest Forever module, then /reload."
	}
	if len(unassessed) > 0 {
		// Do not carry forward legacy empty-slot or percentage advice that
		// cannot be supported by the corresponding equipment assessment.
		var findings []Finding
		for _, f := range cv.Findings {
			unconfirmed := false
			if f.Area == "Gear" {
				for name := range unassessed {
					if f.Text == "Empty "+name+" slot" || strings.HasPrefix(f.Text, name+": ") ||
						(strings.HasPrefix(f.Text, "Upgrade in your bags: ") && strings.HasSuffix(f.Text, " for "+name)) {
						unconfirmed = true
						break
					}
				}
			}
			if !unconfirmed {
				findings = append(findings, f)
			}
		}
		cv.Findings = findings
		cv.Counts = map[string]int{}
		for _, f := range cv.Findings {
			cv.Counts[f.Area]++
		}
	}
	if unreadable > 0 {
		if cv.GearNote != "" {
			cv.GearNote += " "
		}
		cv.GearNote += fmt.Sprintf("Equipment was not captured for %d slots. These are not confirmed empty. Log in with the updated addon, then /reload and Refresh.", unreadable)
		if unreadable == len(slotOrder) && cv.GearStatus == "ok" {
			cv.GearStatus = "unavailable"
		}
	}
	for _, o := range List(g["owned"]) {
		om := M(o)
		if unassessed[S(om["label"])] {
			continue
		}
		ov := OwnedView{Name: S(om["name"]), Slot: S(om["label"]), Gain: int(N(om["gain"]))}
		if om["pctHidden"] != true {
			ov.GainText = "+" + pctText(ov.Gain)
		}
		cv.Owned = append(cv.Owned, ov)
	}

	// Talents: tree split vs points actually available (spent + unspent)
	t := M(s["talents"])
	tabs := List(t["tabs"])
	total := int(N(t["unspent"]))
	for _, tb := range tabs {
		total += int(N(M(tb)["spent"]))
	}
	for _, tb := range tabs {
		sp := int(N(M(tb)["spent"]))
		tv := TalentView{Name: S(M(tb)["name"]), Spent: sp}
		if total > 0 {
			tv.Pct = sp * 100 / total
		}
		cv.Talents = append(cv.Talents, tv)
	}
	if tp := M(c["talentPlan"]); tp != nil {
		cv.TalentSource, cv.TalentURL = S(tp["source"]), S(tp["url"])
		var steps []TalentStep
		for _, st := range List(tp["timeline"]) {
			sm := M(st)
			steps = append(steps, TalentStep{int(N(sm["level"])), S(sm["name"]), S(sm["state"])})
		}
		// show a window: 4 recent, the rest upcoming (max 10)
		start := 0
		for i, st := range steps {
			if st.Level <= cv.Level {
				start = max(0, i-3)
			}
		}
		end := min(len(steps), start+10)
		cv.TalentSteps = steps[start:end]
	}

	// Dungeons
	for _, d := range List(M(c["dungeons"])["list"]) {
		dm := M(d)
		dv := DungeonView{Name: S(dm["name"]), Level: S(dm["level"]), Location: S(dm["location"]), OnRoute: dm["onRoute"] == true}
		for _, q := range List(dm["quests"]) {
			qm := M(q)
			dv.Quests = append(dv.Quests, DungeonQuest{S(qm["name"]), S(qm["pickup"]), qm["inLog"] == true, qm["tooLow"] == true, S(qm["classOnly"])})
		}
		cv.Dungeons = append(cv.Dungeons, dv)
	}

	// Sessions: last 5. Active time excludes AFK, taxi and dead time.
	var secs, combat, stopped, dead, afk, taxi float64
	procs := map[string]*ProcView{}
	for i := len(sess) - 1; i >= 0 && i >= len(sess)-5; i-- {
		ss := M(sess[i])
		secs += N(ss["seconds"])
		combat += N(ss["combat"])
		stopped += N(ss["recovery"])
		dead += N(ss["dead"])
		afk += N(ss["afk"])
		taxi += N(ss["taxi"])
		cv.Stats.Deaths += len(List(ss["deaths"]))
		for name, p := range M(ss["procs"]) {
			pv := procs[name]
			if pv == nil {
				pv = &ProcView{Name: name}
				procs[name] = pv
			}
			pv.Used += int(N(M(p)["used"]))
			pv.Windows += int(N(M(p)["windows"]))
		}
	}
	if secs > 60 {
		st := &cv.Stats
		st.HasData = true
		pct := func(x float64) int { return int(x / secs * 100) }
		st.PlayMin = int(secs / 60)
		st.CombatPct, st.DowntimePct, st.DeadPct, st.AFKPct, st.TaxiPct = pct(combat), pct(stopped), pct(dead), pct(afk), pct(taxi)
		st.OtherPct = max(0, 100-st.CombatPct-st.DowntimePct-st.DeadPct-st.AFKPct-st.TaxiPct)
		if active := secs - afk - taxi - dead; active > 60 {
			st.ActiveMin = int(active / 60)
			st.DeathsPerHour = fmt.Sprintf("%.1f", float64(st.Deaths)/(active/3600))
		}
	}
	if mx := N(s["xpMax"]); mx > 0 && !cv.MaxLevel {
		cv.Stats.XPToLevel = int(mx - N(s["xp"]))
		if cv.XPPerHour > 0 {
			cv.Stats.MinToLevel = cv.Stats.XPToLevel * 60 / cv.XPPerHour
		}
	}
	if cv.MaxLevel {
		cv.XPPct, cv.RestedPct = 100, 0
	}
	for _, p := range procs {
		if p.Windows > 0 {
			p.UsedPct = p.Used * 100 / p.Windows
			cv.Procs = append(cv.Procs, *p)
		}
	}
	sort.Slice(cv.Procs, func(i, j int) bool { return cv.Procs[i].Name < cv.Procs[j].Name })
}

// "|cff...|Hitem:123::|h[Name]|h|r" -> "Name"
func itemName(link string) string {
	for i := 0; i < len(link); i++ {
		if link[i] == '[' {
			for j := i + 1; j < len(link); j++ {
				if link[j] == ']' {
					return link[i+1 : j]
				}
			}
		}
	}
	return "Equipped"
}
