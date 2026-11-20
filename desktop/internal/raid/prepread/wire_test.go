package prepread

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

const minimalBody = "1\t0.2.0\tAlia-EU\tMAGE\t60\tu\tready\t1800000000\t-\tn\tn"

func field(body string, at int, value string) string {
	f := strings.Split(body, "\t")
	f[at-1] = value
	return strings.Join(f, "\t")
}

func TestWireGuards(t *testing.T) {
	for _, tc := range []struct {
		at  int
		bad []string
	}{
		{1, []string{"0", "2", "01", "u"}},
		{2, []string{"", strings.Repeat("a", 25), "a|b", "a\x7fb", "a\x01b"}},
		{3, []string{"", strings.Repeat("a", 97), "Alia", "unreadable", " Alia-EU", "Alia-EU ", "Alia  Stone-EU", "Alia_EU", "Alia_Stone-EU", "Alia,EU", "Alia;EU", "Alia|EU", "Alia\x00EU", "Alia\x7fEU"}},
		{4, []string{"", "mage", "DEATHKNIGHT", "u"}},
		{5, []string{"0", "61", "01", "+1", "-1", "1.0", "1e1", "", "unreadable"}},
		{6, []string{"1000000000", "-1", "00", "u0", "unreadable"}},
		{7, []string{"pull", "Ready", "", "u"}},
		{8, []string{"10000000000", "-1", "00", "u", "1e9"}},
		{9, []string{"n", "unreadable", "", "potion,1,0", "potion,0,1", "potion,1000000000,1", "potion,1,1000000", "potion,1,u", "potion,u,1", "fake,1,1", "potion,1,1,2", "potion,1", "potion,1,1;food,1,2", "potion,01,1", strings.Repeat("potion,1,1;", 32) + "potion,2,1"}},
		{10, []string{"not checked", "unreadable", "-", "u", "", "1,0,2"}}, // field 11 remains n
		{11, []string{"-", "u", "", "1", "1,1"}},                           // field 10 remains n
	} {
		for _, bad := range tc.bad {
			body := field(minimalBody, tc.at, bad)
			if _, why := decodeBody(body); why == "" {
				t.Errorf("wire field %d accepted %q", tc.at, bad)
			}
		}
	}
	for _, bad := range []string{strings.Join(strings.Split(minimalBody, "\t")[:10], "\t"), minimalBody + "\t-", strings.Repeat("a", 4097)} {
		if _, why := decodeBody(bad); why == "" {
			t.Errorf("bad width/size accepted")
		}
	}
	if _, why := decodeBody(strings.Repeat("a", 4097)); why != "body size" {
		t.Fatal("body cap not checked before decoding", why)
	}
	checked := field(field(minimalBody, 10, "-"), 11, "-")
	for _, bad := range []string{"0,1,2", "20,1,2", "1,u,2", "1,-1,2", "1,1000000000,2", "1,0,0", "1,0,1000000000", "1,0,2,3", "1,0", strings.TrimSuffix(strings.Repeat("1,0,2;", 20), ";")} {
		if _, why := decodeBody(field(checked, 10, bad)); why == "" {
			t.Errorf("bad gear %q", bad)
		}
	}
	for _, bad := range []string{"0", "20", "01", "1.5", "1,1", "1,", "u", "n", strings.TrimSuffix(strings.Repeat("1,", 20), ",")} {
		if _, why := decodeBody(field(checked, 11, bad)); why == "" {
			t.Errorf("bad notScored %q", bad)
		}
	}
	if _, why := decodeBody(field(field(minimalBody, 10, "u"), 11, "-")); why == "" {
		t.Fatal("unreadable gear mismatch accepted")
	}
}

