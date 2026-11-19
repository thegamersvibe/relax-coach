package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRaidLogDiscoveryNewestAndRotated(t *testing.T) {
	root := t.TempDir()
	logs := filepath.Join(root, "Logs")
	if err := os.Mkdir(logs, 0700); err != nil {
		t.Fatal(err)
	}
	names := []string{"WoWCombatLog-old.txt", "WoWCombatLog.txt", "WoWCombatLog-new.txt", "Other.txt", "WoWCombatLog.csv"}
	for i, name := range names {
		p := filepath.Join(logs, name)
		if err := os.WriteFile(p, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(logs, "WoWCombatLog-folder.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	choices, err := raidLogChoices(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) != 3 || filepath.Base(choices[0].Path) != "WoWCombatLog-new.txt" || filepath.Base(choices[1].Path) != "WoWCombatLog.txt" {
		t.Fatalf("choices=%v", choices)
	}
	f, err := openRaidLog(root, "")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "WoWCombatLog-new.txt" {
		t.Fatal("not newest")
	}
}
func TestRaidLogManualPathAndFailures(t *testing.T) {
	root := t.TempDir()
	manual := filepath.Join(root, "invented archive.log")
	if err := os.WriteFile(manual, []byte("invented"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := openRaidLog(filepath.Join(root, "absent installation"), manual)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(b) != "invented" {
		t.Fatal("manual path ignored")
	}
	for _, p := range []string{root, filepath.Join(root, "missing")} {
		if f, err := openRaidLog(root, p); err == nil {
			f.Close()
			t.Fatal("accepted nonfile")
		}
	}
	if _, err := raidLogChoices(root); err == nil {
		t.Fatal("missing Logs accepted")
	}
	if err := os.Mkdir(filepath.Join(root, "Logs"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := openRaidLog(root, ""); err == nil {
		t.Fatal("empty folder accepted")
	}
}
func TestRaidLogEqualTimesAreDeterministic(t *testing.T) {
	root := t.TempDir()
	logs := filepath.Join(root, "Logs")
	if err := os.Mkdir(logs, 0700); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(100, 0)
	for _, name := range []string{"WoWCombatLog-z.txt", "WoWCombatLog-a.txt"} {
		p := filepath.Join(logs, name)
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := raidLogChoices(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || filepath.Base(got[0].Path) != "WoWCombatLog-a.txt" {
		t.Fatal("unstable tie")
	}
}
