package compare

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOfficerStrict(t *testing.T) {
	for _, text := range []string{
		`null`, `[]`, `{}`, `{"version":2,"top":["Alia"],"bottom":["Bram"]}`,
		`{"top":["Alia"],"bottom":["Bram"]}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram"],"extra":true}`,
		`{"Version":1,"top":["Alia"],"bottom":["Bram"]}`,
		`{"version":1,"version":1,"top":["Alia"],"bottom":["Bram"]}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram"],"officer":null}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram"],"officer":1}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram"]} {}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram"]} garbage`,
		`{"version":1,"top":["Alia"],"bottom":[]}`,
		`{"version":1,"top":[],"bottom":["Bram"]}`,
		`{"version":1,"top":["Alia"],"bottom":null}`,
		`{"version":1,"top":["Alia"]}`,
		`{"version":1,"bottom":["Bram"]}`,
		`{"version":1,"top":["Alia","Alia"],"bottom":["Bram"]}`,
		`{"version":1,"top":["Alia"],"bottom":["Bram","BRAM"]}`,
		`{"version":1,"top":["Alia"],"bottom":["ALIA"]}`,
		`{"version":1,"top":[""],"bottom":["Bram"]}`,
		`{"version":1,"top":[" "],"bottom":["Bram"]}`,
		`{"version":1,"top":[null],"bottom":["Bram"]}`,
		`{"version":1,"top":[1],"bottom":["Bram"]}`,
		`{"version":1,"top":["a","b","c","d","e","f"],"bottom":["g"]}`,
		`{"version":1,"top":["g"],"bottom":["a","b","c","d","e","f"]}`,
	} {
		t.Run(text, func(t *testing.T) {
			f, err := ParseOfficer([]byte(text))
			if err == nil || !reflect.DeepEqual(f, OfficerFile{}) {
				t.Fatalf("invalid file returned data: %+v error=%v", f, err)
			}
		})
	}
}

func TestOfficerUnicode(t *testing.T) {
	for _, name := range []string{`\ud800`, `\udbff`, `\udc00`, `\udfff`, `\udc00\ud800`, `\ud800x\udc00`, `\ud800\ud800`, string([]byte{255})} {
		data := []byte(`{"version":1,"top":["` + name + `"],"bottom":["Bram"]}`)
		got, err := ParseOfficer(data)
		if err == nil || !reflect.DeepEqual(got, OfficerFile{}) {
			t.Fatal("malformed Unicode was repaired instead of rejected")
		}
	}
	for _, name := range []string{`\ud800\udc00`, `\udbff\udfff`, `\\ud800`, `\ufffd`, `Élan-PvP-EU`} {
		data := []byte(`{"version":1,"top":["` + name + `"],"bottom":["Bram"]}`)
		if _, err := ParseOfficer(data); err != nil {
			t.Fatalf("valid Unicode/escaped backslash rejected: %v", err)
		}
	}
}

func TestOfficerValid(t *testing.T) {
	want := OfficerFile{Version: 1, Officer: "free text preserved", Top: []string{"Edda-PvP-EU", "Dara-PvP-EU", "Cato-PvP-EU", "Bram-PvP-EU", "Alia-PvP-EU"}, Bottom: []string{"Jori-PvP-EU", "Iona-PvP-EU", "Hale-PvP-EU", "Gala-PvP-EU", "Fenn-PvP-EU"}}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseOfficer(data)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("five-name limits, order or officer label changed")
	}
	for _, text := range []string{
		`{"version":1,"top":["Alia-Stone-PvP-EU"],"bottom":["Bram-Oak-Hill-PvP-EU"]}`,
		` {"bottom":["Bram-Oak-Hill-PvP-EU"],"officer":"notes only","top":["Alia-Stone-PvP-EU"],"version":1} `,
	} {
		f, err := ParseOfficer([]byte(text))
		if err != nil || f.Version != 1 || !reflect.DeepEqual(f.Top, []string{"Alia-Stone-PvP-EU"}) || !reflect.DeepEqual(f.Bottom, []string{"Bram-Oak-Hill-PvP-EU"}) {
			t.Fatalf("valid officer file: %+v %v", f, err)
		}
	}
}
