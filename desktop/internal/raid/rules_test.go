package raid_test

// Structure rules for desktop/internal/raid (PLAN.md 3.1, DEC-36):
//   - TestImports: a package imports only the raid packages its imports.json entry lists, never package main or
//     anything else under desktop/, and every package has an entry.
//   - TestNoLargeLiterals: no number above 999 in non-test code, written as a literal or a literal-only expression
//     (1000, 1e3, 1000.0, 999+1), so spell, item and encounter ids live in data files.

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/thegamersvibe/relax-coach/desktop"
const tree = module + "/internal/raid"

func TestImports(t *testing.T) {
	raw, err := os.ReadFile("imports.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &allowed); err != nil {
		t.Fatal(err)
	}
	// Every non-test .go file is parsed, whatever its build tags or GOOS suffix: a *_windows.go file is checked
	// on a Linux runner too. Files are grouped by directory = package.
	imports := map[string]map[string][]string{} // package -> import -> files
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(filepath.Dir(path))
		if rel == "." {
			rel = ""
		}
		if imports[rel] == nil {
			imports[rel] = map[string][]string{}
		}
		for _, spec := range f.Imports {
			imp, _ := strconv.Unquote(spec.Path.Value)
			imports[rel][imp] = append(imports[rel][imp], filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(imports) == 0 {
		t.Fatal("no raid package was checked")
	}
	pkgs := make([]string, 0, len(imports))
	for rel := range imports {
		pkgs = append(pkgs, rel)
	}
	sort.Strings(pkgs)
	for _, rel := range pkgs {
		entry, ok := allowed[rel]
		if !ok {
			t.Errorf("%s has no entry in imports.json", rel)
			continue
		}
		var may []string
		if err := json.Unmarshal(entry, &may); err != nil {
			t.Fatalf("imports.json %s: %v", rel, err)
		}
		for imp, files := range imports[rel] {
			switch {
			case imp == module:
				t.Errorf("%s imports package main (%s) in %v", rel, imp, files)
			case strings.HasPrefix(imp, tree+"/"):
				dep := strings.TrimPrefix(imp, tree+"/")
				if !contains(may, dep) {
					t.Errorf("%s imports %s, which imports.json does not allow (%v)", rel, dep, files)
				}
			case strings.HasPrefix(imp, module+"/"):
				t.Errorf("%s imports %s, outside internal/raid (%v)", rel, imp, files)
			}
		}
	}
}

// Exempt number literals above 999, as "file:line": each entry needs a comment saying why. Empty at R0.
var literalExemptions = map[string]string{}

// literal evaluates an expression built only from number literals (1e3, 1000.0, 999+1, -(2*600)); ok is false
// for anything that names an identifier or a call.
func literal(e ast.Expr) (constant.Value, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.INT && x.Kind != token.FLOAT {
			return nil, false
		}
		v := constant.MakeFromLiteral(x.Value, x.Kind, 0)
		return v, v.Kind() != constant.Unknown
	case *ast.ParenExpr:
		return literal(x.X)
	case *ast.UnaryExpr:
		v, ok := literal(x.X)
		if !ok || (x.Op != token.SUB && x.Op != token.ADD) {
			return nil, false
		}
		return constant.UnaryOp(x.Op, v, 0), true
	case *ast.BinaryExpr:
		a, okA := literal(x.X)
		b, okB := literal(x.Y)
		switch {
		case !okA || !okB:
			return nil, false
		case x.Op == token.ADD, x.Op == token.SUB, x.Op == token.MUL:
			return constant.BinaryOp(a, x.Op, b), true
		case x.Op == token.QUO && constant.Sign(b) != 0:
			op := token.QUO
			if a.Kind() == constant.Int && b.Kind() == constant.Int {
				op = token.QUO_ASSIGN // integer division
			}
			return constant.BinaryOp(a, op, b), true
		case x.Op == token.SHL && b.Kind() == constant.Int:
			if n, exact := constant.Uint64Val(b); exact && n < 64 {
				return constant.Shift(a, token.SHL, uint(n)), true
			}
		}
	}
	return nil, false
}

func TestNoLargeLiterals(t *testing.T) {
	limit := constant.MakeInt64(999)
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
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			v, ok := literal(e)
			if !ok {
				return true
			}
			pos := fset.Position(e.Pos())
			where := filepath.ToSlash(pos.Filename) + ":" + strconv.Itoa(pos.Line)
			abs := v
			if constant.Sign(v) < 0 {
				abs = constant.UnaryOp(token.SUB, v, 0)
			}
			if constant.Compare(abs, token.GTR, limit) && literalExemptions[where] == "" {
				bad = append(bad, where+" "+v.ExactString())
			}
			return false // a literal-only expression is judged as a whole, once
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("number literal above 999 (move it to a data file): %s", b)
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
