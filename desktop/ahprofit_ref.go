package main

// Item, node and map names for the Farming page from AH Profit's own bundled
// data files (Interface/AddOns/AHProfit/FarmData.lua and History.lua) in the
// same game install. These files are Lua code ("local _, ns = ..." followed
// by "ns.farmData = {...}"); only the table literal after that assignment is
// parsed, by the same literal parser as the saves. Never executed, read-only.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ahpRefMaxBytes caps each bundled file (FarmData.lua is under 100 KB today).
var ahpRefMaxBytes int64 = 4 << 20

// farmRefParses counts parses of the bundled files (tests check the cache).
var farmRefParses int

type farmItemRef struct {
	Name   string
	Vendor int // copper per unit; meaningful only when VendorKnown
	// VendorKnown: from the save (the game's own sell price, 0 = cannot be
	// sold to a vendor), or a bundled vendor price above 0. A bundled 0 is
	// "not known", never "worth nothing".
	VendorKnown bool
}

// farmSeed is AH Profit's bundled price feed (Data.lua: ns.seed = { t,
// label, faction, n, flat = { itemID, price, listed, ... } }).
type farmSeed struct {
	Time    time.Time
	Faction string
	Prices  map[int][2]int // itemID -> price per unit, listed
}

type farmRef struct {
	Seed  *farmSeed
	Items map[int]farmItemRef
	Nodes map[int]string // AH Profit's node-name hash -> name (unambiguous hashes only)
	Maps  map[int]string // uiMapID -> zone name
}

type farmRefCache struct {
	key string
	ref farmRef
}

// ahpNodeHash is AH Profit's X.hash: h = (h*31 + byte) % 1000003.
func ahpNodeHash(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = (h*31 + int(s[i])) % 1000003
	}
	return h
}

// parseLuaAssignedTable parses the table literal assigned by the first
// "<name> =" statement at the start of a line. Errors and panics are
// returned as errors; nothing in the file is executed.
func parseLuaAssignedTable(src, name string) (out map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("%s: unreadable (%v)", name, r)
		}
	}()
	at := -1
	for i := 0; i+len(name) <= len(src); {
		j := strings.Index(src[i:], name)
		if j < 0 {
			break
		}
		j += i
		if j == 0 || src[j-1] == '\n' {
			at = j + len(name)
			break
		}
		i = j + 1
	}
	if at < 0 {
		return nil, fmt.Errorf("%s not found", name)
	}
	p := &luaParser{s: src, pos: at, maxValues: ahpMaxValues}
	if !p.eat('=') {
		return nil, fmt.Errorf("%s: expected =", name)
	}
	p.skip()
	if p.pos >= len(p.s) || p.s[p.pos] != '{' {
		return nil, fmt.Errorf("%s: expected a table", name)
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	return M(v), nil
}

// openAddonFile opens Interface/AddOns/<addon>/<file> inside game with the
// same directory checks as the account saves. The second result identifies
// the file version (size and time) for caching.
func openAddonFile(game, addon, file string) (*os.File, string, error) {
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	dir := root
	for _, name := range []string{"Interface", "AddOns", addon} {
		next, err := openPlainDirectory(dir, name)
		if dir != root {
			dir.Close()
		}
		if err != nil {
			return nil, "", err
		}
		dir = next
	}
	defer dir.Close()
	st, err := dir.Lstat(file)
	if err != nil {
		return nil, "", err
	}
	if !st.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%s is not a regular file", file)
	}
	f, err := dir.Open(file)
	if err != nil {
		return nil, "", err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		f.Close()
		return nil, "", fmt.Errorf("%s changed while opening", file)
	}
	return f, fmt.Sprintf("%s:%d:%d", file, opened.Size(), opened.ModTime().UnixNano()), nil
}

