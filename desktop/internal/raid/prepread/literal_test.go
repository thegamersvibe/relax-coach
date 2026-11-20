package prepread

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLiteralRefusals(t *testing.T) {
	for _, raw := range []string{
		"", "RelaxRaidDB = function() end", "RelaxRaidDB = os.execute('bad')", "RelaxRaidDB = {} print('bad')",
		"RelaxRaidDB = {} RelaxRaidDB = {}", "RelaxRaidDB = {x=1,x=2}", "RelaxRaidDB = {[1]=1,2}",
		"RelaxRaidDB = {[nil]=1}", "RelaxRaidDB = {[{}]=1}", "RelaxRaidDB = {[1]=nil,[1]=3}",
		"RelaxRaidDB = {x=1e999}", "RelaxRaidDB = {x=0/0}", "RelaxRaidDB = {x=NaN}", "RelaxRaidDB = {x=1+2}",
		"RelaxRaidDB = {x='\\256'}", "RelaxRaidDB = {x='\\u1234'}", "RelaxRaidDB = {x='\\x41'}", "RelaxRaidDB = {x='broken\n'}",
		"RelaxRaidDB = {x=1e}", "RelaxRaidDB = {x=-}", "RelaxRaidDB = {x='unterminated}", "RelaxRaidDB = {",
		"RelaxRaidDB = {} --[[unfinished", "--[=[\nRelaxRaidDB = {}", "RelaxRaidDB = false",
	} {
		if _, err := parseLiteral([]byte(raw)); err == nil {
			t.Errorf("accepted nonliteral/corrupt input %q", raw)
		}
	}
}

func TestLiteralBudgets(t *testing.T) {
	cases := []struct{ name, raw, why string }{
		{"depth", "RelaxRaidDB=" + strings.Repeat("{", limits.MaxDepth+1) + strings.Repeat("}", limits.MaxDepth+1), "literal depth"},
		{"entries", "RelaxRaidDB={" + strings.Repeat("0,", limits.MaxEntries+1) + "}", "literal entries"},
		{"string", "RelaxRaidDB={x='" + strings.Repeat("a", limits.MaxStringBytes+1) + "'}", "literal string"},
		{"file", "RelaxRaidDB={}" + strings.Repeat(" ", limits.MaxFileBytes), "file too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseLiteral([]byte(tc.raw))
			if err == nil || err.Error() != tc.why {
				t.Fatalf("got %v want %s", err, tc.why)
			}
		})
	}
	for _, raw := range []string{
		"RelaxRaidDB=" + strings.Repeat("{", limits.MaxDepth) + strings.Repeat("}", limits.MaxDepth),
		"RelaxRaidDB={" + strings.Repeat("0,", limits.MaxEntries) + "}",
		"RelaxRaidDB={x='" + strings.Repeat("a", limits.MaxStringBytes) + "'}",
		"RelaxRaidDB={}" + strings.Repeat(" ", limits.MaxFileBytes-len("RelaxRaidDB={}")),
	} {
		if _, err := parseLiteral([]byte(raw)); err != nil {
			t.Fatalf("exact parser bound: %v", err)
		}
	}
}

func TestLiteralLuaEscapesAndTypedKeys(t *testing.T) {
	raw := []byte("--[[comment]]\nRelaxRaidDB = {[1] = 'number', ['1'] = 'string', word = \"\\009\\9\\010\\255\\\\\\\"\\'\\a\\b\\f\\n\\r\\t\\v\", a=-1,b=1.0,c=1e1,d=true,e=false,f=nil}; --done")
	root, err := parseLiteral(raw)
	if err != nil {
		t.Fatal(err)
	}
	if root[float64(1)] != "number" || root["1"] != "string" || root["a"] != float64(-1) || root["b"] != float64(1) || root["c"] != float64(10) || root["d"] != true || root["e"] != false || root["f"] != nil {
		t.Fatalf("values: %+v", root)
	}
	want := []byte{9, 9, 10, 255, '\\', '"', '\'', 7, 8, 12, 10, 13, 9, 11}
	if !bytes.Equal([]byte(root["word"].(string)), want) {
		t.Fatalf("escape bytes=%v", []byte(root["word"].(string)))
	}
	root, err = parseLiteral([]byte("--[=[multi\nline]=]\nRelaxRaidDB={}"))
	if err != nil || len(root) != 0 {
		t.Fatalf("long comment: %v", err)
	}
}

func TestLiteralEveryTruncationAndNoPanic(t *testing.T) {
	raw := fixture(t, "normal")
	for i := 0; i < len(raw); i++ {
		Parse(raw[:i], epoch, epoch+20)
	}
	for i := 0; i < 256; i++ {
		Parse([]byte(fmt.Sprintf("RelaxRaidDB={[%c]=%c}", i, i)), epoch, epoch+20)
	}
}

func TestReadFileSizeAndNewerPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "RelaxRaid.lua")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(int64(limits.MaxFileBytes) + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, d, err := ReadFile(path, epoch, epoch)
	if err != nil || len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"file too large"}) {
		t.Fatalf("oversize: %+v %+v %v", got, d, err)
	}
	raw := fixture(t, "newer")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, d, err = ReadFile(path, epoch, epoch)
	after, _ := os.ReadFile(path)
	if err != nil || len(got) != 0 || !reflect.DeepEqual(d.Warnings, []string{"newer store"}) || !bytes.Equal(raw, after) {
		t.Fatalf("newer: %+v %+v %v", got, d, err)
	}
	link := filepath.Join(dir, "link.lua")
	if err = os.Symlink(path, link); err == nil {
		if _, _, err = ReadFile(link, epoch, epoch); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	for _, name := range []string{"normal", "markers", "dedup"} {
		raw, err := os.ReadFile("testdata/" + name + ".lua")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Add([]byte("RelaxRaidDB={version=1,prep={version=2,nights={}}}"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		a, da := Parse(raw, epoch, epoch+100)
		b, db := Parse(raw, epoch, epoch+100)
		if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(da, db) {
			t.Fatal("nondeterministic parse")
		}
	})
}
