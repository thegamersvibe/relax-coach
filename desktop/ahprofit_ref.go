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
	"strconv"
	"strings"
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

type farmRef struct {
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
	p := &luaParser{s: src, pos: at}
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
	key := k1 + "|" + k2
	if cache != nil && cache.key == key && key != "|" {
		return cache.ref
	}
	farmRefParses++
	if err1 == nil {
		if d, err := parseLuaAssignedTable(farm, "ns.farmData"); err == nil {
			addFarmData(&ref, d)
		}
	}
	if err2 == nil {
		if d, err := parseLuaAssignedTable(hist, "ns.hist"); err == nil {
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
	if cache != nil {
		cache.key, cache.ref = key, ref
	}
	return ref
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
// { name, vendor }, saved data version 3). It wins over the bundled files.
func saveItemRefs(db map[string]any, into map[int]farmItemRef) {
	for k, raw := range M(db["items"]) {
		id, err := strconv.Atoi(k)
		r := List(raw)
		if err != nil || id <= 0 || id > 1e7 || len(r) < 1 {
			continue
		}
		name := S(r[0])
		if _, isString := r[0].(string); !isString || name == "" || len(name) > 128 {
			continue
		}
		it := farmItemRef{Name: name}
		if len(r) > 1 {
			if v, ok := ahpInt(r[1], 1e9); ok {
				it.Vendor, it.VendorKnown = v, true
			}
		}
		into[id] = it
	}
}
