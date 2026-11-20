package main

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestRaidLockRawKeyAndDuplicateRefusals(t *testing.T) {
	valid := `["Alia-PvP-EU"]={result="officer",readAt=1800000000}`
	for _, key := range []string{`[{["Alia-PvP-EU"]=1}]`, `[1]`, `[true]`, `[false]`, `[nil]`, `["NoRealm"]`, `["-Realm"]`, `["Name-"]`, `["Name--Realm"]`} {
		for _, rows := range []string{key + `={result="officer",readAt=1800000000}`, valid + `,` + key + `=false`} {
			if raidOfficer([]byte(`RelaxRaidDB={version=1,ranks={` + rows + `}}`)) {
				t.Errorf("non-name Lua key admitted: %s", key)
			}
		}
	}
	for _, body := range []string{
		`RelaxRaidDB={version=1,ranks={` + valid + `,` + valid + `}}`,
		`RelaxRaidDB={version=1,version=1,ranks={` + valid + `}}`,
		`RelaxRaidDB={version=1,ranks={}} RelaxRaidDB={version=1,ranks={` + valid + `}}`,
		`RelaxRaidDB={version=1,ranks={["Alia-PvP-EU"]={result="member",result="officer",readAt=1800000000}}}`,
		`RelaxRaidDB={version=1,ranks={}} RelaxRaidDB={version=1,ranks={` + valid + `}}`,
	} {
		if raidOfficer([]byte(body)) {
			t.Error("duplicate save key admitted")
		}
	}
	var rows strings.Builder
	for i := 0; i < 51; i++ {
		fmt.Fprintln(&rows, valid+",")
	}
	if raidOfficer([]byte(`RelaxRaidDB={version=1,ranks={` + rows.String() + `}}`)) {
		t.Fatal("51 raw rank entries collapsed into one admitted row")
	}
}

func TestRaidPagePullOutcomeHeader(t *testing.T) {
	r := raidPageFixture()
	r.Fights[0].Boss = true
	r.Fights[0].Success = model.Some(1)
	wipe := r.Fights[0]
	wipe.ID = "second-pull"
	wipe.Success = model.Some(0)
	r.Fights = append(r.Fights, wipe)
	p, err := buildRaidPage(r, logfmt.ParseReport{}, nil, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	html, err := renderRaidBody(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "2 boss pulls · 1 wipes · 1 kills") {
		t.Fatal("header conflates boss pulls, kills or death summaries")
	}
}

func TestRaidPageComparisonNumberDisplay(t *testing.T) {
	report := compare.Report{Ranked: 4, Top: []compare.Entry{{Name: "Alia-PvP-EU", Verdict: "agreement", Part: &compare.PartComparison{Name: "output", Weight: big.NewRat(35, 100), Score: big.NewRat(1, 1), Median: big.NewRat(1, 1), Impact: big.NewRat(-1, 100)}}}, ScoreTop: []compare.Entry{{LoggedName: "Bram-PvP-EU"}}}
	html, err := renderRaidBody(raidPageModel{Comparison: &report})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"(rank 0)", "-0.0", "7/20"} {
		if strings.Contains(html, bad) {
			t.Errorf("unhelpful display: %s", bad)
		}
	}
	if !strings.Contains(html, "configured weight: 35%") || !strings.Contains(html, "0.0 weighted impact") {
		t.Fatal("weight/impact display inconsistent")
	}
}

// Disable duplicate refusal only in this parser probe to isolate the raw count
// from map cardinality. Production enables both guards.
func TestRaidLockRawEntryBudget(t *testing.T) {
	for _, count := range []int{50, 51} {
		body := `RelaxRaidDB={ranks={` + strings.Repeat(`["Alia-PvP-EU"]=false,`, count) + `}}`
		p := luaParser{s: body, maxValues: raidSaveMaxValues, entry: raidRankEntry}
		_, err := p.parse()
		if (err == nil) != (count == 50) {
			t.Fatalf("raw count %d: %v", count, err)
		}
	}
}
