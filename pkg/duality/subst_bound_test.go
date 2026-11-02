package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestDualitySubstBoundDepthPatternsAndSimultaneousReplacement(t *testing.T) {
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
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	yes := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(yes))
	must(err)
	v0 := wrap(m.MkVar(0, boolean))
	v1 := wrap(m.MkVar(1, boolean))
	v2 := wrap(m.MkVar(2, boolean))
	body := wrap(m.MkOrArgs([]ast.ASTHandle{raw(v0), raw(v1), raw(v2)}))
	pattern := wrap(m.MkPattern([]ast.ASTHandle{raw(body)}))
	q := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(body), Patterns: []ast.ASTHandle{raw(pattern)}, Weight: 9}))
	root := wrap(m.MkAnd(raw(v1), raw(q)))
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	result, err := g.SubstBound(map[uint32]*Expr{0: v1, 1: yes}, root)
	must(err)
	defer func() { must(result.Close()) }()
	first, err := m.GetArg(raw(result), 0)
	must(err)
	if first != raw(yes) {
		t.Fatal("outer shared variable was not replaced")
	}
	nested, err := m.GetArg(raw(result), 1)
	must(err)
	changedBody, err := m.GetQuantifierBody(nested)
	must(err)
	for i, want := range []ast.ASTHandle{raw(v0), raw(v1), raw(yes)} {
		child, err := m.GetArg(changedBody, uint32(i))
		must(err)
		if child != want {
			t.Fatal("binder-depth memo collision", i)
		}
	}
	changedPattern, err := m.GetPattern(nested, 0)
	must(err)
	patternBody, err := m.GetArg(changedPattern, 0)
	must(err)
	if patternBody != changedBody {
		t.Fatal("pattern did not follow body index shift")
	}
	weight, err := m.GetQuantifierWeight(nested)
	must(err)
	if weight != 9 {
		t.Fatal("quantifier metadata changed")
	}
	originalBody, err := m.GetQuantifierBody(raw(q))
	must(err)
	originalLast, err := m.GetArg(originalBody, 2)
	must(err)
	if originalLast != raw(v2) {
		t.Fatal("input expression mutated")
	}

	// Replacement v1 is inserted unchanged, even below the binder; applying
	// substitution recursively would instead replace it with true.
	replacement, err := ctx.BoolConst("subst-inserted")
	must(err)
	closeExpr(t, replacement)
	original, err := ctx.BoolConst("subst-original")
	must(err)
	closeExpr(t, original)
	input := wrap(m.MkAnd(raw(original), raw(v0)))
	output, err := g.SubstBound(map[uint32]*Expr{0: replacement}, input)
	must(err)
	defer output.Close()
	od, err := m.GetDecl(raw(original))
	must(err)
	rd, err := m.GetDecl(raw(replacement))
	must(err)
	if _, ok := g.vocabulary.constants[od]; !ok {
		t.Fatal("input constant missing from background")
	}
	if _, ok := g.vocabulary.constants[rd]; ok {
		t.Fatal("inserted replacement incorrectly declared")
	}
	must(g.Close())
	child, err := m.GetArg(raw(output), 1)
	must(err)
	if child != raw(replacement) {
		t.Fatal("output did not survive graph closure")
	}
}
