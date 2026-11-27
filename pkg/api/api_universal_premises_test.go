package api

import (
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

const universalDecls = groundPremiseDecls + `
(declare-sort V 0)(declare-const v V)
(declare-fun r (U U) Bool)(declare-fun t (U V) Bool)`

func TestUniversalPremiseInstances(t *testing.T) {
	for _, tc := range []struct {
		premise, goal string
		proves        bool
	}{
		{"(forall ((x U) (y U)) (r x y))", "(not (r (f a) b))", true},
		{"(forall ((x U) (y V)) (t x y))", "(not (t (f a) v))", true},
		{"(forall ((x U)) (r x x))", "(not (r (f a) (f a)))", true},
		{"(forall ((x U)) (r x x))", "(not (r a b))", false},
		{"(forall ((x U)) (r x a))", "(not (r a b))", false},
		{"(forall ((x U)) (r (f x) x))", "(not (r (f b) b))", true},
		{"(forall ((x U)) (r (f x) x))", "(not (r (f a) b))", false},
		{"(forall ((x U)) (and (p x) (q x)))", "(not (p (f a)))", true},
		{"(forall ((x U)) (or (p x) (q x)))", "(not (p a))", false},
		{"(or (forall ((x U)) (p x)) (q b))", "(not (p a))", false},
		{"(and (forall ((x U)) (p x)) (q b))", "(not (or (p (f a)) (q a)))", true},
		{"(forall ((x U)) (p x))", "(not (and (p a) (q b)))", false},
		{"(forall ((x U)) (not (=> (p x) (q x))))", "(not (p a))", true},
		{"(forall ((x U)) (not (=> (p x) (q x))))", "(q b)", true},
		{"(not (exists ((x U)) (or (p x) (q x))))", "(q b)", true},
		{"(exists ((x U)) (p x))", "(not (p b))", false},
		{"(not (forall ((x U)) (p x)))", "(p b)", false},
		{"(forall ((x U)) (not (q x)))", "(not (=> (q b) (p a)))", true},
		{"(forall ((x U)) (p x))", "(not (=> (q a) (p (f b))))", true},
		{"(forall ((x U)) (p x))", "(not (=> (p a) (q b)))", false},
	} {
		c, s := orderSolver(t, universalDecls+"(assert "+tc.premise+")(assert "+tc.goal+")")
		m, err := c.Manager()
		astTestMust(t, err)
		roots, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		var proof smtcore.UniversalPremises
		got, err := proof.Refutes(m, roots, orderProofSteps, true)
		astTestMust(t, err)
		concrete, err := s.Check()
		astTestMust(t, err)
		if got != tc.proves || concrete == lbool.Undef || (concrete == lbool.False) != tc.proves {
			t.Fatalf("premise %s, goal %s, proved %v, solver %v", tc.premise, tc.goal, got, concrete)
		}
	}
}

func TestUniversalPremiseScopeAndLimits(t *testing.T) {
	const input = universalDecls + `
(assert (forall ((x U)) (p x)))(assert (forall ((x U)) (q x)))
(assert (not (p (f a))))`
	var proof smtcore.UniversalPremises
	for range 2 {
		c, s := orderSolver(t, input)
		m, err := c.Manager()
		astTestMust(t, err)
		roots, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		check := func(base int, steps uint32, early, want bool) {
			t.Helper()
			got, err := proof.Refutes(m, []ast.ASTHandle{roots[base], roots[2]}, steps, early)
			if err != nil || got != want {
				t.Fatalf("base %d, steps %d, before push %v, got %v, %v", base, steps, early, got, err)
			}
		}
		check(0, 1, true, false)
		m.ResourceLimit().IncCancel()
		check(0, orderProofSteps, true, false)
		m.ResourceLimit().DecCancel()
		check(0, orderProofSteps, false, false)
		check(0, orderProofSteps, false, true)
		check(1, orderProofSteps, true, false)
		check(0, orderProofSteps, true, true)
	}
}

func TestUniversalPremiseRequiresWellSortedBindings(t *testing.T) {
	c, s := orderSolver(t, universalDecls+`
(assert (forall ((x U)) (p x)))(assert (forall ((y V)) (t a y)))
(assert (not (p a)))`)
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		astTestMust(t, m.IncRef(h))
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		return h
	}
	u, err := m.GetDeclarationSort(roots[0], 0)
	astTestMust(t, err)
	v, err := m.GetDeclarationSort(roots[1], 0)
	astTestMust(t, err)
	body, err := m.GetQuantifierBody(roots[0])
	astTestMust(t, err)
	header, err := m.NodeHeader(body)
	astTestMust(t, err)
	free := pin(m.MkVar(1, u))
	open := pin(m.MkApp(header.Decl, []ast.ASTHandle{free}))
	for _, tc := range []struct{ sort, body ast.ASTHandle }{{v, body}, {u, open}} {
		q := pin(m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{tc.sort},
			Names: []symbol.Symbol{symbol.Numerical(0)}, Body: tc.body,
		}))
		var proof smtcore.UniversalPremises
		input := []ast.ASTHandle{q, roots[2]}
		ok, err := proof.Refutes(m, input, orderProofSteps, true)
		if err != nil || ok {
			t.Fatalf("malformed binder proved %v, %v", ok, err)
		}
		checkObservedProof(t, c, s, input, false)
	}
}

