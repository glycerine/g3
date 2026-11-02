package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestDualityHistoryProposerMatchesSuffixesParametersAndBackground(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	constant := func(name string) *Expr { v, err := ctx.BoolConst(name); must(err); closeExpr(t, v); return v }
	oldX, newX, z := constant("x@@old"), constant("x@@new"), constant("unmatched@@old")
	oldP, newP := constant("p@@old"), constant("p@@new")
	old, err := NewRPFP(ctx)
	must(err)
	defer old.Close()
	current, err := NewRPFP(ctx)
	must(err)
	defer current.Close()
	for _, pair := range []struct {
		g    *RPFP
		term *Expr
	}{{old, oldP}, {current, newP}} {
		decl, err := pair.term.Decl()
		must(err)
		must(pair.g.DeclareConstant(decl))
		must(decl.Close())
	}
	boolean, err := oldX.Sort()
	must(err)
	defer boolean.Close()
	predicate := func(g *RPFP, name string, arg *Expr) *Node {
		t.Helper()
		symbol, err := ctx.StringSymbol(name)
		must(err)
		decl, err := ctx.Function(symbol, []*Sort{boolean}, boolean)
		must(err)
		defer decl.Close()
		term, err := decl.Call(arg)
		must(err)
		defer term.Close()
		node, err := g.CreateNode(term)
		must(err)
		return node
	}
	annotate := func(node *Node, param, formula *Expr) {
		fact, err := NewTransformer(ctx, nil, []*Expr{param}, formula)
		must(err)
		defer fact.Close()
		must(replaceNodeAnnotation(node, fact))
	}
	and, err := oldX.And(oldP)
	must(err)
	closeExpr(t, and)
	first := predicate(old, "R@@old", oldX)
	annotate(first, oldX, and)
	_ = predicate(old, "R@@old", oldX) // Canonical true is not proposed.
	wrong, err := z.And(oldP)
	must(err)
	closeExpr(t, wrong)
	annotate(predicate(old, "R@@old", z), z, wrong)
	or, err := oldX.Or(oldP)
	must(err)
	closeExpr(t, or)
	annotate(predicate(old, "R@@old", oldX), oldX, or)
	backgroundFunction := func(name string) *FuncDecl {
		symbol, err := ctx.StringSymbol(name)
		must(err)
		decl, err := ctx.Function(symbol, []*Sort{boolean}, boolean)
		must(err)
		closeDecl(t, decl)
		return decl
	}
	oldF, newF := backgroundFunction("bg@@old"), backgroundFunction("bg@@new")
	must(old.DeclareConstant(oldF))
	must(current.DeclareConstant(newF))
	oldApplication, err := oldF.Call(oldX)
	must(err)
	closeExpr(t, oldApplication)
	annotate(predicate(old, "R@@old", oldX), oldX, oldApplication)

	target := predicate(current, "R@@new", newX)
	unrelated := predicate(current, "S@@new", newX)
	instances, err := NewRPFPWithVocabulary(current.vocabulary)
	must(err)
	defer instances.Close()
	instance, err := instances.CloneNode(target)
	must(err)
	other, err := instances.CloneNode(unrelated)
	must(err)
	proposer, err := NewHistoryProposer(old, current)
	must(err)
	defer proposer.Close()
	must(old.Close())
	proposals, err := proposer.Propose(instance)
	must(err)
	if len(proposals) != 3 {
		t.Fatal("history match/full/unmatched filtering changed", len(proposals))
	}
	expectedAnd, err := newX.And(newP)
	must(err)
	closeExpr(t, expectedAnd)
	expectedOr, err := newX.Or(newP)
	must(err)
	closeExpr(t, expectedOr)
	expectedApplication, err := newF.Call(newX)
	must(err)
	closeExpr(t, expectedApplication)
	for i, want := range []*Expr{expectedAnd, expectedOr, expectedApplication} {
		formula, err := proposals[i].Formula()
		must(err)
		if raw(formula) != raw(want) {
			t.Fatal("history parameter/background remapping or source order changed", i)
		}
		must(formula.Close())
	}
	must(proposals[0].SetEmpty())
	again, err := proposer.Propose(instance)
	must(err)
	formula, err := again[0].Formula()
	must(err)
	if raw(formula) != raw(expectedAnd) {
		t.Fatal("mutable returned proposal corrupted history cache")
	}
	must(formula.Close())
	missing, err := proposer.Propose(other)
	must(err)
	if len(missing) != 0 {
		t.Fatal("unmatched predicate proposed facts")
	}
	must(proposer.Close())
	// Returned facts keep all references after proposer and old graph teardown.
	for _, fact := range proposals {
		formula, err := fact.Formula()
		must(err)
		must(formula.Close())
		must(fact.Close())
	}
	for _, fact := range again {
		must(fact.Close())
	}
	before := m.Stats()
	if _, err := proposer.Propose(instance); err == nil {
		t.Fatal("closed proposer accepted query")
	}
	after := m.Stats()
	if before.LiveReferences != after.LiveReferences {
		t.Fatal("closed-query rejection allocated references")
	}
}

