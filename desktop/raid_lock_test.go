package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func raidRankSave(t *testing.T, game, account, body string) string {
	t.Helper()
	p := filepath.Join(game, "WTF", "Account", account, "SavedVariables", "RelaxRaid.lua")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func raidOfficer(data []byte) bool { return raidAdmission(data, raidTestClock()) == nil }
func officerSave() string {
	return `RelaxRaidDB={version=1,ranks={["Alia-Stone-PvP-EU"]={result="officer",guild="Invented Guild",readAt=1800000000}}}`
}
func TestRaidLockBudgetAndOptionalGuild(t *testing.T) {
	for _, guild := range []string{"", `guild="",`, `guild="Invented Guild",`} {
		data := []byte(`RelaxRaidDB={version=1,ranks={["Alia-PvP-EU"]={result="officer",` + guild + `readAt=1800000000}}}`)
		if !raidOfficer(data) {
			t.Fatal("optional string guild rejected")
		}
	}
	if raidOfficer([]byte(officerSave() + strings.Repeat(" ", (16<<20)+1))) {
		t.Fatal("byte budget ignored")
	}
	for _, s := range []string{strings.Replace(officerSave(), "readAt=1800000000", "readAt=0", 1), strings.Replace(officerSave(), "readAt=1800000000", "readAt=1e100", 1), strings.Replace(officerSave(), `guild="Invented Guild"`, `guild={}`, 1), strings.Replace(officerSave(), `["Alia-Stone-PvP-EU"]`, `["bad\nname"]`, 1), officerSave() + ` os.execute("bad")`} {
		if raidOfficer([]byte(s)) {
			t.Fatal("malformed save accepted")
		}
	}
}
func TestRaidLockRenderFixture(t *testing.T) {
	html, err := renderRaidShell()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `id="raids-view"`) {
		t.Fatal("officer shell missing")
	}
	if path := os.Getenv("NATIVE_HTML_OUT"); path != "" {
		if err := os.WriteFile(path+".raid-shell", []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRaidLockSelectedAccount(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	accountSave(t, game, "B#2", "Beta")
	raidRankSave(t, game, "B#2", officerSave())
	a, b := raidLockApp(t, game, raidTestClock)
	for _, tc := range []struct {
		account string
		allowed bool
	}{{"A#1", false}, {"B#2", true}, {"A#1", false}} {
		if _, err := b.Call("selectAccount", nativeArgs(tc.account)); err != nil {
			t.Fatal(err)
		}
		p := accountProjection(t, b)
		if p["HasRaids"] != tc.allowed {
			t.Fatalf("%s HasRaids=%v want %v", tc.account, p["HasRaids"], tc.allowed)
		}
		html, err := renderNativeShell(a.snapshot(), true, "lock-test")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{`id="nav-raids"`, `id="raids-view"`} {
			if strings.Contains(html, id) != tc.allowed {
				t.Fatalf("rendered %s: %v", id, tc.allowed)
			}
		}
		result, err := b.Call("raidPage", nil)
		if tc.allowed {
			if err != nil || result == nil {
				t.Fatalf("officer route: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "locked") || result != nil {
			t.Fatalf("locked route returned data: %v %v", result, err)
		}
	}
}
func TestRaidLockInvalidAndRevocation(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	p := raidRankSave(t, game, "A#1", officerSave())
	_, b := raidLockApp(t, game, raidTestClock)
	bad := []string{"", `RelaxRaidDB={version=1}`, `RelaxRaidDB={version=2,ranks={}}`, strings.Replace(officerSave(), `"officer"`, `"member"`, 1), strings.Replace(officerSave(), `"officer"`, `"noguild"`, 1), strings.Replace(officerSave(), `"officer"`, `"unreadable"`, 1), strings.Replace(officerSave(), `"officer"`, `"pending"`, 1), strings.Replace(officerSave(), `"officer"`, `true`, 1), strings.Replace(officerSave(), `1800000000`, `"1800000000"`, 1), strings.Replace(officerSave(), `1800000000`, `1800000000.5`, 1), strings.Replace(officerSave(), `1800000000`, `-1`, 1), strings.Replace(officerSave(), `"Invented Guild"`, `false`, 1), `RelaxRaidDB={version=1,ranks=true}`, `RelaxRaidDB={version=1,ranks={[1]={result="officer",readAt=1}}}`}
	// A valid officer sibling is essential: empty ranks already lock for a
	// different reason and cannot prove the version admission guard.
	for _, version := range []string{`version=2`, `version="1"`, `version=true`, `other=1`} {
		bad = append(bad, strings.Replace(officerSave(), "version=1", version, 1))
	}
	var rows strings.Builder
	for i := 0; i < 257; i++ {
		fmt.Fprintf(&rows, `["Sample%d-PvP-EU"]={result="officer",readAt=1800000000},`, i)
	}
	bad = append(bad, `RelaxRaidDB={version=1,ranks={`+rows.String()+`}}`)
	for i, s := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			raidRankSave(t, game, "A#1", officerSave())
			if p := accountProjection(t, b); p["HasRaids"] != true {
				t.Fatal("valid control locked")
			}
			raidRankSave(t, game, "A#1", s)
			// No report refresh first: direct route must re-read the rank.
			if data, err := b.Call("raidPage", nil); err == nil || data != nil {
				t.Fatal("bad save retained unlock")
			}
			if p := accountProjection(t, b); p["HasRaids"] != false || p["RaidHTML"] != "" {
				t.Fatal("bad save rendered page")
			}
		})
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if data, err := b.Call("raidPage", nil); err == nil || data != nil {
		t.Fatal("missing save retained unlock")
	}
}
