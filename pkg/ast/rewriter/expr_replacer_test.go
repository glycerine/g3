package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDefaultExprReplacerNestedSubstitutionAndScopedApply(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "replace-a")
	b := goalTestAtom(t, m, "replace-b")
	f := rewriterTestDecl(t, m, "replace-f")
	term := rewriterTestApp(t, m, f, a)
	want := rewriterTestApp(t, m, f, b)

	s, err := m.NewExprSubstitution()
	satOK(t, err)
	t.Cleanup(func() { satOK(t, m.DelExprSubstitution(&s)) })
	satOK(t, m.ExprSubstitutionInsert(s, a, b, ast.ASTHandle{}, ast.ASTDependency{}))
	r, err := NewDefaultExprReplacer(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	satOK(t, r.SetSubstitution(&s))
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, r.Rewrite(term, out))
	got, err := m.RefGet(out)
	satOK(t, err)
	if got != want {
		t.Fatal("nested substitution", got, want)
	}
	steps, err := r.NumSteps()
	satOK(t, err)
	if steps == 0 {
		t.Fatal("source traversal did not count steps")
	}

	satOK(t, r.SetSubstitution(nil))
	satOK(t, m.SetRef(out, term))
	satOK(t, r.ApplySubstitution(a, b, ast.ASTHandle{}, out))
	got, err = m.RefGet(out)
	satOK(t, err)
	if got != want {
		t.Fatal("scoped substitution", got, want)
	}
	// scoped_set_subst clears the substitution after the call.
	satOK(t, m.SetRef(out, term))
	satOK(t, r.Rewrite(term, out))
	got, err = m.RefGet(out)
	satOK(t, err)
	if got != term {
		t.Fatal("scoped substitution remained installed", got, term)
	}
}

func TestDefaultExprReplacerProofOverload(t *testing.T) {
	m := astTestManager(t)
	satOK(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a, b := goalTestAtom(t, m, "replace-proof-a"), goalTestAtom(t, m, "replace-proof-b")
	f := rewriterTestDecl(t, m, "replace-proof-f")
	term := rewriterTestApp(t, m, f, a)
	want := rewriterTestApp(t, m, f, b)
	step, err := m.MkProofRewrite(a, b)
	satOK(t, err)
	s := ast.ASTExprSubstitution{}
	s, err = m.NewExprSubstitution()
	satOK(t, err)
	t.Cleanup(func() { satOK(t, m.DelExprSubstitution(&s)) })
	satOK(t, m.ExprSubstitutionInsert(s, a, b, step, ast.ASTDependency{}))
	r, err := NewDefaultExprReplacer(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	satOK(t, r.SetSubstitution(&s))
	out := astTestPin(t, m, ast.ASTHandle{})
	proof := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, r.RewriteWithProof(term, out, proof))
	got, err := m.RefGet(out)
	satOK(t, err)
	if got != want {
		t.Fatal("proof replacer result", got, want)
	}
	p, err := m.RefGet(proof)
	satOK(t, err)
	fact, err := m.GetProofFact(p)
	satOK(t, err)
	wantFact, err := m.MkIff(term, want)
	satOK(t, err)
	if fact != wantFact {
		t.Fatal("proof replacer fact", fact, wantFact)
	}
}

func TestDefaultExprReplacerCollectsUsedDependencies(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "dependency-a")
	b := goalTestAtom(t, m, "dependency-b")
	x := goalTestAtom(t, m, "dependency-x")
	y := goalTestAtom(t, m, "dependency-y")
	and, err := m.MkAnd(a, b)
	satOK(t, err)
	astTestPin(t, m, and)
	want, err := m.MkAnd(x, y)
	satOK(t, err)
	astTestPin(t, m, want)

	dx, err := m.DependencyLeaf(x)
	satOK(t, err)
	dy, err := m.DependencyLeaf(y)
	satOK(t, err)
	satOK(t, m.DependencyInc(dx))
	satOK(t, m.DependencyInc(dy))
	t.Cleanup(func() { satOK(t, m.DependencyDec(dx)); satOK(t, m.DependencyDec(dy)) })
	s, err := m.NewExprSubstitutionPolicy(true, false)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, m.DelExprSubstitution(&s)) })
	satOK(t, m.ExprSubstitutionInsert(s, a, x, ast.ASTHandle{}, dx))
	satOK(t, m.ExprSubstitutionInsert(s, b, y, ast.ASTHandle{}, dy))

	r, err := NewDefaultExprReplacer(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	satOK(t, r.SetSubstitution(&s))
	out := astTestPin(t, m, ast.ASTHandle{})
	used, err := r.RewriteWithDependency(and, out)
	satOK(t, err)
	if used.IsNull() {
		t.Fatal("missing accumulated dependency")
	}
	defer func() { satOK(t, m.DependencyDec(used)) }()
	got, err := m.RefGet(out)
	satOK(t, err)
	if got != want {
		t.Fatal("dependency rewrite", got, want)
	}
	leaves, err := m.DependencyLinearize(used, nil)
	satOK(t, err)
	if len(leaves) != 2 || leaves[0] != x || leaves[1] != y {
		t.Fatal("source dependency join order", leaves)
	}
}

