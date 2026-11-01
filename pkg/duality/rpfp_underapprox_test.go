package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestRPFPUnderapproximationUsesSourceShallowHiding(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	relation, err := ctx.BoolConst("under-R")
	must(err)
	closeExpr(t, relation)
	private, err := ctx.BoolConst("under-private")
	must(err)
	closeExpr(t, private)
	nested, err := private.Not()
	must(err)
	closeExpr(t, nested)
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	node, err := g.CreateNode(relation)
	must(err)
	for n, formula := range []*Expr{private, nested} {
		replacement, err := NewTransformer(ctx, nil, nil, formula)
		must(err)
		must(node.Underapprox.Close())
		node.Underapprox = replacement
		output, err := g.GetUnderapprox(node)
		must(err)
		b, vars, err := g.RedVars(node)
		must(err)
		if len(vars) != 0 {
			t.Fatal("nullary relation has parameters")
		}
		negative, err := b.Not()
		must(err)
		expectedBody := formula
		var hidden *Expr
		if n == 0 {
			hidden, err = rpfpVariable(private, "@q_", node.Number)
			must(err)
			expectedBody = hidden
		}
		expected, err := negative.Or(expectedBody)
		must(err)
		got, err := output.Raw()
		must(err)
		want, err := expected.Raw()
		must(err)
		if got != want {
			t.Fatal("source root-only hiding differs", n)
		}
		must(expected.Close())
		if hidden != nil {
			must(hidden.Close())
		}
		must(negative.Close())
		must(b.Close())
		must(output.Close())
	}
	// At the outer quantifier, source cloning changes only the body.
	ph, err := private.Raw()
	must(err)
	rh, err := relation.Raw()
	must(err)
	sort, err := m.MkBoolSort()
	must(err)
	body, err := nested.Raw()
	must(err)
	pattern, err := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{body})
	must(err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	must(err)
	qr, err := m.NewRef(q)
	must(err)
	replaced, err := substituteHiddenRoot(ctx, q, map[ast.ASTHandle]ast.ASTHandle{ph: rh}, node.Number)
	must(err)
	qh, err := m.RefGet(replaced)
	must(err)
	kept, err := m.GetPattern(qh, 0)
	must(err)
	if kept != pattern {
		t.Fatal("outer quantifier patterns changed")
	}
	qb, err := m.GetQuantifierBody(qh)
	must(err)
	arg, err := m.GetArg(qb, 0)
	must(err)
	if arg != rh {
		t.Fatal("quantifier body was not substituted")
	}
	must(m.DelRef(&replaced))
	must(m.DelRef(&qr))
	must(g.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("underapproximation leaked", before, after)
	}
}
