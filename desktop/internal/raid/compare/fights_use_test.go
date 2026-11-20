package compare

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func comparisonSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[path] = string(b)
	}
	return out
}

func TestCompareFightsUseGuard(t *testing.T) {
	if err := checkFightsUse(comparisonSources(t)); err != nil {
		t.Fatal(err)
	}
}

func TestCompareFightsUseGuardControls(t *testing.T) {
	src := comparisonSources(t)
	original := src["compare.go"]
	for _, insert := range []string{
		`_ = len(fights)`,
		`_ = fights[0].ID`,
		`for _, f := range fights { _ = f.Boss }`,
		`alias := fights; _ = alias`,
		`fights = nil`,
		`_ = &fights`,
		`consume := func(v any) {}; consume(fights)`,
		`fights = append(fights, model.Fight{})`,
		`forward := score.NightRoleParts; _, _ = forward(fights, scores[0], c)`,
	} {
		t.Run(insert, func(t *testing.T) {
			src["compare.go"] = strings.Replace(original, "out := Report{Officer: officer.Officer}", insert+"; out := Report{Officer: officer.Officer}", 1)
			if err := checkFightsUse(src); err == nil {
				t.Fatal("fights-use guard accepted a forbidden use")
			}
		})
	}
	src["compare.go"] = original
	src["side.go"] = `package compare; import m "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"; type otherFight = m.Fight`
	if err := checkFightsUse(src); err == nil {
		t.Fatal("guard missed another fight type in another file")
	}
	delete(src, "side.go")
	src["compare.go"] = strings.Replace(original, "score.NightRoleParts(fights, n, c)", "score.NightRoleParts(nil, n, c)", 1)
	if err := checkFightsUse(src); err == nil {
		t.Fatal("guard missed failure to forward fights")
	}
}

// This intentionally inspects every production file, not just compare.go.
// A fight slice may exist only as Compare's fifth argument. Every reference
// to that binding must be the direct first argument of NightRoleParts. Aliases,
// indexing, range, reflection, len, reassignment and helpers therefore fail.
func checkFightsUse(sources map[string]string) error {
	fightTypes, calls, comparisons := 0, 0, 0
	for path, src := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return err
		}
		imports := map[string]string{}
		for _, i := range file.Imports {
			p, err := strconv.Unquote(i.Path.Value)
			if err != nil {
				return err
			}
			name := filepath.Base(p)
			if i.Name != nil {
				name = i.Name.Name
			}
			if name == "." {
				return fmt.Errorf("dot import prevents fight-use audit")
			}
			imports[name] = p
		}
		isPackage := func(e ast.Expr, base string) bool {
			id, ok := e.(*ast.Ident)
			return ok && id.Obj == nil && imports[id.Name] == "github.com/thegamersvibe/relax-coach/desktop/internal/raid/"+base
		}
		var parameter *ast.Ident
		var fightType ast.Expr
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Compare" {
				continue
			}
			comparisons++
			if len(fn.Type.Params.List) != 5 {
				return fmt.Errorf("Compare must have five inputs")
			}
			field := fn.Type.Params.List[4]
			if len(field.Names) != 1 {
				return fmt.Errorf("fifth input must name the fight slice")
			}
			array, ok := field.Type.(*ast.ArrayType)
			if !ok || array.Len != nil {
				return fmt.Errorf("fifth input must be a slice")
			}
			sel, ok := array.Elt.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Fight" || !isPackage(sel.X, "model") {
				return fmt.Errorf("fifth input must be []model.Fight")
			}
			parameter, fightType = field.Names[0], sel
		}
		var bad error
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, node)
			if sel, ok := node.(*ast.SelectorExpr); ok && sel.Sel.Name == "Fight" && isPackage(sel.X, "model") {
				fightTypes++
				if node != fightType {
					bad = fmt.Errorf("%s: model.Fight outside fifth input", path)
				}
			}
			if id, ok := node.(*ast.Ident); ok && parameter != nil && id.Obj == parameter.Obj && id != parameter {
				call, ok := parent.(*ast.CallExpr)
				valid := false
				if ok && len(call.Args) == 3 && call.Args[0] == id {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && isPackage(sel.X, "score") && sel.Sel.Name == "NightRoleParts" {
						valid = true
						calls++
					}
				}
				if !valid {
					bad = fmt.Errorf("%s: fights used beyond forwarding to score.NightRoleParts", path)
				}
			}
			return true
		})
		if bad != nil {
			return bad
		}
	}
	if comparisons != 1 || fightTypes != 1 || calls != 1 {
		return fmt.Errorf("want one comparison, one fight type and one forwarding call; got %d/%d/%d", comparisons, fightTypes, calls)
	}
	return nil
}