func TestDualityHistoryProposerPreservesCrossManagerSourceParameterKeys(t *testing.T) {
	oldContext, _, _ := dualityNativeSolverFixture(t)
	currentContext, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	makeGraph := func(ctx *Context, suffix string) (*RPFP, *Node, *Expr, *Expr) {
		t.Helper()
		g, err := NewRPFP(ctx)
		must(err)
		x, err := ctx.BoolConst("x@@" + suffix)
		must(err)
		closeExpr(t, x)
		p, err := ctx.BoolConst("p@@" + suffix)
		must(err)
		closeExpr(t, p)
		pd, err := p.Decl()
		must(err)
		must(g.DeclareConstant(pd))
		must(pd.Close())
		boolean, err := x.Sort()
		must(err)
		defer boolean.Close()
		name, err := ctx.StringSymbol("R@@" + suffix)
		must(err)
		decl, err := ctx.Function(name, []*Sort{boolean}, boolean)
		must(err)
		defer decl.Close()
		term, err := decl.Call(x)
		must(err)
		defer term.Close()
		node, err := g.CreateNode(term)
		must(err)
		return g, node, x, p
	}
	old, oldNode, oldX, oldP := makeGraph(oldContext, "old")
	defer old.Close()
	current, currentNode, _, newP := makeGraph(currentContext, "new")
	defer current.Close()
	body, err := oldX.And(oldP)
	must(err)
	closeExpr(t, body)
	fact, err := NewTransformer(oldContext, nil, []*Expr{oldX}, body)
	must(err)
	defer fact.Close()
	must(replaceNodeAnnotation(oldNode, fact))
	proposer, err := NewHistoryProposer(old, current)
	must(err)
	defer proposer.Close()
	derived, err := NewRPFPWithVocabulary(current.vocabulary)
	must(err)
	defer derived.Close()
	node, err := derived.CloneNode(currentNode)
	must(err)
	proposals, err := proposer.Propose(node)
	must(err)
	if len(proposals) != 1 {
		t.Fatal("cross-context history predicate did not match")
	}
	defer proposals[0].Close()
	formula, err := proposals[0].Formula()
	must(err)
	defer formula.Close()
	h, err := formula.Raw()
	must(err)
	first, err := m.GetArg(h, 0)
	must(err)
	second, err := m.GetArg(h, 1)
	must(err)
	decl, err := m.GetDecl(first)
	must(err)
	name, err := m.GetASTName(decl)
	must(err)
	symbol, err := newSymbol(currentContext, name)
	must(err)
	text, err := symbol.String()
	must(err)
	// Source matches using translated parameters but passes original old-manager
	// terms into SubstParams, so cross-manager parameter replacement is a no-op.
	if text != "x@@old" {
		t.Fatal("source original-parameter-key behavior changed", text)
	}
	expected, err := newP.Raw()
	must(err)
	if second != expected {
		t.Fatal("translated background declaration was not remapped")
	}
}
