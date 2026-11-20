package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnverifiedCreditRefused(t *testing.T) {
	dir := copyData(t)
	change(t, dir, "support.json", func(d map[string]any) {
		e := d["effects"].([]any)[0].(map[string]any)
		e["credit"] = true
		e["verified"] = false
		e["stacks"] = false // Isolate verification from the independent stacking gate.
	})
	refusedField(t, dir, "support.json", "credit")
}

func TestSupportVerificationSeed(t *testing.T) {
	c := load(t, seedDir)
	for _, e := range c.Support.Effects {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		verified, present := raw["verified"].(bool)
		if !present {
			t.Errorf("%s lacks boolean verified", e.ID)
		}
		uncertain := strings.Contains(strings.ToLower(e.Note), "unverified") || e.Scope == "unknown"
		if uncertain && (e.Credit || verified) {
			t.Errorf("%s promotes unverified evidence: credit=%v verified=%v", e.ID, e.Credit, verified)
		}
		if e.Credit && !verified {
			t.Errorf("%s credits unverified support", e.ID)
		}
	}
}

func TestStudySpellNames(t *testing.T) {
	c := load(t, seedDir)
	p, ok := c.Profile("SHAMAN.enhancement_tank")
	if !ok {
		t.Fatal("missing profile")
	}
	found := false
	for _, s := range p.Spells {
		if s.Name == "Rockbiter" {
			found = true
		}
		if s.Name == "Rockbiter Weapon" {
			t.Error("study paladin-shaman.md:153,158,163 says Rockbiter")
		}
	}
	if !found {
		t.Error("missing study-exact Rockbiter")
	}
	removed := map[string]bool{"greater_blessing_of_wisdom": true, "greater_blessing_of_kings": true, "fire_resistance_aura": true, "frost_resistance_aura": true}
	for _, e := range c.Support.Effects {
		if removed[e.ID] {
			t.Errorf("unsourced full spell name retained: %s", e.Spell.Name)
		}
	}
	for _, p := range c.Specs.Profiles {
		for _, id := range p.Support {
			if removed[id] {
				t.Errorf("%s retains unsourced support %s", p.ID, id)
			}
		}
	}
}

func TestEnabledRegistryDocumentation(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(seedDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if strings.Contains(text, "enabled registry starts empty") || !strings.Contains(text, "encounter 478 (Balnazzar)") {
		t.Fatal("README does not describe seeded enabled registry")
	}
}
