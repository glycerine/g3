package macros_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

func TestMacroForbiddenPublicationAfterPartialWalk(t *testing.T) {
	m := fixture(t)
	owner, err := macros.NewManager(m)
	must(t, err)
	t.Cleanup(func() { must(t, owner.Close()) })
	sort, err := m.MkBoolSort()
	must(t, err)
	x, err := m.MkVar(0, sort)
	must(t, err)
	var decls, terms, equations []ast.ASTHandle
	for _, name := range []string{"f", "g", "h"} {
		d := declaration(t, m, name, []ast.ASTHandle{sort}, sort)
		term, err := m.MkApp(d, []ast.ASTHandle{x})
		must(t, err)
		eq, err := m.MkEq(term, x)
		must(t, err)
		decls, terms, equations = append(decls, d), append(terms, term), append(equations, eq)
	}
	check := func(allowed [3]bool) {
		t.Helper()
		for i, eq := range equations {
			_, _, ok, err := owner.Util().IsLeftSimpleMacro(eq, 1)
			must(t, err)
			if ok != allowed[i] {
				t.Fatalf("equation %d allowed = %v, want %v", i, ok, allowed[i])
			}
		}
	}
	must(t, owner.MarkForbidden(terms[:1]))
	must(t, owner.PushScope())
	// The work stack visits the valid application before the null root.
	if err = owner.MarkForbidden([]ast.ASTHandle{{}, terms[1]}); err == nil || !owner.IsForbidden(decls[1]) {
		t.Fatal("expected a partially completed traversal", err)
	}
	check([3]bool{false, true, true})
	owner.Util().SetForbidden(decls[2:])
	check([3]bool{true, true, false})
	if err = owner.MarkForbidden([]ast.ASTHandle{{}, terms[2]}); err == nil {
		t.Fatal("expected invalid root")
	}
	check([3]bool{true, true, false})
	must(t, owner.MarkForbidden(nil))
	check([3]bool{})
	must(t, owner.PopScope(1))
	check([3]bool{false, true, true})
}

func BenchmarkMacroForbiddenScope(b *testing.B) {
	for _, n := range []int{64, 512} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			m := fixture(b)
			owner, err := macros.NewManager(m)
			must(b, err)
			b.Cleanup(func() { must(b, owner.Close()) })
			sort, err := m.MkBoolSort()
			must(b, err)
			x, err := m.MkVar(0, sort)
			must(b, err)
			var terms []ast.ASTHandle
			for i := 0; i < n; i++ {
				d := declaration(b, m, fmt.Sprintf("f%d", i), []ast.ASTHandle{sort}, sort)
				term, err := m.MkApp(d, []ast.ASTHandle{x})
				must(b, err)
				terms = append(terms, term)
			}
			must(b, owner.MarkForbidden(terms))
			b.ReportAllocs()
			for b.Loop() {
				if err := owner.PushScope(); err != nil {
					b.Fatal(err)
				}
				if err := owner.PopScope(1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
