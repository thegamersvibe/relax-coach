package raid_test

// Structure rules for desktop/internal/raid (PLAN.md 3.1, DEC-36):
//   - TestImports: a package imports only the raid packages its imports.json entry lists, never package main or
//     anything else under desktop/, and every package has an entry.
//   - TestNoLargeLiterals: no integer literal above 999 in non-test code, so spell, item and encounter ids live
//     in data files.

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/thegamersvibe/relax-coach/desktop"
const tree = module + "/internal/raid"

func TestImports(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not on PATH: import rule not checked")
	}
	raw, err := os.ReadFile("imports.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &allowed); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(gobin, "list", "-deps", "-json", "./...")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	checked := 0
	for {
		var p struct {
			ImportPath string
			GoFiles    []string
			Imports    []string
		}
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if p.ImportPath != tree && !strings.HasPrefix(p.ImportPath, tree+"/") {
			continue
		}
		if len(p.GoFiles) == 0 {
			continue // test-only, like this rules package: nothing is imported by product code
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, tree), "/")
		entry, ok := allowed[rel]
		if !ok {
			t.Errorf("%s has no entry in imports.json", rel)
			continue
		}
		var may []string
		if err := json.Unmarshal(entry, &may); err != nil {
			t.Fatalf("imports.json %s: %v", rel, err)
		}
		checked++
		for _, imp := range p.Imports {
			switch {
			case imp == module:
				t.Errorf("%s imports package main (%s)", rel, imp)
			case strings.HasPrefix(imp, tree+"/"):
				dep := strings.TrimPrefix(imp, tree+"/")
				if !contains(may, dep) {
					t.Errorf("%s imports %s, which imports.json does not allow", rel, dep)
				}
			case strings.HasPrefix(imp, module+"/"):
				t.Errorf("%s imports %s, outside internal/raid", rel, imp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no raid package was checked")
	}
}

// Exempt integer literals above 999, as "file:line": each entry needs a comment saying why. Empty at R0.
var literalExemptions = map[string]string{}

func TestNoLargeLiterals(t *testing.T) {
	var bad []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.INT {
				return true
			}
			v, err := strconv.ParseUint(strings.ReplaceAll(lit.Value, "_", ""), 0, 64)
			pos := fset.Position(lit.Pos())
			where := filepath.ToSlash(pos.Filename) + ":" + strconv.Itoa(pos.Line)
			if (err != nil || v > 999) && literalExemptions[where] == "" {
				bad = append(bad, where+" "+lit.Value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("integer literal above 999 (move it to a data file): %s", b)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
