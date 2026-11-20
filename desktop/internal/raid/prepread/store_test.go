package prepread

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Test-only serializer for hostile variants. Positive fixtures come from Lua.
func lua(v any) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case string:
		return strconv.Quote(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case table:
		rows := make([]string, 0, len(v))
		for k, x := range v {
			rows = append(rows, "["+lua(k)+"]="+lua(x))
		}
		sort.Strings(rows)
		return "{" + strings.Join(rows, ",") + "}"
	default:
		panic(fmt.Sprintf("unsupported test type %T", v))
	}
}

func rootFixture(t *testing.T, name string) table {
	t.Helper()
	root, err := parseLiteral(fixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func firstNight(root table) table  { return root["prep"].(table)["nights"].(table)[float64(1)].(table) }
func firstWindow(root table) table { return firstNight(root)["windows"].(table)[float64(1)].(table) }
func row(root table) table         { return firstWindow(root)["reports"].(table)["alia-stone-pvp-eu"].(table) }
func bodies(root table) table {
	return firstNight(root)["reports"].(table)["alia-stone-pvp-eu"].(table)
}
func rawRoot(root table) []byte { return []byte("RelaxRaidDB=" + lua(root)) }

func TestStoreStructureGuards(t *testing.T) {
	tests := []struct {
		name, why string
		change    func(table)
	}{
		{"root version", "account version", func(r table) { r["version"] = "1" }},
		{"root missing", "account version", func(r table) { delete(r, "version") }},
		{"root future", "newer store", func(r table) { r["version"] = float64(2) }},
		{"root older", "account version", func(r table) { r["version"] = float64(0) }},
		{"prep absent", "store type", func(r table) { delete(r, "prep") }},
		{"prep version string", "store version", func(r table) { r["prep"].(table)["version"] = "2" }},
		{"prep version absent", "store version", func(r table) { delete(r["prep"].(table), "version") }},
		{"prep version fractional", "store version", func(r table) { r["prep"].(table)["version"] = 1.5 }},
		{"nights type", "nights shape or cap", func(r table) { r["prep"].(table)["nights"] = true }},
		{"nights string index", "nights shape or cap", func(r table) {
			ns := r["prep"].(table)["nights"].(table)
			ns["1"] = ns[float64(1)]
			delete(ns, float64(1))
		}},
		{"nights hole", "nights shape or cap", func(r table) {
			ns := r["prep"].(table)["nights"].(table)
			ns[float64(2)] = ns[float64(1)]
			delete(ns, float64(1))
		}},
		{"nights cap", "nights shape or cap", func(r table) {
			ns := r["prep"].(table)["nights"].(table)
			for i := 2; i <= 4; i++ {
				n := firstNight(r)
				ns[float64(i)] = table{"day": n["day"].(float64) + float64(i), "windows": n["windows"], "reports": n["reports"]}
			}
		}},
		{"night type", "night type", func(r table) { r["prep"].(table)["nights"].(table)[float64(1)] = "bad" }},
		{"night day", "night day", func(r table) { firstNight(r)["day"] = float64(-2) }},
		{"night duplicate", "night day", func(r table) { r["prep"].(table)["nights"].(table)[float64(2)] = firstNight(r) }},
		{"windows cap", "windows shape or cap", func(r table) {
			ws := firstNight(r)["windows"].(table)
			for i := 2; i <= 81; i++ {
				ws[float64(i)] = firstWindow(r)
			}
		}},
		{"windows type", "windows shape or cap", func(r table) { firstNight(r)["windows"] = "bad" }},
		{"windows empty", "windows shape or cap", func(r table) { firstNight(r)["windows"] = table{} }},
		{"window type", "window type", func(r table) { firstNight(r)["windows"].(table)[float64(1)] = true }},
		{"window clock", "window clock", func(r table) { firstWindow(r)["openedAt"] = "5" }},
		{"window clock high", "window clock", func(r table) { firstWindow(r)["openedAt"] = float64(10000000000) }},
		{"rows empty", "characters shape or cap", func(r table) { firstWindow(r)["reports"] = table{} }},
		{"rows type", "characters shape or cap", func(r table) { firstWindow(r)["reports"] = false }},
		{"rows cap", "characters shape or cap", func(r table) {
			rs := firstWindow(r)["reports"].(table)
			for i := 1; i <= 40; i++ {
				key := fmt.Sprintf("alia%d-eu", i)
				rs[key] = row(r)
				firstNight(r)["reports"].(table)[key] = table{float64(1): field(minimalBody, 3, key)}
			}
		}},
		{"key uppercase", "character key", func(r table) { rs := firstWindow(r)["reports"].(table); rs["Alia-EU"] = row(r) }},
		{"key number", "character key", func(r table) { firstWindow(r)["reports"].(table)[float64(1)] = row(r) }},
		{"key bare", "character key", func(r table) { firstWindow(r)["reports"].(table)["alia"] = row(r) }},
		{"reference type", "reference type", func(r table) { firstWindow(r)["reports"].(table)["alia-stone-pvp-eu"] = true }},
		{"reference clock", "reference clock", func(r table) { row(r)["receivedAt"] = "1" }},
		{"reference fraction", "reference clock", func(r table) { row(r)["receivedAt"] = 0.5 }},
		{"reference negative", "reference clock", func(r table) { row(r)["receivedAt"] = float64(-1) }},
		{"reference high", "reference clock", func(r table) { row(r)["receivedAt"] = float64(10000000000) }},
		{"dangling", "dangling reference", func(r table) { row(r)["report"] = float64(2) }},
		{"zero index", "dangling reference", func(r table) { row(r)["report"] = float64(0) }},
		{"fraction index", "dangling reference", func(r table) { row(r)["report"] = 1.5 }},
		{"string index", "dangling reference", func(r table) { row(r)["report"] = "1" }},
		{"reports type", "reports shape or cap", func(r table) { firstNight(r)["reports"] = true }},
		{"bodies type", "bodies shape or cap", func(r table) { firstNight(r)["reports"].(table)["alia-stone-pvp-eu"] = true }},
		{"bodies hole", "bodies shape or cap", func(r table) { bs := bodies(r); bs[float64(2)] = bs[float64(1)]; delete(bs, float64(1)) }},
		{"bodies cap", "bodies shape or cap", func(r table) {
			bs := bodies(r)
			for i := 2; i <= 81; i++ {
				bs[float64(i)] = bs[float64(1)]
			}
		}},
		{"body type", "body type", func(r table) { bodies(r)[float64(1)] = table{} }},
		{"orphan body", "unreferenced body", func(r table) { bs := bodies(r); bs[float64(2)] = bs[float64(1)] }},
		{"orphan character", "unreferenced body", func(r table) { firstNight(r)["reports"].(table)["bram-eu"] = table{float64(1): minimalBody} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := rootFixture(t, "normal")
			tc.change(r)
			got, d := Parse(rawRoot(r), epoch, epoch+100)
			if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{tc.why}) {
				t.Fatalf("got %d reports, %+v; want %s", len(got), d, tc.why)
			}
		})
	}
}

func TestNewerVersionsAreNeverInterpreted(t *testing.T) {
	for _, v := range []float64{1.5, 2} {
		r := rootFixture(t, "normal")
		r["version"] = v
		r["prep"].(table)["nights"] = "opaque"
		got, d := Parse(rawRoot(r), epoch, epoch)
		if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"newer store"}) {
			t.Fatalf("account version=%v diag=%+v", v, d)
		}
	}
	for _, v := range []float64{2.5, 3, 99999999999} {
		for _, account := range []bool{false, true} {
			r := rootFixture(t, "normal")
			if account {
				r["version"] = v
			} else {
				r["prep"].(table)["version"] = v
			}
			if !account {
				r["version"] = "malformed root version"
			}
			// Deliberately invalid future layout. Nothing below version may be guessed.
			r["prep"].(table)["nights"] = "opaque"
			got, d := Parse(rawRoot(r), epoch, epoch)
			if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"newer store"}) {
				t.Fatalf("version=%v account=%v diagnostic=%+v", v, account, d)
			}
		}
	}
}

