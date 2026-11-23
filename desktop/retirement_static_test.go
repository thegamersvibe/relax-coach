package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRetirementSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func TestRetirementSerializesNativeLaunches(t *testing.T) {
	source := readRetirementSource(t, "main.go")
	if !strings.Contains(source, "if !nativeSingleInstance(mode, nativeAcquireInstance, nativeFocusOther)") {
		t.Fatal("ordinary launches can recover another live retirement pass's journal")
	}
}

func TestRetirementBridgeOnlyAppUpdateCapabilities(t *testing.T) {
	b := newNativeBridge(newRetirementApp(""), false, nativeServices{})
	defer b.Close()
	s := b.services
	if s.checkApp == nil || s.applyApp == nil || s.ready == nil {
		t.Fatal("app updater/health boundary missing")
	}
	if s.check != nil || s.apply != nil || s.checkExtra != nil || s.applyExtra != nil || s.open != nil || s.place != nil || s.choose != nil || s.chooseFile != nil || s.raidConfig != nil || s.appLast != nil || s.startup != nil || s.gameOpen != nil || s.selfUpdate != nil {
		t.Fatal("non-app capability in retirement bridge")
	}
}
func TestRetirementUserDataNeverUsesReplacingRename(t *testing.T) {
	files, err := filepath.Glob("retirement*.go")
	if err != nil {
		t.Fatal(err)
	}
	metadataRenames := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		set := token.NewFileSet()
		f, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					var b bytes.Buffer
					format.Node(&b, set, c)
					fn, ok := decl.(*ast.FuncDecl)
					if name == "retirement_quarantine.go" && ok && fn.Name.Name == "atomicJSON" && b.String() == "q.root.Rename(temp, filepath.Join(q.run, name))" {
						metadataRenames++
						return false // The only allowed replacing rename: internal metadata.
					}
				}
				if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Rename" {
					var b bytes.Buffer
					format.Node(&b, set, sel)
					t.Errorf("replacing user-data rename in %s: %s", name, b.String())
				}
				return true
			})
		}
	}
	if metadataRenames != 1 {
		t.Fatalf("want exactly one documented internal metadata rename, got %d", metadataRenames)
	}
}

func TestRetirementNeverDeletesUserDataStatic(t *testing.T) {
	files, err := filepath.Glob("retirement*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		set := token.NewFileSet()
		f, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "RemoveAll" {
					t.Errorf("recursive deletion in %s.%s", name, fn.Name.Name)
				}
				if sel.Sel.Name == "Remove" {
					var b bytes.Buffer
					format.Node(&b, set, c)
					// These are exclusively app-created empty scaffolding and journal
					// metadata. No source/target name can be passed to a deletion API.
					allowed := map[string]string{
						"retirement_quarantine.go:close":     "q.root.Remove(q.created[i])",
						"retirement_journal.go:clearPending": `q.root.Remove(filepath.Join(q.run, "pending.json"))`,
						"retirement_journal.go:recover":      "prior.root.Remove(dir)",
					}
					if allowed[name+":"+fn.Name.Name] != b.String() {
						t.Errorf("data deletion: %s.%s: %s", name, fn.Name.Name, b.String())
					}
				}
				return true
			})
		}
	}
}
