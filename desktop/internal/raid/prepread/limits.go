package prepread

import (
	_ "embed"
	"encoding/json"
)

// Protocol limits come from PREP-PROTOCOL.md and Prep/Store/Wire.lua. Parser
// budgets are separate defensive bounds, not score tuning. As in logfmt,
// larger literals live in embedded data, not raid-tree Go code.
//
//go:embed limits.json
var limitsJSON []byte

var limits = func() struct {
	MaxFileBytes, MaxEntries, MaxDepth, MaxStringBytes, MaxBodyBytes  int
	MaxCharacterBytes, MaxNightBytes                                  int
	MaxItemID, MaxCount, MaxPlayed, MaxStamp, MaxDay, LookbackSeconds int64
} {
	var v struct {
		MaxFileBytes, MaxEntries, MaxDepth, MaxStringBytes, MaxBodyBytes  int
		MaxCharacterBytes, MaxNightBytes                                  int
		MaxItemID, MaxCount, MaxPlayed, MaxStamp, MaxDay, LookbackSeconds int64
	}
	if err := json.Unmarshal(limitsJSON, &v); err != nil || v.MaxFileBytes <= 0 || v.MaxEntries <= 0 || v.MaxDepth <= 0 || v.MaxStringBytes <= 0 || v.MaxBodyBytes <= 0 || v.MaxCharacterBytes <= 0 || v.MaxNightBytes <= 0 || v.MaxItemID <= 0 || v.MaxCount <= 0 || v.MaxPlayed <= 0 || v.MaxStamp <= 0 || v.MaxDay <= 0 || v.LookbackSeconds <= 0 {
		panic("invalid embedded prep reader limits")
	}
	return v
}()

const (
	maxNights  = 3
	maxWindows = 80
	maxPlayers = 40
)
