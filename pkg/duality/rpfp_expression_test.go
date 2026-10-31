package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRPFPExpressionComplexityAndNames(t *testing.T) {
	c, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err := c.BoolConst("rpfp-a")
	must(err)
	closeExpr(t, a)
	b, err := c.BoolConst("rpfp-b")
	must(err)
	closeExpr(t, b)
	ah, err := a.Raw()
	must(err)
	bh, err := b.Raw()
	must(err)
	sub, err := m.MkAnd(ah, bh)
	must(err)
	not, err := m.MkNot(sub)
	must(err)
	disj, err := m.MkOr(sub, not)
	must(err)
	// The conjunction occurs twice but under NOT the source does not descend.
	root, err := m.MkAnd(disj, sub)
	must(err)
	wrapped, err := NewExpr(c, root)
	must(err)
	closeExpr(t, wrapped)
	count, err := CountOperators(wrapped)
	must(err)
	if count != 3 {
		t.Fatal("shared Boolean operators", count)
	}
	sort, err := m.MkBoolSort()
	must(err)
	n1, err := m.Symbols.Intern("rpfp-x")
	must(err)
	n2, err := m.Symbols.Intern("rpfp-y")
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort, sort}, Names: []usymbol.Symbol{n1, n2}, Body: root})
	must(err)
	q, err := NewExpr(c, quantified)
	must(err)
	closeExpr(t, q)
	count, err = CountOperators(q)
	must(err)
	if count != 7 {
		t.Fatal("quantifier complexity", count)
	}
	negative, err := SuffixVariable(a, -3)
	must(err)
	closeExpr(t, negative)
	hidden, err := HideVariable(a, 42)
	must(err)
	closeExpr(t, hidden)
	for _, tc := range []struct {
		expr *Expr
		name string
	}{{negative, "rpfp-a_-3"}, {hidden, "@p_rpfp-a_42"}} {
		decl, e := tc.expr.Decl()
		must(e)
		name, e := decl.Name()
		must(e)
		text, e := name.String()
		must(e)
		if text != tc.name {
			t.Fatal("variable name", text, tc.name)
		}
		must(decl.Close())
		gotSort, e := tc.expr.Sort()
		must(e)
		raw, e := gotSort.Raw()
		must(e)
		if raw != sort {
			t.Fatal("variable sort", raw, sort)
		}
		must(gotSort.Close())
	}
}
