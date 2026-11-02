package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityRemoveLabelsNestedSharedAndQuantified(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	p, err := ctx.BoolConst("label-input")
	must(err)
	closeExpr(t, p)
	first, err := m.Symbols.Intern("inner")
	must(err)
	second, err := m.Symbols.Intern("outer-a")
	must(err)
	third, err := m.Symbols.Intern("outer-b")
	must(err)
	inner := wrap(m.MkLabel(false, []usymbol.Symbol{first}, raw(p)))
	outer := wrap(m.MkLabel(true, []usymbol.Symbol{second, third}, raw(inner)))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(outer)}))
	boolean, err := m.GetSort(raw(p))
	must(err)
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(outer), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 7}))
	root := wrap(m.MkAndArgs([]ast.ASTHandle{raw(outer), raw(outer), raw(q)}))
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	output, labels, err := g.RemoveLabels(root)
	must(err)
	defer output.Close()
	defer func() {
		for i := range labels {
			must(labels[i].Close())
			must(labels[i].Close())
		}
	}()
	if len(labels) != 3 {
		t.Fatalf("shared labels duplicated: %d", len(labels))
	}
	for i, name := range []usymbol.Symbol{first, second, third} {
		if labels[i].Name != name || labels[i].Positive != (i > 0) || raw(labels[i].Term) != raw(p) {
			t.Fatalf("label order/value mismatch at %d", i)
		}
	}
	for i := uint32(0); i < 2; i++ {
		h, err := m.GetArg(raw(output), i)
		must(err)
		if h != raw(p) {
			t.Fatal("application label survived")
		}
	}
	quant, err := m.GetArg(raw(output), 2)
	must(err)
	body, err := m.GetQuantifierBody(quant)
	must(err)
	if body != raw(p) {
		t.Fatal("quantifier body label survived")
	}
	pat, err := m.GetPattern(quant, 0)
	must(err)
	if pat != raw(pattern) {
		t.Fatal("quantifier annotation was traversed")
	}
	weight, err := m.GetQuantifierWeight(quant)
	must(err)
	if weight != 7 {
		t.Fatal("quantifier metadata changed")
	}
	original, err := m.GetQuantifierBody(raw(q))
	must(err)
	if original != raw(outer) {
		t.Fatal("input mutated")
	}
	must(g.Close())
	if raw(labels[0].Term) != raw(p) {
		t.Fatal("label lost ownership after graph closure")
	}
	// A label-free variable is passed through without becoming a background constant.
	g2, err := NewRPFP(ctx)
	must(err)
	defer g2.Close()
	v := wrap(m.MkVar(0, boolean))
	identity, none, err := g2.RemoveLabels(v)
	must(err)
	defer identity.Close()
	if len(none) != 0 || raw(identity) != raw(v) || len(g2.vocabulary.constants) != 0 {
		t.Fatal("label-free traversal changed input")
	}
}
