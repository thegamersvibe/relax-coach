package main

// Farming page: reads the optional AH Profit addon's SavedVariables
// (WTF/Account/<selected account>/SavedVariables/AHProfit.lua) for the same
// local account the Coach report uses. Read-only, parsed as Lua literals by
// luasv.go (never executed). No other account is ever used as a fallback.

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ahpMaxBytes caps how much of AHProfit.lua is read, and ahpMaxValues how
// many table entries are parsed. Two auction houses with both price layers
// at AH Profit's 60,000-item cap are about 10 MB and 720,000 values and must
// still be read; anything past either cap is refused.
var (
	ahpMaxBytes  int64 = 16 << 20
	ahpMaxValues       = 1000000
)

// farmParses counts parses of AHProfit.lua (tests check the cache).
var farmParses int

// The addon's bounds on prices received from guild members (Sync.lua): a
// scan timed more than 5 minutes ahead or more than 12 hours ago is ignored.
const (
	ahpGuildAhead  = 300 * time.Second
	ahpGuildBehind = 12 * time.Hour
)

// ahpRowsPerHouse: Sell now shows at most this many rows per auction house.
const ahpRowsPerHouse = 150

// ahpMaxCopper bounds sums so they cannot overflow (far above the gold cap).
const ahpMaxCopper = 1_000_000_000_000_000

// ahpMul and ahpAdd saturate at ahpMaxCopper; inputs are never negative.
func ahpMul(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	if a > ahpMaxCopper/b {
		return ahpMaxCopper
	}
	return min(a*b, ahpMaxCopper)
}

func ahpAdd(a, b int) int { return min(a+b, ahpMaxCopper) }

type farmParseCache struct {
	key string // game, account, size and time of the file parsed
	db  map[string]any
}

// ahpKnownVersion is the newest AHProfitDB.v this app understands. Newer
// versions only add fields, so they are still read, with a note.
const ahpKnownVersion = 2

// ahpOwnFresh: your own scan wins over a guild price while it is this young
// (the same rule as AH Profit's lookup).
const ahpOwnFresh = 2 * time.Hour

type FarmView struct {
	Present      bool   // AHProfit.lua exists for the selected account
	Error        string // could not be read or parsed (Present stays true)
	Warning      string // shown with the last good data
	Note         string // e.g. written by a newer AH Profit
	SaveTime     time.Time
	Cut          int
	Houses       []FarmHouse
	DefaultHouse string
	HasLedger    bool // PR-D fields present
	HasSessions  bool
	Sell         []FarmSellRow
	Gathering    []FarmZone
	Auctions     []FarmAuctionStat
	Listings     []FarmListing
}

type FarmHouse struct {
	Key, Realm, Faction string
	Items               int
	FullScan            time.Time
}

type FarmSellRow struct {
	House, Char, Name string
	ID, Count, Bound  int
	Price, Net, Value int // copper: lowest listing, after the cut, Net × unbound count
	Listed            int
	Source, From      string // "scan" or "guild"
	Seen, BankSeen    time.Time
	Vendor            int // vendor price per unit; 0 = unknown
	VendorKnown       bool
	Gain              int // (Net − Vendor) × unbound, only when VendorKnown
	NoPrice           bool
	AllBound          bool // every item of the stack is bound: nothing to auction
}

type FarmZone struct {
	MapID, Nodes, Kinds int
	Name                string
	KindNames           []string // node kinds whose name is known
	Last                time.Time
	Who                 []string
}

type FarmAuctionStat struct {
	Name          string
	Sold, Expired int
	RatePct       int
}

type FarmListing struct {
	Char                string
	Seen                time.Time
	Active, SoldWaiting int
	ActiveAfterCut      int
	BidOnly             int // listed without a buyout: proceeds unknown, not counted
}

