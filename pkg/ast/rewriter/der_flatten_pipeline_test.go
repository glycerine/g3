package rewriter

// Source: src/ast/rewriter/der.cpp:149-226 (der::reduce1).

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md ast_rewriter_03-F1: der::reduce1 in 4.7.1 reads only the direct
// arguments of a top-level or (no flatten_or), and leaves existential
// quantifiers alone, so a disequality nested in an inner or, or an
// existential conjunction, is not eliminated.
func TestDERDoesNotFlattenNestedUniversalOrExistentialBodies(t *testing.T) {
	for _, forall := range []bool{true, false} {
		t.Run(map[bool]string{true: "forall", false: "exists"}[forall], func(t *testing.T) {
			f := newFactorFixture(t)
			d, err := NewDER(f.m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, d.Close()) })
			x, err := f.m.MkVar(0, f.sort)
			astTestMust(t, err)
			equality, err := f.m.MkEq(x, f.numeral(3))
			astTestMust(t, err)
			literal := equality
			if forall {
				literal, err = f.m.MkNot(equality)
				astTestMust(t, err)
			}
			boolean, err := f.m.MkBoolSort()
			astTestMust(t, err)
			p, err := f.m.MkConst(astTestSymbol(t, f.m, "der-flat-p"), boolean)
			astTestMust(t, err)
			qv, err := f.m.MkConst(astTestSymbol(t, f.m, "der-flat-q"), boolean)
			astTestMust(t, err)
			var nested, body ast.ASTHandle
			if forall {
				nested, err = f.m.MkOr(literal, qv)
				astTestMust(t, err)
				body, err = f.m.MkOr(p, nested)
			} else {
				nested, err = f.m.MkAndArray([]ast.ASTHandle{p, qv})
				astTestMust(t, err)
				body, err = f.m.MkAndArray([]ast.ASTHandle{literal, nested})
			}
			astTestMust(t, err)
			q := derQuantifier(t, f.m, forall, []ast.ASTHandle{f.sort}, body)
			if got := derReduce(t, d, f.m, q); got != q {
				t.Fatal("nested body was flattened and reduced", got, q)
			}
		})
	}
}
