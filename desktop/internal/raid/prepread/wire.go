package prepread

import (
	"strconv"
	"strings"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func text(s string, cap int) bool {
	if len(s) == 0 || len(s) > cap {
		return false
	}
	for i := range len(s) {
		if s[i] < 32 || s[i] == 127 || s[i] == '|' {
			return false
		}
	}
	return true
}

// Lua string.lower in the client's ASCII locale, not Unicode case folding.
func nameKey(s string) string {
	if !text(s, 96) || s == "unreadable" || s[0] == ' ' || s[len(s)-1] == ' ' || strings.Contains(s, "  ") {
		return ""
	}
	b := []byte(s)
	for i, c := range b {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || digit(c) || c == ' ' || c == '\'' || c == '-' || c >= 128) {
			return ""
		}
		if c >= 'A' && c <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func decimal(s string, low, high int64) (int64, bool) {
	if len(s) == 0 || len(s) > 10 || len(s) > 1 && s[0] == '0' {
		return 0, false
	}
	for i := range len(s) {
		if !digit(s[i]) {
			return 0, false
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil && v >= low && v <= high
}

func category(s string) bool {
	switch s {
	case "generic", "potion", "elixir", "flask", "scroll", "food", "enhancement", "bandage", "other", "vantus", "utility", "combat", "relic", "draught", "device", "temporary":
		return true
	}
	return false
}

func class(s string) bool {
	switch s {
	case "WARRIOR", "PALADIN", "HUNTER", "ROGUE", "PRIEST", "SHAMAN", "MAGE", "WARLOCK", "DRUID", "unreadable":
		return true
	}
	return false
}

// decodeBody accepts only canonical stored bodies. Unlike a live client, this
// reader has no local realm to append: bare names are not repaired. Original
// spelling and list order survive. Bounds match Wire.decode + Prep.validate.
func decodeBody(body string) (model.PrepReport, string) {
	var r model.PrepReport
	if len(body) > limits.MaxBodyBytes {
		return r, "body size"
	}
	f := strings.Split(body, "\t")
	if len(f) != 11 || f[0] != "1" {
		return r, "wire fields"
	}
	if !text(f[1], 24) || !class(f[3]) || nameKey(f[2]) == "" || !strings.Contains(f[2], "-") {
		return r, "wire identity"
	}
	r.Protocol, r.AddonVersion, r.Name, r.Class = 1, f[1], f[2], f[3]
	if r.AddonVersion == "unreadable" {
		r.AddonVersion = ""
	}
	if r.Class == "unreadable" {
		r.Class = ""
	}
	var ok bool
	if f[4] != "u" {
		r.Level, ok = decimal(f[4], 1, 60)
		if !ok {
			return r, "wire level"
		}
	}
	if f[5] != "u" {
		v, ok := decimal(f[5], 0, limits.MaxPlayed)
		if !ok {
			return r, "wire played"
		}
		r.PlayedAtLevelS = model.Some(v)
	}
	r.Reason = f[6]
	if r.Reason != "join" && r.Reason != "ready" && r.Reason != "encounter" {
		return r, "wire reason"
	}
	r.SenderClock, ok = decimal(f[7], 0, limits.MaxStamp)
	if !ok {
		return r, "wire clock"
	}
	switch f[8] {
	case "u":
		r.ConsumablesUnreadable = true
	case "-":
	default:
		rows := strings.Split(f[8], ";")
		if len(rows) > 32 {
			return r, "wire consumables"
		}
		seen := map[int64]bool{}
		for _, row := range rows {
			fields := strings.Split(row, ",")
			if len(fields) != 3 || !category(fields[0]) {
				return r, "wire consumables"
			}
			id, idOK := decimal(fields[1], 1, limits.MaxItemID)
			count, countOK := decimal(fields[2], 1, limits.MaxCount)
			if !idOK || !countOK || seen[id] {
				return r, "wire consumables"
			}
			seen[id] = true
			r.Consumables = append(r.Consumables, model.PrepConsumable{Category: fields[0], ItemID: id, Count: count})
		}
	}
	switch f[9] {
	case "n":
		if f[10] != "n" {
			return r, "wire gear markers"
		}
	case "u":
		if f[10] != "u" {
			return r, "wire gear markers"
		}
		r.GearChecked, r.GearUnreadable = true, true
	default:
		r.GearChecked = true
		if f[9] != "-" {
			rows := strings.Split(f[9], ";")
			if len(rows) > 19 {
				return r, "wire gear"
			}
			for _, row := range rows {
				fields := strings.Split(row, ",")
				if len(fields) != 3 {
					return r, "wire gear"
				}
				slot, slotOK := decimal(fields[0], 1, 19)
				worn, wornOK := decimal(fields[1], 0, limits.MaxItemID)
				bag, bagOK := decimal(fields[2], 1, limits.MaxItemID)
				if !slotOK || !wornOK || !bagOK {
					return r, "wire gear"
				}
				r.BetterGear = append(r.BetterGear, model.PrepGear{Slot: slot, WornID: worn, BagID: bag})
			}
		}
		if f[10] != "-" {
			slots := strings.Split(f[10], ",")
			if len(slots) > 19 {
				return r, "wire notScored"
			}
			seen := map[int64]bool{}
			for _, s := range slots {
				slot, ok := decimal(s, 1, 19)
				if !ok || seen[slot] {
					return r, "wire notScored"
				}
				seen[slot] = true
				r.NotScored = append(r.NotScored, slot)
			}
		}
	}
	return r, ""
}

func fmtNumber(n int64) string { return strconv.FormatInt(n, 10) }

// encodeBody is the inverse for validated model reports only, not a public
// writer. It never writes a save. Tests compare it against every Lua body.
func encodeBody(r model.PrepReport) string {
	version, cls, level, played := r.AddonVersion, r.Class, "u", "u"
	if version == "" {
		version = "unreadable"
	}
	if cls == "" {
		cls = "unreadable"
	}
	if r.Level != 0 {
		level = fmtNumber(r.Level)
	}
	if r.PlayedAtLevelS.Present {
		played = fmtNumber(r.PlayedAtLevelS.V)
	}
	c, g, ns := "-", "n", "n"
	if r.ConsumablesUnreadable {
		c = "u"
	} else if len(r.Consumables) > 0 {
		rows := make([]string, 0, len(r.Consumables))
		for _, row := range r.Consumables {
			rows = append(rows, row.Category+","+fmtNumber(row.ItemID)+","+fmtNumber(row.Count))
		}
		c = strings.Join(rows, ";")
	}
	if r.GearChecked {
		g, ns = "-", "-"
		if r.GearUnreadable {
			g, ns = "u", "u"
		} else {
			if len(r.BetterGear) > 0 {
				rows := make([]string, 0, len(r.BetterGear))
				for _, row := range r.BetterGear {
					rows = append(rows, fmtNumber(row.Slot)+","+fmtNumber(row.WornID)+","+fmtNumber(row.BagID))
				}
				g = strings.Join(rows, ";")
			}
			if len(r.NotScored) > 0 {
				slots := make([]string, 0, len(r.NotScored))
				for _, slot := range r.NotScored {
					slots = append(slots, fmtNumber(slot))
				}
				ns = strings.Join(slots, ",")
			}
		}
	}
	return strings.Join([]string{fmtNumber(r.Protocol), version, r.Name, cls, level, played, r.Reason, fmtNumber(r.SenderClock), c, g, ns}, "\t")
}
