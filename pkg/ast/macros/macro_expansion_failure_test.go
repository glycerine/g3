package macros_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

// Exercise cold member construction on both sides of a macro hit. The
// fixture closes every owner and checks that the arena budget returns to zero.
func TestMacroExpansionConstructorFailure(t *testing.T) {
	for fault := int64(0); fault < 24; fault++ {
		t.Run(fmt.Sprint(fault), func(t *testing.T) {
			m := fixture(t)
			manager, err := macros.NewManager(m)
			must(t, err)
			t.Cleanup(func() { must(t, manager.Close()) })
			b, err := m.MkBoolSort()
			must(t, err)
			x, err := m.MkVar(0, b)
			must(t, err)
			f := declaration(t, m, "identity", []ast.ASTHandle{b}, b)
			q := quantifiedMacro(t, m, f, x, b)
			inserted, err := manager.Insert(f, q, ast.ASTHandle{}, ast.ASTDependency{})
			must(t, err)
			if !inserted {
				t.Fatal("expected macro insertion")
			}
			term, err := m.MkApp(f, []ast.ASTHandle{x})
			must(t, err)
			out, err := m.NewRef(term)
			must(t, err)
			t.Cleanup(func() { must(t, m.DelRef(&out)) })
			proof, err := m.NewRef(ast.ASTHandle{})
			must(t, err)
			t.Cleanup(func() { must(t, m.DelRef(&proof)) })
			m.Budget().FailAfter(fault)
			_, err = manager.ExpandMacros(term, ast.ASTHandle{}, ast.ASTDependency{}, out, proof)
			m.Budget().FailAfter(-1)
			if err != nil && !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatalf("unexpected expansion error: %v", err)
			}
			if err == nil {
				result, e := m.RefGet(out)
				must(t, e)
				if result != x {
					t.Fatal("identity expansion returned a different term")
				}
			}
		})
	}
}
