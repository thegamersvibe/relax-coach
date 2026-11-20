package prepread

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

const epoch int64 = 1800000000

func emptyReport(name string, at int64) model.PrepReport {
	return modeltest.Prep(name, "MAGE", 60, 1, "0.2.0").Reason("ready").Clock(at, at).Build()
}

func TestDedupAndPlayedRefresh(t *testing.T) {
	got, d := Parse(fixture(t, "dedup"), epoch, epoch+20)
	{
		want := []model.PrepReport{emptyReport("Alia-Stone-PvP-EU", epoch)}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("dedup reports=%+v diagnostic=%+v", got, d)
		}
	}
	normal, _ := Parse(fixture(t, "normal"), epoch, epoch+20)
	got, d = Parse(fixture(t, "played-refresh"), epoch, epoch+20)
	normal[0].PlayedAtLevelS = model.Some(143)
	if !reflect.DeepEqual(got, normal) {
		t.Fatalf("refresh=%+v want=%+v diagnostic=%+v", got, normal, d)
	}
}

func TestNightBoundsAndEarliestReference(t *testing.T) {
	raw := fixture(t, "dedup")
	for _, tc := range []struct {
		first, end int64
		count      int
	}{
		{epoch, epoch, 1},
		{epoch + 7210, epoch + 7210, 1},
		{epoch + 7211, epoch + 7211, 0},
		{epoch - 1, epoch - 1, 0},
	} {
		got, d := Parse(raw, tc.first, tc.end)
		if len(got) != tc.count || len(d.Warnings) != 0 {
			t.Fatalf("bounds %+v got=%+v diag=%+v", tc, got, d)
		}
		if len(got) > 0 && got[0].Stamped != epoch {
			t.Fatalf("earliest stamp=%d", got[0].Stamped)
		}
	}
	got, d := Parse(fixture(t, "night-eviction"), epoch+3*86400, epoch+3*86400)
	want := []model.PrepReport{emptyReport("Alia-Stone-PvP-EU", epoch+3*86400)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("night before not filtered: %+v (%+v)", got, d)
	}
	for _, bounds := range [][2]int64{{1, 0}, {-1, 1}, {0, 10000000000}} {
		got, d := Parse(raw, bounds[0], bounds[1])
		if len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"invalid fight bounds"}) {
			t.Fatalf("bad bounds: %+v %+v", got, d)
		}
	}
}

func TestAllMarkerMappings(t *testing.T) {
	want := []model.PrepReport{
		emptyReport("Alia-Stone-PvP-EU", epoch),
		modeltest.Prep("Bram-Oak-PvP-EU", "", 0, 1, "").Reason("ready").Clock(epoch+1, epoch+1).ConsumablesUnreadable().GearUnreadable().Build(),
		modeltest.Prep("Cato-PvP-EU", "MAGE", 60, 1, "").Reason("ready").Clock(epoch+2, epoch+2).Played(0).Gear().Build(),
	}
	got, d := Parse(fixture(t, "markers"), epoch, epoch+2)
	if !reflect.DeepEqual(got, want) || len(d.Warnings) != 0 || len(d.Skipped) != 0 {
		t.Fatalf("markers=%+v want=%+v diagnostic=%+v", got, want, d)
	}
}

func TestVersionPolicy(t *testing.T) {
	for _, tc := range []struct{ name, why string }{{"version-one", "store version"}, {"newer", "newer store"}, {"version-one-reloaded", ""}} {
		raw := fixture(t, tc.name)
		before := bytes.Clone(raw)
		got, d := Parse(raw, epoch, epoch+100)
		if len(got) != 0 || !bytes.Equal(raw, before) {
			t.Fatalf("version %s mutated or interpreted: %+v", tc.name, got)
		}
		if tc.why == "" {
			if len(d.Warnings) != 0 {
				t.Fatal(d)
			}
		} else if !reflect.DeepEqual(d.Warnings, []string{tc.why}) {
			t.Fatal(d)
		}
	}
}

func TestRealEvictionsAndChanges(t *testing.T) {
	got, d := Parse(fixture(t, "window-eviction"), epoch, epoch+100)
	want := emptyReport("Alia-Stone-PvP-EU", epoch)
	want.Stamped = epoch + 1
	if !reflect.DeepEqual(got, []model.PrepReport{want}) {
		t.Fatalf("window eviction: %+v %+v", got, d)
	}
	got, d = Parse(fixture(t, "player-eviction"), epoch, epoch+100)
	var players []model.PrepReport
	for i := 1; i <= 40; i++ {
		players = append(players, emptyReport(fmt.Sprintf("Alia%02d-PvP-EU", i), epoch+int64(i)))
	}
	if !reflect.DeepEqual(got, players) {
		t.Fatalf("player eviction: %+v %+v", got, d)
	}
	got, d = Parse(fixture(t, "changes"), epoch, epoch+100)
	changes := []model.PrepReport{emptyReport("Alia-Stone-PvP-EU", epoch)}
	for i, c := range []int64{2, 1} {
		at := epoch + 20 + int64(i)*10
		r := emptyReport("Alia-Stone-PvP-EU", at)
		r.Consumables = []model.PrepConsumable{{Category: "potion", ItemID: 101, Count: c}}
		changes = append(changes, r)
	}
	if !reflect.DeepEqual(got, changes) {
		t.Fatalf("changes: %+v want=%+v %+v", got, changes, d)
	}
}

func TestReadFileReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "RelaxRaid.lua")
	raw := fixture(t, "normal")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, d, err := ReadFile(path, epoch, epoch+10)
	if err != nil || len(got) != 1 || len(d.Warnings) != 0 {
		t.Fatalf("read: %+v %+v %v", got, d, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatalf("modified file: %v", err)
	}
	if _, _, err = ReadFile(path+".missing", epoch, epoch); err == nil {
		t.Fatal("missing file has no IO error")
	}
	if _, _, err = ReadFile(filepath.Dir(path), epoch, epoch); err == nil {
		t.Fatal("directory admitted")
	}
}

func TestLuaBodiesRoundTrip(t *testing.T) {
	files, err := filepath.Glob("testdata/*.lua")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		root, err := parseLiteral(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, nv := range root["prep"].(table)["nights"].(table) {
			for _, bv := range nv.(table)["reports"].(table) {
				for _, v := range bv.(table) {
					body := v.(string)
					r, why := decodeBody(body)
					if why != "" || encodeBody(r) != body {
						t.Fatalf("%s body failed roundtrip: %q (%s)", file, body, why)
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no real addon bodies checked")
	}
	t.Logf("%d real serialized Lua bodies re-encode identically", checked)
}
