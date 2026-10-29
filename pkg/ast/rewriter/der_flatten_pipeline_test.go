package rewriter

// Source: src/ast/rewriter/der.cpp:166-174 flatten_or/flatten_and.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDERFlattensNestedUniversalAndExistentialBodies(t *testing.T) {
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
				nested, err = f.m.MkOr(p, qv)
				astTestMust(t, err)
				body, err = f.m.MkOr(literal, nested)
			} else {
				nested, err = f.m.MkAndArray([]ast.ASTHandle{p, qv})
				astTestMust(t, err)
				body, err = f.m.MkAndArray([]ast.ASTHandle{literal, nested})
			}
			astTestMust(t, err)
			got := derReduce(t, d, f.m, derQuantifier(t, f.m, forall, []ast.ASTHandle{f.sort}, body))
			if kind, xerr := f.m.GetKind(got); xerr != nil || kind == ast.ASTQuantifier {
				t.Fatal("flattened elimination left quantifier", kind, xerr)
			}
		})
	}
}
