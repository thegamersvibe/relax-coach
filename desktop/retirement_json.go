package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Keep extensions opaque (including large numbers) when updating either journal.
func retirementDecodeJSON(data []byte, known any, extra *map[string]json.RawMessage, keys ...string) error {
	if err := json.Unmarshal(data, known); err != nil {
		return err
	}
	if err := json.Unmarshal(data, extra); err != nil {
		return err
	}
	for _, key := range keys {
		delete(*extra, key)
	}
	return nil
}
func retirementEncodeJSON(known any, extra map[string]json.RawMessage) ([]byte, error) {
	data, err := json.Marshal(known)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for key, value := range extra {
		if _, known := fields[key]; !known {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}
func (e *retirementEntry) UnmarshalJSON(data []byte) error {
	type known retirementEntry
	return retirementDecodeJSON(data, (*known)(e), &e.Extra, "source", "destination", "type", "sha256")
}
func (e retirementEntry) MarshalJSON() ([]byte, error) {
	type known retirementEntry
	return retirementEncodeJSON(known(e), e.Extra)
}
func (m *retirementManifest) UnmarshalJSON(data []byte) error {
	type known retirementManifest
	return retirementDecodeJSON(data, (*known)(m), &m.Extra, "schema", "entries")
}
func (m retirementManifest) MarshalJSON() ([]byte, error) {
	type known retirementManifest
	return retirementEncodeJSON(known(m), m.Extra)
}

// Intent extensions must not disappear when the intent is finalized. Conflicting
// opaque values cannot be reconciled safely: retain both journals unchanged.
func retirementMergeJSON(a, b map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	merged := make(map[string]json.RawMessage)
	for key, value := range a {
		merged[key] = value
	}
	for key, value := range b {
		if old, exists := merged[key]; exists {
			var left, right bytes.Buffer
			if json.Compact(&left, old) != nil || json.Compact(&right, value) != nil || !bytes.Equal(left.Bytes(), right.Bytes()) {
				return nil, fmt.Errorf("conflicting retirement journal extension %q", key)
			}
		}
		merged[key] = value
	}
	return merged, nil
}
func retirementSameEntry(a, b retirementEntry) bool {
	return a.Source == b.Source && a.Destination == b.Destination && a.Type == b.Type && a.SHA256 == b.SHA256
}