func TestInvalidBodiesCountOnceAndDoNotBlankValidOnes(t *testing.T) {
	for _, replacement := range []string{
		"bad", strings.Join(strings.Split(minimalBody, "\t")[:10], "\t"), minimalBody + "\t-",
		field(minimalBody, 3, "Forged-EU"), field(minimalBody, 11, "-"),
	} {
		root := rootFixture(t, "dedup")
		bodies(root)[float64(1)] = replacement
		n := firstNight(root)
		n["reports"].(table)["bram-eu"] = table{float64(1): field(minimalBody, 3, "Bram-EU")}
		firstWindow(root)["reports"].(table)["bram-eu"] = table{"receivedAt": float64(epoch), "report": float64(1)}
		got, d := Parse(rawRoot(root), epoch, epoch+100)
		count := 0
		for _, n := range d.Skipped {
			count += n
		}
		if len(got) != 1 || got[0].Name != "Bram-EU" || count != 1 || len(d.Warnings) != 0 {
			t.Fatalf("bad body=%q reports=%+v diag=%+v", replacement, got, d)
		}
	}
}

// Real wide Lua body -> boundary variants. Only hostile/size controls use this
// serializer; it does not manufacture the committed addon-output fixtures.
func budgetStore(t *testing.T, characters, windows, size int) table {
	t.Helper()
	wide := firstNight(rootFixture(t, "wide"))["reports"].(table)["alia01-pvp-eu"].(table)[float64(1)].(string)
	base, why := decodeBody(wide)
	if why != "" {
		t.Fatal(why)
	}
	base.Name = "Alia00-EU"
	for len(encodeBody(base)) > size {
		base.Consumables = base.Consumables[:len(base.Consumables)-1]
	}
	pad := strings.Repeat("Z", size-len(encodeBody(base)))
	ns, ws := table{}, table{}
	for w := 1; w <= windows; w++ {
		ws[float64(w)] = table{"openedAt": float64(epoch), "reports": table{}}
	}
	for c := 1; c <= characters; c++ {
		base.Name = fmt.Sprintf("Alia%02d", c) + pad + "-EU"
		key := nameKey(base.Name)
		bs := table{}
		for w := 1; w <= windows; w++ {
			base.Consumables[0].Count = 999900 + int64(w)
			body := encodeBody(base)
			if len(body) != size {
				t.Fatalf("boundary body %d != %d", len(body), size)
			}
			if _, why := decodeBody(body); why != "" {
				t.Fatal(why)
			}
			bs[float64(w)] = body
			ws[float64(w)].(table)["reports"].(table)[key] = table{"receivedAt": float64(epoch), "report": float64(w)}
		}
		ns[key] = bs
	}
	return table{"version": float64(1), "prep": table{"version": float64(2), "nights": table{float64(1): table{"day": float64(20832), "reports": ns, "windows": ws}}}}
}

