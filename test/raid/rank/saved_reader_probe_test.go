// Run beside the unmodified SavedVariables parser from each published app tag.
package main

import (
	"os"
	"testing"
)

func TestRankSaveAccepted(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("RANK_SAVE"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := ParseSavedVariables(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	db := M(saved["RelaxRaidDB"])
	if db["version"] != float64(1) || M(db["other"])["keep"] != "unchanged" {
		t.Fatal("account version or existing data changed")
	}
	ranks := M(db["ranks"])
	for name, result := range map[string]string{
		"Alia-Stone-PvP-EU": "officer", "Bram-Oak-PvP-EU": "member",
		"Cato-PvP-EU": "noguild", "Dara-Fern-PvP-EU": "unreadable",
	} {
		row := M(ranks[name])
		if row["result"] != result {
			t.Fatalf("%s: wrong rank reading", name)
		}
	}
	omitted := os.Getenv("RANK_OMITTED") == "1"
	var guild any = "Invented Guild"
	if omitted {
		guild = nil
	}
	if len(ranks) != 4 || M(ranks["Alia-Stone-PvP-EU"])["readAt"] != float64(1800000000) ||
		M(ranks["Alia-Stone-PvP-EU"])["guild"] != guild ||
		M(ranks["Cato-PvP-EU"])["guild"] != nil {
		t.Fatal("rank table, time or guild lost")
	}
	for i, name := range []string{"Bram-Oak-PvP-EU", "Cato-PvP-EU", "Dara-Fern-PvP-EU"} {
		var at any = float64(1800000001 + i)
		if omitted {
			at = nil
		}
		if M(ranks[name])["readAt"] != at {
			t.Fatal("negative reading timestamp changed")
		}
	}
	prep := M(db["prep"])
	night := M(M(prep["nights"])["1"])
	body := M(M(night["reports"])["alia-stone-pvp-eu"])["1"]
	if prep["version"] != float64(2) || body != "1\t0.2.0\tAlia-Stone-PvP-EU\tMAGE\t60\tu\tjoin\t1800000000\t-\tn\tn" {
		t.Fatal("existing preparation report lost")
	}
	t.Log("real addon save accepted: four rank readings and existing preparation retained")
}
