package main

import "encoding/json"

// Absent preferences default on. An unreadable preference fails closed instead
// of accidentally re-enabling a user's opt-out.
func savedWowheadNames() bool {
	configMu.Lock()
	defer configMu.Unlock()
	c, err := readCoachConfig()
	if err != nil {
		return false
	}
	raw, exists := c["wowheadNames"]
	if !exists {
		return true
	}
	var enabled bool
	return json.Unmarshal(raw, &enabled) == nil && enabled
}

func saveWowheadNames(enabled bool) error {
	return writeCoachConfig(func(c map[string]json.RawMessage) error {
		c["wowheadNames"], _ = json.Marshal(enabled)
		return nil
	})
}
