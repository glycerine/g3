package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"
)

func TestBVArray2UFTacticPublishesSideConstraintsAndDepth(t *testing.T) {
	f := newArrayFixture(t)
	selectTerm := f.app(t, ast.ASTArrayOpSelect, f.app(t, ast.ASTArrayOpStore, f.a, f.i, f.v), f.j)
	g := f.goal(t, false, false, false, selectTerm)
	var out tactic.TacticResult
	t.Cleanup(func() { arrayMust(t, out.Close()) })
	arrayMust(t, f.tactic.Apply(g, &out))
	if len(out.Goals) != 1 {
		t.Fatal(len(out.Goals))
	}
	n, e := g.Size()
	arrayMust(t, e)
	if n < 3 {
		t.Fatal("missing store side constraints", n)
	}
	depth, e := g.Depth()
	arrayMust(t, e)
	if depth != 1 {
		t.Fatal(depth)
	}
}
func TestBVArray2UFRejectsCoresAndQuantifiers(t *testing.T) {
	f := newArrayFixture(t)
	selectTerm := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	var out tactic.TacticResult
	err := f.tactic.Apply(f.goal(t, false, false, true, selectTerm), &out)
	if err == nil || !strings.Contains(err.Error(), "unsat core") {
		t.Fatal(err)
	}
	x, e := f.m.MkVar(0, f.index)
	arrayMust(t, e)
	body, e := f.m.MkEq(x, x)
	arrayMust(t, e)
	name, e := f.m.Symbols.Intern("qx")
	arrayMust(t, e)
	q, e := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.index}, Names: []usymbol.Symbol{name}, Body: body})
	arrayMust(t, e)
	err = f.tactic.Apply(f.goal(t, false, false, false, q), &out)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatal(err)
	}
}

func TestBVArray2UFComposesRewriteProof(t *testing.T) {
	f := newArrayFixtureProof(t, true)
	selected := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	equal, e := f.m.MkEq(selected, f.v)
	arrayMust(t, e)
	g := f.goal(t, true, false, false, equal)
	var out tactic.TacticResult
	t.Cleanup(func() { arrayMust(t, out.Close()) })
	arrayMust(t, f.tactic.Apply(g, &out))
	pr, e := g.Proof(0)
	arrayMust(t, e)
	if pr.IsNull() {
		t.Fatal("rewritten formula lost its proof")
	}
	fact, e := f.m.GetProofFact(pr)
	arrayMust(t, e)
	form, e := g.Form(0)
	arrayMust(t, e)
	if fact != form {
		t.Fatal("rewrite proof fact does not match rewritten goal", fact, form)
	}
}
