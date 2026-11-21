package compare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestOfficerBOM(t *testing.T) {
	const doc = `{"version":1,"top":["Alia-PvP-EU"],"bottom":["Bram-PvP-EU"]}`
	data := []byte("\xef\xbb\xbf" + doc)
	before := bytes.Clone(data)
	want := officer("Alia-PvP-EU", "Bram-PvP-EU")
	got, err := ParseOfficer(data)
	if err != nil || !reflect.DeepEqual(got, want) || !bytes.Equal(data, before) {
		t.Errorf("leading BOM should be stripped without editing caller bytes: %+v %v", got, err)
	}
	for _, text := range []string{"\xef\xbb\xbf\xef\xbb\xbf" + doc, " \xef\xbb\xbf" + doc, doc + "\xef\xbb\xbf", "\xef\xbb\xbf\xff" + doc} {
		got, err := ParseOfficer([]byte(text))
		if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
			t.Errorf("BOM support must not loosen JSON/Unicode checks: %+v %v", got, err)
		}
	}
}

func TestOfficerTrimNames(t *testing.T) {
	ns, roster, fights := fixture(4)
	f := OfficerFile{Version: 1, Officer: "  free text preserved  ", Top: []string{"  " + roster[3].Name + "\u2003", "\u00a0" + roster[2].Name + " "}, Bottom: []string{" " + roster[0].Name + "  "}}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(data)
	want := OfficerFile{Version: 1, Officer: f.Officer, Top: []string{roster[3].Name, roster[2].Name}, Bottom: []string{roster[0].Name}}
	got, err := ParseOfficer(data)
	if err != nil || !reflect.DeepEqual(got, want) || !bytes.Equal(data, before) {
		t.Fatalf("trim both lists, preserving order, spelling and officer label: %+v %v", got, err)
	}
	report, err := Compare(ns, roster, config(t), got, fights)
	if err != nil || len(report.Top) != 2 || len(report.Bottom) != 1 || report.Top[0].PlayerID != ns[3].PlayerID || report.Top[1].PlayerID != ns[2].PlayerID || report.Bottom[0].PlayerID != ns[0].PlayerID {
		t.Fatalf("trimmed parsed names did not resolve: %+v %v", report, err)
	}
}

func TestOfficerNameByteLimit(t *testing.T) {
	for _, tc := range []struct {
		label, name string
		valid       bool
	}{
		{label: "63-bytes", name: strings.Repeat("a", 63), valid: true},
		{label: "64-bytes", name: strings.Repeat("a", 64), valid: true},
		{label: "65-bytes", name: strings.Repeat("a", 65)},
		{label: "63-multibyte", name: strings.Repeat("é", 31) + "a", valid: true},
		{label: "64-multibyte", name: strings.Repeat("é", 32), valid: true},
		{label: "65-multibyte", name: strings.Repeat("é", 32) + "a"},
		{label: "64-after-trim", name: "  " + strings.Repeat("é", 32) + "\u2003", valid: true},
	} {
		for _, top := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/top=%v", tc.label, top), func(t *testing.T) {
				f := officer("Alia", "Bram")
				if top {
					f.Top[0] = tc.name
				} else {
					f.Bottom[0] = tc.name
				}
				data, err := json.Marshal(f)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ParseOfficer(data)
				if tc.valid {
					f.Top[0], f.Bottom[0] = strings.TrimSpace(f.Top[0]), strings.TrimSpace(f.Bottom[0])
					if err != nil || !reflect.DeepEqual(got, f) {
						t.Fatalf("valid boundary rejected or changed: %+v %v", got, err)
					}
				} else {
					if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
						t.Errorf("overlong name accepted: error=%v", err)
					}
					r, err := Compare(nil, nil, config(t), f, nil)
					if err == nil || !reflect.DeepEqual(r, Report{}) {
						t.Errorf("typed file bypassed the name byte limit: %+v %v", r, err)
					}
				}
			})
		}
	}
}

func TestOfficerNameControls(t *testing.T) {
	// Reject C0 (including NUL), DEL and C1 controls, not UTF-8 continuation bytes.
	for ch := rune(0); ch <= 0x9f; ch++ {
		if ch > 0x1f && ch < 0x7f {
			continue
		}
		for _, top := range []bool{true, false} {
			t.Run(fmt.Sprintf("U+%04X/top=%v", ch, top), func(t *testing.T) {
				f := officer("Alia", "Bram")
				if top {
					f.Top[0] = "A" + string(ch) + "lia"
				} else {
					f.Bottom[0] = "B" + string(ch) + "ram"
				}
				data, err := json.Marshal(f)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ParseOfficer(data)
				if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
					t.Errorf("control character accepted: U+%04X error=%v", ch, err)
				}
				if err := f.validate(); err == nil {
					t.Error("typed officer file accepted a control character")
				}
			})
		}
	}
	for _, name := range []string{"\tAlia", "Alia\n", "\rAlia\r"} {
		data, err := json.Marshal(officer(name, "Bram"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseOfficer(data)
		if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
			t.Error("trimming hid a forbidden leading/trailing control")
		}
	}
}

func TestOfficerTrimmedDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		top    []string
		bottom []string
	}{
		{name: "empty-after-trim", top: []string{" \u2003\u00a0 "}, bottom: []string{"Bram"}},
		{name: "top-repeat-after-trim", top: []string{" Alia ", "ALIA"}, bottom: []string{"Bram"}},
		{name: "bottom-repeat-after-trim", top: []string{"Alia"}, bottom: []string{"Bram", "\u2003BRAM "}},
		{name: "overlap-after-trim", top: []string{" Alia"}, bottom: []string{"ALIA\u00a0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(OfficerFile{Version: 1, Top: tc.top, Bottom: tc.bottom})
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseOfficer(data)
			if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
				t.Fatalf("invalid trimmed names returned data: %+v %v", got, err)
			}
		})
	}
}