// readAHProfitLocked returns the raw save of the selected account, or
// fs.ErrNotExist when that account has none. The key names the file version
// (game, account, size, time); when it equals known the file is not read and
// data is nil. Callers hold a.mu.
func (a *App) readAHProfitLocked(known string) ([]byte, time.Time, string, error) {
	if a.game == "" || !validAccountID(a.account) {
		return nil, time.Time{}, "", fs.ErrNotExist
	}
	available := false
	for _, account := range a.accounts {
		if account.ID == a.account {
			available = true
			break
		}
	}
	if !available {
		return nil, time.Time{}, "", fs.ErrNotExist
	}
	root, err := openAccountsRoot(a.game)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	defer root.Close()
	f, err := openAccountFile(root, a.account, "AHProfit.lua")
	if err != nil {
		return nil, time.Time{}, "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, "", err
	}
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%d", a.game, a.account, st.Size(), st.ModTime().UnixNano())
	if st.Size() > ahpMaxBytes {
		return nil, st.ModTime(), key, fmt.Errorf("AHProfit.lua is larger than %d MB; not read", ahpMaxBytes>>20)
	}
	if key == known {
		return nil, st.ModTime(), key, nil
	}
	data, err := io.ReadAll(io.LimitReader(f, ahpMaxBytes+1))
	if err == nil && int64(len(data)) > ahpMaxBytes {
		err = fmt.Errorf("AHProfit.lua is larger than %d MB; not read", ahpMaxBytes>>20)
	}
	return data, st.ModTime(), key, err
}

// loadFarmLocked fills v.Farm. A read or parse failure keeps the previous
// good farm data of the same game and account, with a warning.
func (a *App) loadFarmLocked(v *View, prev View) {
	data, mod, key, err := a.readAHProfitLocked(a.farmParse.key)
	if errors.Is(err, fs.ErrNotExist) {
		return // page hidden
	}
	var fv FarmView
	if err == nil {
		// An unchanged file (same size and time) is not read or parsed again;
		// the page is still rebuilt, since ages and the Coach data move on.
		db := a.farmParse.db
		if data != nil || key != a.farmParse.key {
			a.farmParse = farmParseCache{}
			if db, err = parseFarm(data); err == nil {
				a.farmParse = farmParseCache{key: key, db: db}
			}
		}
		if err == nil {
			fv = buildFarmDB(db, v.Chars, time.Now(), loadFarmRef(a.game, &a.farmRef))
		}
	}
	if err != nil {
		if prev.Farm.Present && prev.Farm.Error == "" && prev.Game == v.Game && prev.Account == v.Account {
			v.Farm = prev.Farm
			v.Farm.Warning = "Latest AH Profit save couldn't be read yet, showing the previous one (" + err.Error() + ")"
			return
		}
		v.Farm = FarmView{Present: true, Error: err.Error()}
		return
	}
	fv.SaveTime = mod
	v.Farm = fv
}

func ahpInt(v any, max float64) (int, bool) {
	f, ok := v.(float64)
	if !ok || f < 0 || f > max || f != math.Floor(f) {
		return 0, false
	}
	return int(f), true
}

func ahpTime(v any) time.Time {
	if n, ok := ahpInt(v, 1e10); ok && n > 0 {
		return time.Unix(int64(n), 0)
	}
	return time.Time{}
}

// buildFarm turns a parsed AHProfit.lua into the page model. chars are the
// Coach report's characters of the same account, used only to tell which
// auction house (realm and faction) a character belongs to.
func buildFarm(data []byte, chars []CharView, now time.Time, ref farmRef) (FarmView, error) {
	db, err := parseFarm(data)
	if err != nil {
		return FarmView{}, err
	}
	return buildFarmDB(db, chars, now, ref), nil
}

// parseFarm parses AHProfit.lua within the byte and value caps.
func parseFarm(data []byte) (map[string]any, error) {
	if int64(len(data)) > ahpMaxBytes {
		return nil, fmt.Errorf("AHProfit.lua is larger than %d MB; not read", ahpMaxBytes>>20)
	}
	farmParses++
	vars, err := ParseSavedVariablesLimited(string(data), ahpMaxValues)
	if err != nil {
		return nil, err
	}
	db := M(vars["AHProfitDB"])
	if db == nil {
		return nil, errors.New("no AHProfitDB in AHProfit.lua")
	}
	return db, nil
}

