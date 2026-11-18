package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// Invented names with the real shape: a hyphen inside the surname, and no surname.
var sampleNames = []string{"Alia-Stone-PvP-EU", "Bram-Oak-Hill-PvP-EU", "Cato-PvP-EU"}

func TestNameIsOpaque(t *testing.T) {
	for _, name := range sampleNames {
		owner := modeltest.Player(name)
		for _, a := range []model.Actor{owner, modeltest.Creature(name, 42), modeltest.Pet(name, owner)} {
			if a.Name != name {
				t.Fatalf("name changed: %q -> %q", name, a.Name)
			}
		}
		if modeltest.Player(name).ID != owner.ID {
			t.Fatalf("id for %q is not stable", name)
		}
	}
}

func TestNoNameSplitting(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "model.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Type.Results == nil {
			continue
		}
		returnsString := false
		for _, r := range fn.Type.Results.List {
			if id, ok := r.Type.(*ast.Ident); ok && id.Name == "string" {
				returnsString = true
			}
		}
		for _, bad := range []string{"First", "Last", "Realm", "Surname"} {
			if returnsString && strings.Contains(fn.Name.Name, bad) {
				t.Errorf("%s: model must not split names (%s)", fset.Position(fn.Pos()), fn.Name.Name)
			}
		}
	}
}

func TestIsDeath(t *testing.T) {
	a, b := modeltest.Player("Alia-Stone-PvP-EU"), modeltest.Creature("Ash Hound", 42)
	absent := modeltest.At(1).Src(a).Dst(b).Build()
	absent.Kind = model.UnitDied
	cases := []struct {
		name string
		e    model.Event
		want bool
	}{
		{"flag absent", absent, false},
		{"feign death (1)", modeltest.At(1).Dst(a).Died(1).Build(), false},
		{"death (0)", modeltest.At(1).Dst(a).Died(0).Build(), true},
	}
	for _, c := range cases {
		if got := c.e.IsDeath(); got != c.want {
			t.Errorf("%s: IsDeath()=%v, want %v", c.name, got, c.want)
		}
	}
	for _, k := range model.Kinds() {
		if k == model.UnitDied {
			continue
		}
		e := model.Event{Kind: k, DiedFlag: model.Some(0)}
		if e.IsDeath() {
			t.Errorf("kind %s with flag 0 counted as a death", k)
		}
	}
}

func TestOptZeroIsNotAbsent(t *testing.T) {
	if (model.Opt{V: 0, Present: true}) == (model.Opt{}) {
		t.Fatal("a present zero equals an absent value")
	}
}

func TestNightAndRunAreDistinct(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not on PATH: compile-fail check skipped")
	}
	dir, err := filepath.Abs(filepath.Join("testdata", "nightrun"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(gobin, "build", "-o", os.DevNull, ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("passing a DungeonRun to func(model.RaidNight) compiled")
	}
	if !strings.Contains(string(out), "cannot use") || !strings.Contains(string(out), "DungeonRun") {
		t.Fatalf("build failed for another reason:\n%s", out)
	}
}

func TestKindNames(t *testing.T) {
	seen := map[string]model.Kind{}
	kinds := model.Kinds()
	if len(kinds) != 25 {
		t.Fatalf("%d kinds, want 25", len(kinds))
	}
	for _, k := range kinds {
		n := k.String()
		if n == "" || n == "Kind(?)" {
			t.Errorf("kind %d has no name", k)
		}
		if prev, dup := seen[n]; dup {
			t.Errorf("kinds %d and %d share the name %q", prev, k, n)
		}
		seen[n] = k
	}
}
