package prepread

import "testing"

// "u" is the only unreadable marker for level and played time; "-" is not
// a synonym and must be refused rather than read as unknown.
func TestLevelAndPlayedDashIsNotUnreadable(t *testing.T) {
	for _, tc := range []struct {
		at  int
		why string
	}{{5, "wire level"}, {6, "wire played"}} {
		if _, why := decodeBody(field(minimalBody, tc.at, "-")); why != tc.why {
			t.Errorf("field %d \"-\": got %q, want %q", tc.at, why, tc.why)
		}
		r, why := decodeBody(field(minimalBody, tc.at, "u"))
		if why != "" {
			t.Errorf("field %d \"u\" refused: %s", tc.at, why)
		}
		if tc.at == 5 && r.Level != 0 || tc.at == 6 && r.PlayedAtLevelS.Present {
			t.Errorf("field %d \"u\" not read as unknown: %+v", tc.at, r)
		}
	}
}
