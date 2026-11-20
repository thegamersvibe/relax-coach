package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const raidTestServerS int64 = 1800000000

func raidTestClock() time.Time { return time.Unix(raidTestServerS, 0).UTC() }
func raidLockApp(t *testing.T, game string, clock func() time.Time) (*App, *nativeBridge) {
	t.Helper()
	a := &App{raidNow: clock}
	a.setGame(game)
	a.refresh()
	b := newNativeBridge(a, true, nativeServices{})
	t.Cleanup(b.Close)
	return a, b
}
func assertRaidAdmission(t *testing.T, a *App, b *nativeBridge, allowed, badTime bool) {
	t.Helper()
	p := accountProjection(t, b)
	if p["HasRaids"] != allowed || (p["RaidHTML"] != "") != allowed {
		t.Fatalf("admission projection: got %v want %v", p["HasRaids"], allowed)
	}
	html, err := renderNativeShell(a.snapshot(), true, "lock-time-test")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{`id="nav-raids"`, `id="raids-view"`} {
		if strings.Contains(html, id) != allowed {
			t.Fatalf("wrong rendered admission for %s", id)
		}
	}
	data, err := b.Call("raidPage", nil)
	if allowed {
		if err != nil || data == nil {
			t.Fatalf("valid officer route locked: %v", err)
		}
	} else {
		if err == nil || data != nil || !strings.Contains(err.Error(), "locked") {
			t.Fatal("locked route returned data")
		}
		if badTime && !strings.Contains(err.Error(), "reading is too old or its time is invalid") {
			t.Fatalf("missing time explanation: %v", err)
		}
	}
}
func TestRaidLockFreshness(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	a, b := raidLockApp(t, game, raidTestClock)
	for _, tc := range []struct {
		name    string
		offset  time.Duration
		allowed bool
	}{
		{"current", 0, true}, {"13-days", -13 * 24 * time.Hour, true},
		{"14-days", -14 * 24 * time.Hour, true}, {"14-days-plus-second", -14*24*time.Hour - time.Second, false},
		{"15-days", -15 * 24 * time.Hour, false}, {"future-one-day", 24 * time.Hour, true},
		{"future-over-one-day", 24*time.Hour + time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := raidTestClock().Add(tc.offset).Unix()
			save := strings.Replace(officerSave(), "1800000000", fmt.Sprint(at), 1)
			raidRankSave(t, game, "A#1", save)
			assertRaidAdmission(t, a, b, tc.allowed, !tc.allowed)
		})
	}
	for _, at := range []string{"", `readAt="unreadable",`, `readAt=0,`, `readAt=1800000000.5,`} {
		save := `RelaxRaidDB={version=1,ranks={["Alia-PvP-EU"]={result="officer",` + at + `}}}`
		raidRankSave(t, game, "A#1", save)
		assertRaidAdmission(t, a, b, false, true)
	}
}
func TestRaidLockGuildAbsent(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	a, b := raidLockApp(t, game, raidTestClock)
	for _, result := range []string{"officer", "member", "noguild", "unreadable"} {
		for _, stamp := range []string{"", "readAt=1800000000,"} {
			save := `RelaxRaidDB={version=1,ranks={["Alia-PvP-EU"]={result="` + result + `",` + stamp + `}}}`
			raidRankSave(t, game, "A#1", save)
			assertRaidAdmission(t, a, b, result == "officer" && stamp != "", result == "officer" && stamp == "")
		}
	}
}
func TestRaidLockMixedRows(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	a, b := raidLockApp(t, game, raidTestClock)
	// Invalid rows cannot themselves admit, but do not poison a separate valid
	// officer. Non-officer rows may intentionally lack a server timestamp.
	other := `["Bram-PvP-EU"]=false,["Cato-PvP-EU"]={result="member"},["Dara-PvP-EU"]={result="pending"},["Elen-PvP-EU"]={result="officer"}`
	valid := `["Alia-PvP-EU"]={result="officer",readAt=1800000000},`
	for _, control := range []bool{true, false} {
		rows := other
		if control {
			rows = valid + rows
		}
		raidRankSave(t, game, "A#1", `RelaxRaidDB={version=1,ranks={`+rows+`}}`)
		assertRaidAdmission(t, a, b, control, !control)
	}
}
func TestRaidLockFiftyRows(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	a, b := raidLockApp(t, game, raidTestClock)
	for _, count := range []int{50, 51} {
		var rows strings.Builder
		for i := 0; i < count; i++ {
			fmt.Fprintf(&rows, `["Sample%d-PvP-EU"]={result="officer",readAt=1800000000},`, i)
		}
		raidRankSave(t, game, "A#1", `RelaxRaidDB={version=1,ranks={`+rows.String()+`}}`)
		assertRaidAdmission(t, a, b, count == 50, false)
	}
}
func TestRaidLockGuildBounds(t *testing.T) {
	for _, count := range []int{96, 97} {
		body := strings.Replace(officerSave(), "Invented Guild", strings.Repeat("G", count), 1)
		if (raidAdmission([]byte(body), raidTestClock()) == nil) != (count == 96) {
			t.Fatalf("guild byte boundary %d", count)
		}
	}
	body := strings.Replace(officerSave(), "Invented Guild", `Invented\nGuild`, 1)
	if raidAdmission([]byte(body), raidTestClock()) == nil {
		t.Fatal("guild control byte admitted")
	}
}
func TestRaidLockClockExpiryAndUTC(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	raidRankSave(t, game, "A#1", officerSave())
	current := raidTestClock().In(time.FixedZone("Fixture East", 5*60*60))
	a, b := raidLockApp(t, game, func() time.Time { return current })
	assertRaidAdmission(t, a, b, true, false)
	current = current.Add(15 * 24 * time.Hour)
	// No preceding report refresh: a previously displayed page is not authority.
	if data, err := b.Call("raidPage", nil); data != nil || err == nil || !strings.Contains(err.Error(), "reading is too old or its time is invalid") {
		t.Fatal("direct route kept an expired unlock")
	}
	assertRaidAdmission(t, a, b, false, true)
}
