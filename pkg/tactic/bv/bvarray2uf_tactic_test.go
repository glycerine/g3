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
	// m_produce_proofs is never set in the source imp, so goal::update
	// receives the rewriter's proof of (= old new) as is, without a
	// modus ponens step (AUDIT.md FUZZ_C-F1).
	if n, e := f.m.GetNumArgs(fact); e != nil || n != 2 {
		t.Fatal("rewrite proof fact is not an equation", fact, e)
	}
	lhs, e := f.m.GetArg(fact, 0)
	arrayMust(t, e)
	rhs, e := f.m.GetArg(fact, 1)
	arrayMust(t, e)
	if lhs != equal || rhs != form {
		t.Fatal("rewrite proof fact does not relate the old and new goal", fact, equal, form)
	}
}

// bvarray2uf_rewriter is a rewriter_tpl: ite with a constant condition is
// folded during the walk, fresh functions are mk_fresh_func_decl("f_t", "")
// (f_t!!N), and distinct over arrays is expanded and rewritten again
// (BR_REWRITE1). AUDIT.md FUZZ_C-F1.
func TestBVArray2UFFoldsIteAndNamesFreshFunctions(t *testing.T) {
	f := newArrayFixture(t)
	m := f.m
	boolSort, e := m.MkBoolSort()
	arrayMust(t, e)
	pName, e := m.Symbols.Intern("array-p")
	arrayMust(t, e)
	p, e := m.MkConst(pName, boolSort)
	arrayMust(t, e)
	truth, e := m.MkTrue()
	arrayMust(t, e)
	falsity, e := m.MkFalse()
	arrayMust(t, e)
	ite, e := m.MkIte(truth, falsity, p)
	arrayMust(t, e)
	notITE, e := m.MkNot(ite)
	arrayMust(t, e)
	want, e := m.MkNot(falsity)
	arrayMust(t, e)
	got, _, e := f.tactic.rewriter.rewrite(notITE)
	arrayMust(t, e)
	if got != want {
		t.Fatal("ite with a true condition was not folded", got, want)
	}

	store := f.app(t, ast.ASTArrayOpStore, f.a, f.i, f.v)
	eq, e := m.MkEq(store, f.b)
	arrayMust(t, e)
	_, _, e = f.tactic.rewriter.rewrite(eq)
	arrayMust(t, e)
	decl := arrayDecl(t, f.tactic.rewriter, f.b)
	if decl.IsNull() {
		t.Fatal("no fresh function for array-b")
	}
	name, e := m.GetASTName(decl)
	arrayMust(t, e)
	text, e := m.Symbols.String(name)
	arrayMust(t, e)
	if !strings.HasPrefix(text, "f_t!!") {
		t.Fatal("fresh function name", text)
	}

	distinct, e := m.MkDistinct([]ast.ASTHandle{f.a, f.b})
	arrayMust(t, e)
	got, _, e = f.tactic.rewriter.rewrite(distinct)
	arrayMust(t, e)
	yes, inner, e := m.IsNot(got)
	arrayMust(t, e)
	if !yes {
		t.Fatal("distinct over arrays is not a negation", got)
	}
	k, e := m.GetKind(inner)
	arrayMust(t, e)
	if k != ast.ASTApp {
		t.Fatal("distinct over arrays became a quantifier", inner)
	}
}