func readAddonFile(game, addon, file string) (string, string, error) {
	f, key, err := openAddonFile(game, addon, file)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, ahpRefMaxBytes+1))
	if err == nil && int64(len(data)) > ahpRefMaxBytes {
		err = fmt.Errorf("%s is larger than %d MB; not read", file, ahpRefMaxBytes>>20)
	}
	return string(data), key, err
}

// loadFarmRef reads the bundled files of the AHProfit folder in game. A
// missing or unreadable file only means fewer names; it is never an error
// for the page. cache avoids reparsing unchanged files on every refresh.
func loadFarmRef(game string, cache *farmRefCache) farmRef {
	ref := farmRef{Items: map[int]farmItemRef{}, Nodes: map[int]string{}, Maps: map[int]string{}}
	if game == "" {
		return ref
	}
	farm, k1, err1 := readAddonFile(game, "AHProfit", "FarmData.lua")
	hist, k2, err2 := readAddonFile(game, "AHProfit", "History.lua")
	seed, k3, err3 := readAddonFile(game, "AHProfit", "Data.lua")
	key := filepath.Clean(game) + "\x00" + k1 + "|" + k2 + "|" + k3
	if cache != nil && cache.key == key && k1+k2+k3 != "" {
		return cache.ref
	}
	farmRefParses++
	// A file whose first data row has an unexpected shape (a changed format)
	// means no bundled names at all: wrong names are worse than none.
	var farmD, histD map[string]any
	if err1 == nil {
		farmD, _ = parseLuaAssignedTable(farm, "ns.farmData")
	}
	if err2 == nil {
		histD, _ = parseLuaAssignedTable(hist, "ns.hist")
	}
	if (farmD != nil && !farmDataShapeOK(farmD)) || (histD != nil && !historyShapeOK(histD)) {
		farmD, histD = nil, nil
	}
	if farmD != nil {
		addFarmData(&ref, farmD)
	}
	if d := histD; d != nil {
		{
			for _, row := range List(d["3"]) {
				r := List(row)
				if len(r) < 2 {
					continue
				}
				id, ok := ahpInt(r[1], 1e7)
				name := S(r[0])
				if ok && id > 0 && name != "" && len(name) <= 128 {
					if _, have := ref.Items[id]; !have {
						ref.Items[id] = farmItemRef{Name: name}
					}
				}
			}
		}
	}
	if err3 == nil {
		if d, err := parseLuaAssignedTable(seed, "ns.seed"); err == nil {
			ref.Seed = farmSeedFrom(d)
		}
	}
	if cache != nil {
		cache.key, cache.ref = key, ref
	}
	return ref
}

// farmSeedFrom reads the feed; anything malformed means no feed at all
// (a partly read feed could show prices the game would not).
func farmSeedFrom(d map[string]any) *farmSeed {
	t := ahpTime(d["t"])
	flat := List(d["flat"])
	if t.IsZero() || len(flat) == 0 || len(flat)%3 != 0 {
		return nil
	}
	s := &farmSeed{Time: t, Prices: make(map[int][2]int, len(flat)/3)}
	if f, ok := d["faction"]; ok {
		if s.Faction, ok = f.(string); !ok || (s.Faction != "Alliance" && s.Faction != "Horde") {
			return nil
		}
	}
	for i := 0; i < len(flat); i += 3 {
		id, ok1 := ahpInt(flat[i], 1e7)
		p, ok2 := ahpInt(flat[i+1], 1e12)
		q, ok3 := ahpInt(flat[i+2], 1e9)
		if !ok1 || !ok2 || !ok3 || id == 0 || p == 0 {
			return nil
		}
		s.Prices[id] = [2]int{p, q}
	}
	return s
}

// rowShape reports whether the first row of list has exactly the given
// column kinds: 's' string, 'n' number, 't' table.
func rowShape(list []any, kinds string) bool {
	if len(list) == 0 {
		return true // nothing to read from it anyway
	}
	r := List(list[0])
	if len(r) != len(kinds) {
		return false
	}
	for i, k := range kinds {
		var ok bool
		switch k {
		case 's':
			_, ok = r[i].(string)
		case 'n':
			_, ok = r[i].(float64)
		case 't':
			_, ok = r[i].(map[string]any)
		}
		if !ok {
			return false
		}
	}
	return true
}