func TestUniversalPremiseDepthLimit(t *testing.T) {
	chain := func(term string) string { return strings.Repeat("(f ", 70) + term + strings.Repeat(")", 70) }
	c, s := orderSolver(t, universalDecls+"(assert (forall ((x U)) (p "+chain("x")+")))"+
		"(assert (not (p "+chain("a")+")))")
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	var proof smtcore.UniversalPremises
	got, err := proof.Refutes(m, roots, orderProofSteps, true)
	if err != nil || got {
		t.Fatalf("exhausted structural proof returned %v, %v", got, err)
	}
}

func TestUniversalPremiseTemporaryGoal(t *testing.T) {
	const input = universalDecls + `
(assert (forall ((x U)) (r (f x) x)))
(assert (not (r (f a) a)))(assert (not (r (f a) b)))`
	c, donor := orderSolver(t, input)
	roots, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	s, err := c.NewSolverForIncrementalChecks()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	astTestMust(t, s.Assert(roots[0]))
	for _, tc := range []struct {
		goal ast.ASTHandle
		want bool
	}{{roots[1], true}, {roots[2], false}} {
		got, err := batch.ProveWithGoal(tc.goal)
		astTestMust(t, err)
		active, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		level, err := s.ScopeLevel()
		astTestMust(t, err)
		if got != tc.want || level != 0 || !slices.Equal(active, roots[:1]) {
			t.Fatalf("temporary goal proof %v, scope %d, assertions %v", got, level, active)
		}
	}
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(roots[2]))
	answer, err := s.Check()
	astTestMust(t, err)
	if answer != lbool.True {
		t.Fatal("counterexample", answer)
	}
	model, err := s.Model()
	astTestMust(t, err)
	astTestMust(t, model.Close())
	astTestMust(t, s.Pop(1))
}

func BenchmarkUniversalPremiseSiblings(b *testing.B) {
	const input = universalDecls + `
(assert (forall ((x U)) (r (f x) x)))
(assert (not (r (f a) a)))(assert (not (r (f a) b)))`
	m, roots := benchmarkOrderAssertions(b, input, (*SMTContext).NewSolver)
	assertions := slices.Clone(roots[:2])
	var proof smtcore.UniversalPremises
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assertions[1] = roots[1+i%2]
		got, err := proof.Refutes(m, assertions, orderProofSteps, true)
		if err != nil || got != (i%2 == 0) {
			b.Fatal(i, got, err)
		}
	}
}
