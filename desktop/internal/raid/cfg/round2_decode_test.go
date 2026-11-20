package cfg_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func replaceJSONText(t *testing.T, dir, file, old, replacement string) {
	t.Helper()
	path := filepath.Join(dir, file)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(old)) {
		t.Fatalf("missing JSON fixture anchor %q", old)
	}
	if err := os.WriteFile(path, bytes.Replace(b, []byte(old), []byte(replacement), 1), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRound2SurrogateStrings(t *testing.T) {
	for _, escaped := range []string{`\ud800`, `\udc00`, `\udc00\ud800`, `\udc00\udc00`, `\ud800\ud800`, `\ud800\u0041`, `\ud800x\udc00`, `\uDBFF`, `\uDFFF`} {
		for _, location := range []string{"value", "key"} {
			t.Run(location+"/"+escaped, func(t *testing.T) {
				dir := copyValidationData(t)
				field := "$.source"
				if location == "value" {
					replaceJSONText(t, dir, "scoring.json", `"source": "`, `"source": "`+escaped)
				} else {
					field = "$.count_penalties."
					replaceJSONText(t, dir, "scoring.json", `"count_penalties": {`, `"count_penalties": {"review_`+escaped+`": 1,`)
				}
				err := refused(t, dir, "scoring.json")
				if !strings.Contains(err.Error(), field) || !strings.Contains(err.Error(), "surrogate") {
					t.Fatalf("missing field/surrogate diagnosis: %v", err)
				}
			})
		}
	}
}

func TestRound2TrailingWhitespace(t *testing.T) {
	for _, suffix := range []string{"", " ", "\t\r\n", " \r\n\t  "} {
		t.Run(suffix, func(t *testing.T) {
			dir := copyValidationData(t)
			path := filepath.Join(dir, "scoring.json")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(b, []byte(suffix)...), 0600); err != nil {
				t.Fatal(err)
			}
			if load(t, dir).Scoring.Schema != 1 {
				t.Fatal("legal trailing whitespace changed the value")
			}
		})
	}
}

func TestRound2NULKeyPath(t *testing.T) {
	for _, tc := range []struct{ file, old, replacement, path string }{
		{"scoring.json", `"source":`, "\"sou\x00rce\":", `$["sou\x00rce"]`},
		{"scoring.json", `"tail_ms":`, "\"\x00tail_ms\":", `$.fights["\x00tail_ms"]`},
		{"support.json", `"note":`, "\"note\x00\":", `$.effects[0]["note\x00"]`},
		{"scoring.json", `"count_penalties": {`, "\"count_penalties\": {\"a\\\"b\x00c\": 1,", `$.count_penalties["a\\\"b\x00c"]`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := copyValidationData(t)
			replaceJSONText(t, dir, tc.file, tc.old, tc.replacement)
			refusedField(t, dir, tc.file, tc.path)
		})
	}
}

func TestRound2ValidStringEscapes(t *testing.T) {
	for _, tc := range []struct{ encoded, decoded string }{
		{`\ud800\udc00`, "\U00010000"},
		{`\uDBFF\uDFFF`, "\U0010ffff"},
		{`\uD83d\udE00`, "😀"},
		{`\ufffd`, "�"},
		{`\\ud800`, `\ud800`},
		{`\\\"\ud800\udc00`, "\\\"\U00010000"},
		{`\u0000`, "\x00"}, // Escaped NUL is valid JSON, unlike a raw control byte.
	} {
		t.Run(tc.encoded, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) {
				d["source"] = "marker"
				d["count_penalties"] = map[string]any{}
			})
			replaceJSONText(t, dir, "scoring.json", `"source":"marker"`, `"source":"prefix_`+tc.encoded+`"`)
			replaceJSONText(t, dir, "scoring.json", `"count_penalties":{}`, `"count_penalties":{"key_`+tc.encoded+`":1}`)
			c := load(t, dir)
			if c.Scoring.Source != "prefix_"+tc.decoded || c.Scoring.CountPenalties["key_"+tc.decoded] != 1 {
				t.Fatalf("string was repaired or lost: source=%q keys=%v", c.Scoring.Source, c.Scoring.CountPenalties)
			}
		})
	}
}