// farmDataShapeOK: item and zone rows give values by position (id, vendor,
// mapID), so their column count and kinds must match exactly; node rows give
// only their first column, the name.
func farmDataShapeOK(d map[string]any) bool {
	_, timeOK := d["1"].(float64)
	nodes := List(d["3"])
	nodeOK := len(nodes) == 0
	if !nodeOK {
		r := List(nodes[0])
		_, nodeOK = func() (string, bool) {
			if len(r) == 0 {
				return "", false
			}
			s, ok := r[0].(string)
			return s, ok
		}()
	}
	return timeOK && nodeOK && rowShape(List(d["2"]), "snnnnn") && rowShape(List(d["4"]), "snnnnttt")
}

func historyShapeOK(d map[string]any) bool {
	_, timeOK := d["1"].(float64)
	return timeOK && rowShape(List(d["3"]), "snnnn")
}

// FarmData: { time, ITEMS, NODES, ZONES, ... } with ITEMS[i] = { name,
// itemID, price, listed, vendor, inPriceList }, NODES[i] = { name, ... }
// and ZONES[i] = { name, lo, hi, horde, mapID, ... }.
func addFarmData(ref *farmRef, d map[string]any) {
	for _, row := range List(d["2"]) {
		r := List(row)
		if len(r) < 5 {
			continue
		}
		name := S(r[0])
		id, ok := ahpInt(r[1], 1e7)
		if !ok || id == 0 || name == "" || len(name) > 128 {
			continue
		}
		it := farmItemRef{Name: name}
		if v, ok := ahpInt(r[4], 1e9); ok && v > 0 {
			it.Vendor, it.VendorKnown = v, true
		}
		ref.Items[id] = it
	}
	clash := map[int]bool{}
	for _, row := range List(d["3"]) {
		r := List(row)
		if len(r) < 1 {
			continue
		}
		name := S(r[0])
		if name == "" || len(name) > 128 {
			continue
		}
		h := ahpNodeHash(name)
		if old, have := ref.Nodes[h]; have && old != name {
			clash[h] = true
		}
		ref.Nodes[h] = name
	}
	for h := range clash { // the addon itself cannot tell these apart either
		delete(ref.Nodes, h)
	}
	for _, row := range List(d["4"]) {
		r := List(row)
		if len(r) < 5 {
			continue
		}
		mapID, ok := ahpInt(r[4], 1e7)
		name := S(r[0])
		if ok && mapID > 0 && name != "" && len(name) <= 128 {
			if _, have := ref.Maps[mapID]; !have {
				ref.Maps[mapID] = name
			}
		}
	}
}

// saveItemRefs reads the save's own item cache (AH Profit's items[id] =
// { name, vendor }, saved data version 3). Each field wins over the bundled
// files on its own: a valid save name with no vendor keeps a known bundled
// vendor price, and the other way round.
func saveItemRefs(db map[string]any, into map[int]farmItemRef) {
	for k, raw := range M(db["items"]) {
		id, err := strconv.Atoi(k)
		r := List(raw)
		if err != nil || id <= 0 || id > 1e7 || len(r) < 1 {
			continue
		}
		it, had := into[id]
		changed := false
		if name, isString := r[0].(string); isString && name != "" && len(name) <= 200 { // AH Profit's own cap
			it.Name, changed = name, true
		}
		if len(r) > 1 {
			if v, ok := ahpInt(r[1], 1e10); ok { // AH Profit's own cap
				it.Vendor, it.VendorKnown, changed = v, true, true
			}
		}
		if changed && (had || it.Name != "") {
			into[id] = it
		}
	}
}
