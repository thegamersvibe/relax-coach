package prepread

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestPrimitiveAndProtocolBounds(t *testing.T) {
	if limits.MaxFileBytes != 16<<20 || limits.MaxEntries != 150000 || limits.MaxDepth != 16 || limits.MaxStringBytes != 4096 || limits.MaxBodyBytes != 4096 || limits.MaxCharacterBytes != 100000 || limits.MaxNightBytes != 4000000 || limits.MaxItemID != 999999999 || limits.MaxCount != 999999 || limits.MaxPlayed != 999999999 || limits.MaxStamp != 9999999999 || limits.MaxDay != 999999 || limits.LookbackSeconds != 7200 {
		t.Fatal("protocol/parser budgets drifted")
	}
	for _, v := range []any{table{float64(0): true}, table{float64(2): true}, table{"1": true}, table{float64(1): nil}, table{float64(1): true, "x": true}, table{float64(1): true, float64(2): true}, true} {
		if _, ok := array(v, 1); ok {
			t.Fatalf("invalid array accepted %#v", v)
		}
	}
	if v, ok := array(table{float64(1): true}, 1); !ok || len(v) != 1 {
		t.Fatal("exact array cap rejected")
	}
	for _, tc := range []struct {
		value any
		want  bool
	}{{float64(-1), false}, {float64(0), true}, {float64(19), true}, {float64(20), false}, {0.5, false}, {"1", false}, {true, false}, {nil, false}} {
		if _, ok := integer(tc.value, 0, 19); ok != tc.want {
			t.Fatalf("integer %v: %v", tc.value, ok)
		}
	}
	for _, digits := range []int{64, 65} {
		_, err := parseLiteral([]byte("RelaxRaidDB={value=" + strings.Repeat("0", digits) + "}"))
		if (err == nil) != (digits == 64) {
			t.Fatalf("number token length %d: %v", digits, err)
		}
	}
	for _, raw := range []string{"OtherDB={}", "RelaxRaidDB={x='\\q'}", "RelaxRaidDB={x='\\'}"} {
		if _, err := parseLiteral([]byte(raw)); err == nil {
			t.Fatalf("not a supported literal: %s", raw)
		}
	}
	for _, equals := range []int{64, 65} {
		delimiter := strings.Repeat("=", equals)
		_, err := parseLiteral([]byte("--[" + delimiter + "[comment]" + delimiter + "]\nRelaxRaidDB={}"))
		if (err == nil) != (equals == 64) {
			t.Fatalf("comment delimiter length %d: %v", equals, err)
		}
	}
}

func TestThreeNightsFullRotatingRoster(t *testing.T) {
	ns := table{}
	for day := 1; day <= 3; day++ {
		reports, windows := table{}, table{}
		for w := 1; w <= 80; w++ {
			rows := table{}
			for c := 1; c <= 40; c++ {
				name := fmt.Sprintf("Alia%04d-EU", (w-1)*40+c)
				key := nameKey(name)
				reports[key] = table{float64(1): field(minimalBody, 3, name)}
				rows[key] = table{"receivedAt": float64(epoch + int64(day)), "report": float64(1)}
			}
			windows[float64(w)] = table{"openedAt": float64(epoch), "reports": rows}
		}
		ns[float64(day)] = table{"day": float64(day), "reports": reports, "windows": windows}
	}
	root := table{"version": float64(1), "prep": table{"version": float64(2), "nights": ns}}
	got, d := Parse(rawRoot(root), epoch, epoch+3)
	if len(got) != 9600 || len(d.Warnings) != 0 || len(d.Skipped) != 0 {
		t.Fatalf("full count=%d diag=%+v", len(got), d)
	}
	want := emptyReport("Alia0001-EU", epoch)
	want.Stamped = epoch + 1
	if !reflect.DeepEqual(got[0], want) {
		t.Fatal("first report differs")
	}
}