func TestSimplifyingExprReplacerTheoryRewriteAndDependencyTransfer(t *testing.T) {
	m, context, globals, params := boolRewriterConfigFixture(t)
	a := goalTestAtom(t, m, "simplifying-replacer-a")
	b := goalTestAtom(t, m, "simplifying-replacer-b")
	truth, err := m.MkTrue()
	satOK(t, err)
	term, err := m.MkAnd(a, truth)
	satOK(t, err)
	astTestPin(t, m, term)

	dependency, err := m.DependencyLeaf(b)
	satOK(t, err)
	satOK(t, m.DependencyInc(dependency))
	t.Cleanup(func() { satOK(t, m.DependencyDec(dependency)) })
	substitution, err := m.NewExprSubstitutionPolicy(true, false)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, m.DelExprSubstitution(&substitution)) })
	satOK(t, m.ExprSubstitutionInsert(substitution, a, b, ast.ASTHandle{}, dependency))

	r, err := NewSimplifyingExprReplacer(m, context, globals, params)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	if r.Manager() != m {
		t.Fatal("simplifying replacer manager")
	}
	satOK(t, r.SetSubstitution(&substitution))
	out := astTestPin(t, m, term)
	used, err := r.RewriteWithDependency(term, out)
	satOK(t, err)
	if used.IsNull() {
		t.Fatal("simplifying replacer omitted dependency")
	}
	defer func() { satOK(t, m.DependencyDec(used)) }()
	got, err := m.RefGet(out)
	satOK(t, err)
	if got != b {
		t.Fatal("substitution was not theory simplified", got, b)
	}
	contains, err := m.DependencyContains(used, b)
	satOK(t, err)
	if !contains {
		t.Fatal("transferred dependency omitted replacement")
	}
	if steps, e := r.NumSteps(); e != nil || steps == 0 {
		t.Fatal("simplifying replacer steps", steps, e)
	}
}

// AUDIT ast_rewriter_03-F7: every C++ operator() overload funnels into the
// four-argument one, which hands the used dependencies to a local
// expr_dependency_ref and resets the cache. A plain Rewrite must not leave
// them for the next RewriteWithDependency.
func TestDefaultExprReplacerPlainRewriteResetsDependencies(t *testing.T) {
	m := astTestManager(t)
	a := goalTestAtom(t, m, "plain-dependency-a")
	b := goalTestAtom(t, m, "plain-dependency-b")
	x := goalTestAtom(t, m, "plain-dependency-x")
	dx, err := m.DependencyLeaf(x)
	satOK(t, err)
	satOK(t, m.DependencyInc(dx))
	t.Cleanup(func() { satOK(t, m.DependencyDec(dx)) })
	s, err := m.NewExprSubstitutionPolicy(true, false)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, m.DelExprSubstitution(&s)) })
	satOK(t, m.ExprSubstitutionInsert(s, a, x, ast.ASTHandle{}, dx))

	r, err := NewDefaultExprReplacer(m)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	satOK(t, r.SetSubstitution(&s))
	out := astTestPin(t, m, ast.ASTHandle{})
	satOK(t, r.Rewrite(a, out))
	if got, err := m.RefGet(out); err != nil || got != x {
		t.Fatal("plain rewrite", got, x, err)
	}
	used, err := r.RewriteWithDependency(b, out)
	satOK(t, err)
	if !used.IsNull() {
		satOK(t, m.DependencyDec(used))
		t.Fatal("plain Rewrite left its dependency to the next call")
	}
}
