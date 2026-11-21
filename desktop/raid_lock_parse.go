package main

import "strings"

// Audit raw keys and entry counts before the permissive save parser reduces a
// table to a map. Only raid admission opts in; other save consumers are unchanged.
func raidSaveVariables(data []byte) (out map[string]any, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, errRaidRankLocked
		}
	}()
	p := luaParser{s: string(data), maxValues: raidSaveMaxValues, uniqueKeys: true, entry: raidRankEntry}
	return p.parse()
}

func raidRankEntry(path []string, rawKey any, entries int) error {
	if len(path) != 2 || path[0] != "RelaxRaidDB" || path[1] != "ranks" {
		return nil
	}
	if entries > raidRankMaxCharacters {
		return errRaidRankLocked
	}
	name, ok := rawKey.(string)
	if !ok || !raidRankText(name, 64) {
		return errRaidRankLocked
	}
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return errRaidRankLocked
	}
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return errRaidRankLocked
		}
	}
	return nil
}