func TestCharacterByteBudget(t *testing.T) {
	root := budgetStore(t, 1, 80, 1250)
	got, d := Parse(rawRoot(root), epoch, epoch)
	if len(got) != 80 || len(d.Warnings) != 0 {
		t.Fatalf("exact character budget: %d %+v", len(got), d)
	}
	reports := firstNight(root)["reports"].(table)
	for _, v := range reports {
		bs := v.(table)
		body := bs[float64(80)].(string)
		// 24-byte version stays bounded: add a byte to the name and key instead
		// would change all bodies. Here change level 1 to 10 in one body.
		bs[float64(80)] = field(body, 5, "10")
	}
	got, d = Parse(rawRoot(root), epoch, epoch)
	if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"character byte budget"}) {
		t.Fatalf("over character budget: %d %+v", len(got), d)
	}
}

func TestNightByteBudget(t *testing.T) {
	root := budgetStore(t, 40, 80, 1250)
	got, d := Parse(rawRoot(root), epoch, epoch)
	if len(got) != 3200 || len(d.Warnings) != 0 {
		t.Fatalf("exact night budget: %d %+v", len(got), d)
	}
	n := firstNight(root)
	rs := n["reports"].(table)
	names, _ := keys(rs)
	last := names[len(names)-1]
	bs := rs[last].(table)
	body := bs[float64(80)].(string)
	delete(bs, float64(80))
	rows := n["windows"].(table)[float64(80)].(table)["reports"].(table)
	delete(rows, last)
	// One 1251-byte replacement holder: no per-character or player cap masks
	// the NIGHT budget. Preserve name width and change level's byte width.
	body = field(field(body, 3, "Z"+strings.Split(body, "\t")[2][1:]), 5, "10")
	key := nameKey(strings.Split(body, "\t")[2])
	rs[key] = table{float64(1): body}
	rows[key] = table{"receivedAt": float64(epoch), "report": float64(1)}
	got, d = Parse(rawRoot(root), epoch, epoch)
	if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"night byte budget"}) {
		t.Fatalf("over night budget: %d %+v", len(got), d)
	}
}

func TestSortByReceiptThenExactName(t *testing.T) {
	root := rootFixture(t, "markers")
	rows := firstWindow(root)["reports"].(table)
	rows["cato-pvp-eu"].(table)["receivedAt"] = float64(epoch)
	for repeat := 0; repeat < 20; repeat++ {
		got, d := Parse(rawRoot(root), epoch, epoch+2)
		if len(got) != 3 || got[0].Name != "Alia-Stone-PvP-EU" || got[1].Name != "Cato-PvP-EU" || got[2].Name != "Bram-Oak-PvP-EU" {
			t.Fatalf("order: %+v %+v", got, d)
		}
	}
	rows["cato-pvp-eu"].(table)["receivedAt"] = float64(epoch + 1)
	got, _ := Parse(rawRoot(root), epoch, epoch+2)
	if got[1].Name != "Bram-Oak-PvP-EU" || got[2].Name != "Cato-PvP-EU" {
		t.Fatal("name tie order")
	}
}
