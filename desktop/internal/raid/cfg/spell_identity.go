package cfg

import "fmt"

// Repeating a shared spell across catalogues is valid; conflicting names for
// one numeric id are not. Names without ids remain research-only labels.
func (c *Config) validateSpellIdentities() error {
	type identity struct{ name, path string }
	ids := map[int64]identity{}
	check := func(s Spell, path string) error {
		if s.ID == 0 {
			return nil
		}
		if prior, ok := ids[s.ID]; ok && prior.name != normalized(s.Name) {
			return fmt.Errorf("%s.id: spell id %d conflicts with %s.name", path, s.ID, prior.path)
		}
		ids[s.ID] = identity{normalized(s.Name), path}
		return nil
	}
	for i, p := range c.Specs.Profiles {
		for j, s := range p.Spells {
			if err := check(s, fmt.Sprintf("specs.json: profiles[%d].spells[%d]", i, j)); err != nil {
				return err
			}
		}
		for j, s := range p.CastSignatures {
			if err := check(s, fmt.Sprintf("specs.json: profiles[%d].cast_signatures[%d]", i, j)); err != nil {
				return err
			}
		}
	}
	for i, e := range c.Support.Effects {
		if err := check(e.Spell, fmt.Sprintf("support.json: effects[%d].spell", i)); err != nil {
			return err
		}
	}
	for i, e := range c.Candidates.Encounters {
		for j, s := range e.Abilities {
			if err := check(s, fmt.Sprintf("encounters-candidate.json: encounters[%d].abilities[%d]", i, j)); err != nil {
				return err
			}
		}
	}
	for i, e := range c.Encounters.Encounters {
		for j, m := range e.Mechanics {
			if err := check(m.Spell, fmt.Sprintf("encounters-enabled.json: encounters[%d].mechanics[%d].spell", i, j)); err != nil {
				return err
			}
		}
	}
	return nil
}