func buildFarmDB(db map[string]any, chars []CharView, now time.Time, ref farmRef) FarmView {
	now = now.Truncate(time.Second) // whole seconds, like the game's GetServerTime
	fv := FarmView{Present: true, Cut: 5}
	// Names and vendor prices: the save's own item cache first, then AH
	// Profit's bundled files, then "item <id>" with no vendor claim.
	items := map[int]farmItemRef{}
	for id, it := range ref.Items {
		items[id] = it
	}
	saveItemRefs(db, items)
	if c, ok := ahpInt(db["cut"], 100); ok && (c == 5 || c == 15) {
		fv.Cut = c
	}
	if ver, ok := ahpInt(db["v"], 1e6); ok && ver > ahpKnownVersion {
		fv.Note = "This save was written by a newer AH Profit; parts it added are not shown yet. Update Relax Coach."
	}
	_, fv.HasLedger = db["ledger"].(map[string]any)
	_, fv.HasSessions = db["sessions"].(map[string]any)

	live, shared := M(db["live"]), M(db["shared"])
	// Guild prices only from a saved-data version that has them, as the addon reads them.
	if v, ok := ahpInt(db["v"], 1e6); !ok || v < 1 || v > 3 {
		shared = nil
	}
	for key, raw := range live {
		L := M(raw)
		realm, faction, ok := strings.Cut(key, "|")
		if L == nil || !ok || realm == "" {
			continue
		}
		fv.Houses = append(fv.Houses, FarmHouse{Key: key, Realm: realm, Faction: faction, Items: len(M(L["p"])), FullScan: ahpTime(L["full"])})
	}
	sort.Slice(fv.Houses, func(i, j int) bool { return fv.Houses[i].Key < fv.Houses[j].Key })

	// Which house a bank character trades on: only the Coach character with
	// the same name and realm says (its faction). Without one the house is
	// unknown and the row unpriced; never a guess.
	houseOf := func(char string) string {
		name, realm, ok := strings.Cut(char, "-")
		if !ok {
			return ""
		}
		for _, c := range chars {
			if c.Name == name && c.Realm == realm && c.Faction != "" {
				return realm + "|" + c.Faction
			}
		}
		return ""
	}
	// The most recently saved Coach character picks the house shown first.
	var newest time.Time
	for _, c := range chars {
		if c.Realm != "" && c.Faction != "" && c.Updated.After(newest) {
			newest, fv.DefaultHouse = c.Updated, c.Realm+"|"+c.Faction
		}
	}
	if fv.DefaultHouse == "" && len(fv.Houses) == 1 {
		fv.DefaultHouse = fv.Houses[0].Key
	}

	// The same order as AH Profit's lookup: your own scan under 2 hours old;
	// then a guild scan newer than yours; then your older scan; then the
	// bundled feed (Data.lua) of the house's faction. A scan older than the
	// feed loses to it, and a complete scan newer than the feed hides it.
	// Guild prices outside the addon's time bounds are ignored.
	guildOK := func(t time.Time) bool {
		return !t.IsZero() && !t.After(now.Add(ahpGuildAhead)) && !t.Before(now.Add(-ahpGuildBehind))
	}
	price := func(house string, id int) (p, q int, t time.Time, src, from string) {
		k := strconv.Itoa(id)
		L, H := M(live[house]), M(shared[house])
		lp, okL := ahpInt(M(L["p"])[k], 1e12)
		lt := ahpTime(M(L["t"])[k])
		gp, okG := ahpInt(M(H["p"])[k], 1e12)
		gt := ahpTime(M(H["t"])[k])
		okL, okG = okL && lp > 0 && !lt.IsZero(), okG && gp > 0 && guildOK(gt)
		if okL && now.Sub(lt) <= ahpOwnFresh {
			q, _ = ahpInt(M(L["q"])[k], 1e9)
			return lp, q, lt, "scan", ""
		}
		var sp, sq int
		var st time.Time
		okS := false
		if s := ref.Seed; s != nil {
			_, faction, _ := strings.Cut(house, "|")
			if e, have := s.Prices[id]; have && (s.Faction == "" || s.Faction == faction) {
				sp, sq, st, okS = e[0], e[1], s.Time, true
			}
		}
		if okS {
			full := ahpTime(L["full"])
			if hf := ahpTime(H["full"]); guildOK(hf) && hf.After(full) {
				full = hf
			}
			switch {
			case !full.IsZero() && !full.Before(st):
				okS = false
			default:
				if okL && lt.Before(st) {
					okL = false
				}
				if okG && gt.Before(st) {
					okG = false
				}
			}
		}
		if okG && (!okL || gt.After(lt)) {
			q, _ = ahpInt(M(H["q"])[k], 1e9)
			return gp, q, gt, "guild", S(M(H["from"])[strconv.FormatInt(gt.Unix(), 10)])
		}
		if okL {
			q, _ = ahpInt(M(L["q"])[k], 1e9)
			return lp, q, lt, "scan", ""
		}
		if okS {
			return sp, sq, st, "feed", ""
		}
		return 0, 0, time.Time{}, "", ""
	}

	// 3. Sell now: what is in each character's bank, at its house's price.
	for char, raw := range M(db["bank"]) {
		b := M(raw)
		house := houseOf(char)
		seen := ahpTime(b["t"])
		for _, it := range List(b["items"]) {
			row := List(it)
			if len(row) < 2 {
				continue
			}
			id, ok1 := ahpInt(row[0], 1e7)
			n, ok2 := ahpInt(row[1], 1e6)
			bound := 0
			if len(row) > 2 {
				bound, _ = ahpInt(row[2], 1e6)
			}
			if !ok1 || !ok2 || id == 0 || n == 0 || bound > n {
				continue
			}
			r := FarmSellRow{House: house, Char: char, ID: id, Name: "item " + strconv.Itoa(id), Count: n, Bound: bound, BankSeen: seen, AllBound: bound == n}
			if it, ok := items[id]; ok {
				r.Name, r.Vendor, r.VendorKnown = it.Name, it.Vendor, it.VendorKnown
			}
			if house != "" {
				r.Price, r.Listed, r.Seen, r.Source, r.From = price(house, id)
			}
			if r.Price == 0 {
				r.NoPrice = true
			} else {
				r.Net = r.Price * (100 - fv.Cut) / 100
				r.Value = ahpMul(r.Net, n-bound)
				if r.VendorKnown && !r.AllBound {
					if r.Net >= r.Vendor {
						r.Gain = ahpMul(r.Net-r.Vendor, n-bound)
					} else {
						r.Gain = -ahpMul(r.Vendor-r.Net, n-bound)
					}
				}
			}
			fv.Sell = append(fv.Sell, r)
		}
	}
	sort.Slice(fv.Sell, func(i, j int) bool {
		a, b := fv.Sell[i], fv.Sell[j]
		if a.Value != b.Value {
			return a.Value > b.Value
		}
		if a.Char != b.Char {
			return a.Char < b.Char
		}
		return a.ID < b.ID
	})
	// At most ahpRowsPerHouse rows per house (the most valuable), so one
	// large house cannot push another out of the list.
	perHouse, kept := map[string]int{}, fv.Sell[:0]
	for _, r := range fv.Sell {
		if perHouse[r.House] < ahpRowsPerHouse {
			perHouse[r.House]++
			kept = append(kept, r)
		}
	}
	fv.Sell = kept
	// Every house that has rows can be chosen, also one known from Coach but
	// never scanned (listed as "not scanned").
	listed := map[string]bool{}
	for _, h := range fv.Houses {
		listed[h.Key] = true
	}
	for _, r := range fv.Sell {
		if r.House != "" && !listed[r.House] {
			listed[r.House] = true
			realm, faction, _ := strings.Cut(r.House, "|")
			fv.Houses = append(fv.Houses, FarmHouse{Key: r.House, Realm: realm, Faction: faction})
		}
	}
	sort.Slice(fv.Houses, func(i, j int) bool { return fv.Houses[i].Key < fv.Houses[j].Key })

	// 4. Gathering: nodes per map, how many kinds, when last seen, who picked.
	for mapKey, raw := range M(db["nodes"]) {
		mapID, err := strconv.Atoi(mapKey)
		if err != nil || mapID <= 0 {
			continue
		}
		z := FarmZone{MapID: mapID, Name: "Map " + mapKey}
		if name, ok := ref.Maps[mapID]; ok {
			z.Name = name
		}
		kinds, who := map[int]bool{}, map[string]bool{}
		for _, nraw := range List(raw) {
			n := List(nraw)
			if len(n) < 4 {
				continue
			}
			h, ok := ahpInt(n[0], 1e7)
			if !ok {
				continue
			}
			z.Nodes++
			kinds[h] = true
			if t := ahpTime(n[3]); t.After(z.Last) {
				z.Last = t
			}
			if len(n) > 4 {
				if w := S(n[4]); w != "" && len(w) <= 64 {
					who[w] = true
				}
			}
		}
		if z.Nodes == 0 {
			continue
		}
		z.Kinds = len(kinds)
		for h := range kinds {
			if name, ok := ref.Nodes[h]; ok {
				z.KindNames = append(z.KindNames, name)
			}
		}
		sort.Strings(z.KindNames)
		for w := range who {
			z.Who = append(z.Who, w)
		}
		sort.Strings(z.Who)
		fv.Gathering = append(fv.Gathering, z)
	}
	sort.Slice(fv.Gathering, func(i, j int) bool {
		if fv.Gathering[i].Nodes != fv.Gathering[j].Nodes {
			return fv.Gathering[i].Nodes > fv.Gathering[j].Nodes
		}
		return fv.Gathering[i].MapID < fv.Gathering[j].MapID
	})

	// 5. Auctions: sold/expired per item as the game reported them, and the
	// listings each character last saw on the Auctions tab.
	auc := M(db["auc"])
	for name, raw := range M(auc["stat"]) {
		s := List(raw)
		if name == "" || len(name) > 128 || len(s) < 2 {
			continue
		}
		sold, ok1 := ahpInt(s[0], 1e7)
		exp, ok2 := ahpInt(s[1], 1e7)
		if !ok1 || !ok2 || sold+exp == 0 {
			continue
		}
		fv.Auctions = append(fv.Auctions, FarmAuctionStat{Name: name, Sold: sold, Expired: exp, RatePct: int(math.Round(100 * float64(sold) / float64(sold+exp)))})
	}
	sort.Slice(fv.Auctions, func(i, j int) bool {
		a, b := fv.Auctions[i], fv.Auctions[j]
		if a.Sold+a.Expired != b.Sold+b.Expired {
			return a.Sold+a.Expired > b.Sold+b.Expired
		}
		return a.Name < b.Name
	})
	for char, raw := range M(auc["own"]) {
		o := M(raw)
		l := FarmListing{Char: char, Seen: ahpTime(o["t"])}
		for _, araw := range List(o["list"]) {
			x := List(araw)
			if len(x) < 4 {
				continue
			}
			q, ok1 := ahpInt(x[1], 1e6)
			each, ok2 := ahpInt(x[2], 1e12)
			if !ok1 || !ok2 {
				continue
			}
			if N(x[3]) == 1 {
				l.SoldWaiting++
			} else if each == 0 {
				l.BidOnly++
			} else {
				l.Active++
				l.ActiveAfterCut = ahpAdd(l.ActiveAfterCut, ahpMul(each*(100-fv.Cut)/100, q)) // per unit after the cut, as AH Profit shows it
			}
		}
		if l.Active+l.SoldWaiting+l.BidOnly > 0 {
			fv.Listings = append(fv.Listings, l)
		}
	}
	sort.Slice(fv.Listings, func(i, j int) bool { return fv.Listings[i].Char < fv.Listings[j].Char })
	return fv
}

// coin formats copper the way AH Profit does: "1g 23s 45c", "23s 0c", "7c".
func coin(c int) string {
	neg := c < 0
	if neg {
		c = -c
	}
	g, sv, k := c/10000, c/100%100, c%100
	out := ""
	if g > 0 {
		out = ahpCommas(g) + "g "
	}
	if g > 0 || sv > 0 {
		out += strconv.Itoa(sv) + "s "
	}
	out += strconv.Itoa(k) + "c"
	if neg {
		return "-" + out
	}
	return out
}

func ahpCommas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

var farmTemplate = template.Must(template.New("farm.html").Funcs(funcs).ParseFS(assets, "farm.html"))

// renderFarmBody renders the Farming page fragment. Save text stays escaped.
func renderFarmBody(v View) (string, error) {
	if !v.Farm.Present {
		return "", nil
	}
	var out strings.Builder
	err := farmTemplate.ExecuteTemplate(&out, "farm-body", v.Farm)
	return out.String(), err
}
