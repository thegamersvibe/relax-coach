package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep the app's sole comparison entrypoint auditable: the parsed officer value
// goes directly to Compare, with no construction, aliases or intervening edits.
// This source gate supplements (not replaces) real file/error/reread tests.
func raidOfficerPathCheck(src []byte) (int, error) {
	bad := errors.New("officer input must flow directly from ParseOfficer(data) to Compare")
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", src, 0)
	if err != nil {
		return 0, err
	}
	aliases := map[string]bool{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path != "github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare" {
			continue
		}
		alias := "compare"
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if alias == "." {
			return 0, bad
		}
		aliases[alias] = true
	}
	var calls []*ast.SelectorExpr
	ast.Inspect(file, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := s.X.(*ast.Ident)
		if !ok || !aliases[id.Name] {
			return true
		}
		if s.Sel.Name == "OfficerFile" {
			err = bad
		}
		if s.Sel.Name == "Compare" || s.Sel.Name == "ParseOfficer" {
			if id.Obj != nil {
				err = bad
			} // no shadowed package qualifier
			calls = append(calls, s)
		}
		return true
	})
	if err != nil || len(calls) == 0 {
		return 0, err
	}
	if len(calls) != 2 {
		return 0, bad
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "raidComparisonFile" {
			fn = f
		}
	}
	if fn == nil || fn.Body == nil || len(fn.Body.List) == 0 {
		return 0, bad
	}
	var parsed *ast.Ident
	var parseSelector *ast.SelectorExpr
	for _, stmt := range fn.Body.List {
		a, ok := stmt.(*ast.AssignStmt)
		if !ok || len(a.Lhs) != 2 || len(a.Rhs) != 1 {
			continue
		}
		call, ok := a.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ParseOfficer" {
			continue
		}
		arg, ok := call.Args[0].(*ast.Ident)
		if !ok || arg.Name != "data" {
			return 0, bad
		}
		parsed, _ = a.Lhs[0].(*ast.Ident)
		parseSelector = sel
	}
	last, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(last.Results) != 1 || parsed == nil || parsed.Obj == nil {
		return 0, bad
	}
	call, ok := last.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 5 {
		return 0, bad
	}
	compareSelector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || compareSelector.Sel.Name != "Compare" {
		return 0, bad
	}
	arg, ok := call.Args[3].(*ast.Ident)
	if !ok || arg.Obj != parsed.Obj {
		return 0, bad
	}
	for _, s := range calls {
		if s != parseSelector && s != compareSelector {
			return 0, bad
		}
	}
	refs := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Obj == parsed.Obj {
			refs++
		}
		return true
	})
	if refs != 2 {
		return 0, bad
	} // declaration and Compare argument, nothing else
	return 1, nil
}

func TestRaidPageOfficerParserPath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	paths := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		n, err := raidOfficerPathCheck(src)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		paths += n
	}
	if paths != 1 {
		t.Fatal("missing or duplicate comparison entrypoint")
	}
	// Challenge the checker itself; ordinary filesystem reads do not see Go
	// build overlays, so these are not mislabeled as runtime mutation kills.
	src, err := os.ReadFile("raid_pipeline.go")
	if err != nil {
		t.Fatal(err)
	}
	good := string(src)
	aliased := strings.ReplaceAll(strings.Replace(good, `"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"`, `cmp "github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"`, 1), "compare.", "cmp.")
	if n, err := raidOfficerPathCheck([]byte(aliased)); n != 1 || err != nil {
		t.Fatal("valid import alias was rejected")
	}
	for _, broken := range []string{
		strings.Replace(good, "officer, err := compare.ParseOfficer(data)", `officer, err := compare.OfficerFile{Version: 1}, error(nil)`, 1),
		strings.Replace(good, "return compare.Compare", `officer.Officer = "different"; return compare.Compare`, 1),
		good + "\ntype forged = compare.OfficerFile\n",
		good + "\nvar bypass = compare.Compare\n",
		good + "\nfunc otherRoute() { compare.ParseOfficer(nil) }\n",
	} {
		if _, err := raidOfficerPathCheck([]byte(broken)); err == nil {
			t.Fatal("source gate accepted a bypass")
		}
	}
}
