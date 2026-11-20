package death_test

import "testing"

func TestAnonymousDiagnosticInputs(t *testing.T) {
	f := raidwide(5, 3)
	d := forVictim(t, f, config())
	got := diagnostics(f, d, config().Scoring.Death)
	if got.HealthSamples != 2 || got.RaidwideShare == nil || *got.RaidwideShare != .6 || len(got.RaidwideCandidates) != 1 || got.RaidwideCandidates[0].PlayersAlive != 5 || got.RaidwideCandidates[0].PlayersHit != 3 {
		t.Fatalf("diagnostics=%+v", got)
	}
	f.Events = f.Events[3:4]
	d = forVictim(t, f, config())
	got = diagnostics(f, d, config().Scoring.Death)
	if got.HealthSamples != 0 || got.RaidwideShare != nil || len(got.RaidwideCandidates) != 0 {
		t.Fatalf("absent candidate fabricated: %+v", got)
	}
}