func TestWireBoundariesAndMarkerRows(t *testing.T) {
	cases := []string{
		minimalBody,
		field(minimalBody, 2, strings.Repeat("v", 24)),
		field(minimalBody, 3, strings.Repeat("a", 93)+"-EU"),
		field(minimalBody, 3, "Alia O'Stone-\xc9U"),
		field(minimalBody, 5, "1"), field(minimalBody, 5, "u"),
		field(minimalBody, 6, "0"), field(minimalBody, 6, "999999999"),
		field(minimalBody, 8, "0"), field(minimalBody, 8, "9999999999"),
		field(minimalBody, 9, "potion,999999999,999999"),
		field(field(minimalBody, 10, "1,0,999999999"), 11, "19,1"),
		field(field(minimalBody, 10, "1,0,2;1,3,4"), 11, "-"), // Lua does not reject duplicate gear slots
	}
	for _, body := range cases {
		r, why := decodeBody(body)
		if why != "" || encodeBody(r) != body {
			t.Fatalf("boundary %q: %s", body, why)
		}
	}
	for _, tc := range []struct {
		gear, ns            string
		checked, unreadable bool
	}{{"n", "n", false, false}, {"u", "u", true, true}, {"-", "-", true, false}} {
		r, why := decodeBody(field(field(minimalBody, 10, tc.gear), 11, tc.ns))
		if why != "" || r.GearChecked != tc.checked || r.GearUnreadable != tc.unreadable || len(r.NotScored) != 0 || len(r.BetterGear) != 0 {
			t.Fatal(r, why)
		}
	}
	r, why := decodeBody(field(field(field(field(minimalBody, 2, "unreadable"), 4, "unreadable"), 5, "u"), 9, "u"))
	if why != "" || r.Level != 0 || r.Class != "" || r.AddonVersion != "" || r.PlayedAtLevelS.Present || !r.ConsumablesUnreadable || len(r.Consumables) != 0 {
		t.Fatal(r, why)
	}
	r, why = decodeBody(field(minimalBody, 6, "0"))
	if why != "" || r.PlayedAtLevelS != model.Some(0) {
		t.Fatal(r, why)
	}
	if nameKey("\xc9LIA-EU") != "\xc9lia-eu" {
		t.Fatal("non-ASCII name folded")
	}
}

func TestWideRealReportsFieldByField(t *testing.T) {
	got, d := Parse(fixture(t, "wide"), epoch, epoch)
	if len(got) != 32 || len(d.Warnings) != 0 || len(d.Skipped) != 0 {
		t.Fatalf("wide reports=%d diagnostic=%+v", len(got), d)
	}
	categories := []string{"generic", "potion", "elixir", "flask", "scroll", "food", "enhancement", "bandage", "other", "vantus", "utility", "combat", "relic", "draught", "device", "temporary"}
	classes := []string{"WARRIOR", "PALADIN", "HUNTER", "ROGUE", "PRIEST", "SHAMAN", "MAGE", "WARLOCK", "DRUID"}
	for i, r := range got {
		want := model.PrepReport{Protocol: 1, Level: int64(i + 1), AddonVersion: strings.Repeat("\\", 24), Name: fmt.Sprintf("Alia%02d-PvP-EU", i+1), Class: classes[i%9], PlayedAtLevelS: model.Some(999999999), Reason: "encounter", SenderClock: 9999999999, Stamped: epoch, GearChecked: true}
		for j := 1; j <= 32; j++ {
			want.Consumables = append(want.Consumables, model.PrepConsumable{Category: categories[(j-1)%16], ItemID: 999999999 - int64(j), Count: 999999})
		}
		for j := 1; j <= 19; j++ {
			worn := int64(999999999)
			if j == 1 {
				worn = 0
			}
			want.BetterGear = append(want.BetterGear, model.PrepGear{Slot: int64(j), WornID: worn, BagID: 999999999})
			want.NotScored = append(want.NotScored, int64(20-j))
		}
		if !reflect.DeepEqual(r, want) {
			t.Fatalf("report %d got=%+v want=%+v", i, r, want)
		}
	}
}

func TestABAUsesStoredIndexIdentity(t *testing.T) {
	got, d := Parse(fixture(t, "aba"), epoch, epoch)
	if len(got) != 3 || len(d.Warnings) != 0 || len(d.Skipped) != 0 {
		t.Fatalf("ABA=%+v diag=%+v", got, d)
	}
	for i, count := range []int64{1, 2, 1} {
		want := emptyReport("Alia-Stone-PvP-EU", epoch)
		want.Consumables = []model.PrepConsumable{{Category: "potion", ItemID: 101, Count: count}}
		if !reflect.DeepEqual(got[i], want) {
			t.Fatalf("ABA[%d]=%+v", i, got[i])
		}
	}
}
