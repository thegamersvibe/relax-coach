package main

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sample = `
ForeverCoachDB = {
	["schema"] = 1,
	["chars"] = {
		["Testwarrior-Test"] = {
			["snapshot"] = { ["name"] = "Testwarrior", ["level"] = 20, ["class"] = "WARRIOR", ["race"] = "Dwarf", ["xp"] = 500, ["xpMax"] = 10000, ["rested"] = 0 },
			["gear"] = { ["spec"] = "protection", ["specSource"] = "talents" },
			["findings"] = {
				{ ["impact"] = 80, ["area"] = "Build", ["text"] = "2 unspent talent points", ["fix"] = "Open talents" }, -- [1]
				{ ["impact"] = 50.5, ["area"] = "Play", ["text"] = "Quote \"x\" and \\ slash", ["fix"] = "a\nb", ["where"] = "here", ["estimate"] = true }, -- [2]
			},
			["sessions"] = { { ["deaths"] = { { ["level"] = 19, ["zone"] = "Redridge" } } } },
			["updated"] = 1700000000,
			["flag"] = true, ["nothing"] = nil, ["hex"] = 0x10, ["neg"] = -1.5e2,
		},
	},
}
Other = "x"
`

func TestParse(t *testing.T) {
	v, err := ParseSavedVariables(sample)
	if err != nil {
		t.Fatal(err)
	}
	cs := buildChars(M(v["ForeverCoachDB"]))
	if len(cs) != 1 {
		t.Fatalf("chars %d", len(cs))
	}
	c := cs[0]
	if c.Level != 20 || c.Spec != "protection" || len(c.Findings) != 2 || c.Findings[1].Text != `Quote "x" and \ slash` || c.Findings[1].Fix != "a\nb" ||
		c.Findings[1].Where != "here" || !c.Findings[1].Estimate {
		t.Fatalf("bad %+v", c)
	}
	if len(c.Deaths) != 1 || !strings.Contains(c.Deaths[0], "Redridge") {
		t.Fatalf("deaths %v", c.Deaths)
	}
	ch := M(M(M(v["ForeverCoachDB"])["chars"])["Testwarrior-Test"])
	if N(ch["hex"]) != 16 || N(ch["neg"]) != -150 || ch["flag"] != true || S(v["Other"]) != "x" {
		t.Fatalf("scalars %v", ch)
	}
}

// Every truncation of a valid save must return an error, never panic (the game writes the file while we watch it).
func TestTruncatedNeverPanics(t *testing.T) {
	for i := 0; i < len(sample); i++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic at cut %d: %v", i, r)
				}
			}()
			_, _ = ParseSavedVariables(sample[:i])
		}()
	}
	for _, bad := range []string{"ForeverCoachDB = {", "ForeverCoachDB = { [1] = 2,", "X = {{{{", "X = \"abc", "X = { [", strings.Repeat("{", 200)} {
		if _, err := ParseSavedVariables(bad); err == nil && bad != "" {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

// A half-written save keeps the previous report on screen instead of blanking it.
func TestReloadKeepsLastGood(t *testing.T) {
	dir := t.TempDir()
	sv := filepath.Join(dir, "ForeverCoach.lua")
	os.WriteFile(sv, []byte(sample), 0o644)
	a := &App{game: dir, svPath: sv, addonDir: dir}
	a.reload()
	if len(a.snapshot().Chars) != 1 || a.snapshot().Error != "" {
		t.Fatalf("first load: %+v", a.snapshot().Error)
	}
	os.WriteFile(sv, []byte(sample[:len(sample)/2]), 0o644)
	time.Sleep(10 * time.Millisecond)
	a.reload()
	v := a.snapshot()
	if len(v.Chars) != 1 || v.Warning == "" {
		t.Fatalf("partial save should keep old data with a warning: chars=%d warn=%q err=%q", len(v.Chars), v.Warning, v.Error)
	}
	os.WriteFile(sv, []byte(strings.Replace(sample, `["schema"] = 1`, `["schema"] = 9`, 1)), 0o644)
	a.view = View{}
	a.reload()
	if !strings.Contains(a.snapshot().Error, "newer") {
		t.Fatalf("newer schema not rejected: %q", a.snapshot().Error)
	}
}

func TestRealFile(t *testing.T) {
	p := os.Getenv("SV_FILE")
	if p == "" {
		t.Skip()
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSavedVariables(string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestRulesToLuaRoundTrip(t *testing.T) {
	b, err := os.ReadFile("../rules/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if err := jsonUnmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	r["probe"] = "A -> B & café → \"q\" \\ \n end"
	lua := RulesToLua(r)
	if strings.Contains(lua, `\u00`) || strings.Contains(lua, `\u2`) {
		t.Fatal("JSON unicode escapes leaked into Lua output")
	}
	v, err := ParseSavedVariables(strings.Replace(lua, "-- Generated", "--", 1))
	if err != nil {
		t.Fatal(err)
	}
	got := M(v["ForeverCoach_Rules"])
	if S(got["version"]) != S(r["version"]) || int(N(got["schema"])) != RulesSchema || S(got["probe"]) != S(r["probe"]) {
		t.Fatalf("round trip: %q", S(got["probe"]))
	}
	if !semverLess("1.0.0", "v1.0.1") || semverLess("1.10.0", "1.9.9") || semverLess("1.0.0", "1.0.0") {
		t.Fatal("semver")
	}
}

func TestRender(t *testing.T) {
	p := os.Getenv("SV_FILE")
	src := sample
	if p != "" {
		b, _ := os.ReadFile(p)
		src = string(b)
	}
	v, err := ParseSavedVariables(src)
	if err != nil {
		t.Fatal(err)
	}
	tpl := template.Must(template.New("report.html").Funcs(funcs).ParseFS(assets, "report.html"))
	var sb strings.Builder
	if err := tpl.Execute(&sb, View{Chars: buildChars(M(v["ForeverCoachDB"])), AppVersion: Version, RulesVersion: "1.1.0"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sb.String(), "ZgotmplZ") {
		t.Fatal("unsafe css placeholder in output")
	}
	if out := os.Getenv("HTML_OUT"); out != "" {
		os.WriteFile(out, []byte(sb.String()), 0o644)
	}
}

func TestNormalizeGame(t *testing.T) {
	root := t.TempDir()
	wow := filepath.Join(root, "World of Warcraft")
	fv := filepath.Join(wow, "_forever_")
	os.MkdirAll(filepath.Join(fv, "Interface", "AddOns"), 0o755)
	os.MkdirAll(filepath.Join(wow, "_retail_", "WTF"), 0o755)
	os.WriteFile(filepath.Join(fv, "WowForever.exe"), nil, 0o644)
	for _, in := range []string{fv, `"` + fv + `"`, wow, filepath.Join(fv, "Interface", "AddOns"), filepath.Join(fv, "WowForever.exe"), " " + fv + string(filepath.Separator) + " "} {
		if got := NormalizeGame(in); got != fv {
			t.Fatalf("%q -> %q, want %q", in, got, fv)
		}
	}
	if NormalizeGame(filepath.Join(root, "nothing")) != "" || NormalizeGame("") != "" {
		t.Fatal("bad paths must be rejected")
	}
}

func TestSetGameEndpointState(t *testing.T) {
	a := &App{}
	a.setGame("")
	a.reload()
	if !a.snapshot().NoGame || !strings.Contains(a.snapshot().Error, "Couldn't find") {
		t.Fatalf("no game: %+v", a.snapshot().Error)
	}
}
