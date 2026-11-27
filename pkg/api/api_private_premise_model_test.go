package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPrivatePremiseModels(t *testing.T) {
	const decls = "(declare-sort U 0)(declare-const a U)(declare-const b U)(declare-fun p (U) Bool)(declare-fun q (U) Bool)"
	for _, tc := range []struct {
		name, assertions string
		model            bool
		answer           lbool.LBool
	}{
		{"nested binders", "(assert (forall ((x U)) (= (p x) (ite (= x a) (exists ((y U)) (q y)) false))))(assert (not (p b)))", true, lbool.True},
		{"contradiction", "(assert (forall ((x U)) (= (p x) (q x))))(assert (p a))(assert (not (q a)))", false, lbool.False},
		{"requires two elements", "(assert (not (= a b)))", false, lbool.True},
		{"interpreted equality", "(assert (= 1 2))", false, lbool.False},
		{"opposite Boolean values", "(assert (not (= (p a) (q b))))", true, lbool.True},
		{"bound Boolean variable", "(assert (forall ((x Bool)) x))", false, lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, s := orderSolver(t, decls+tc.assertions)
			m, err := ctx.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			if got := hasPrivatePremiseModel(m, roots); got != tc.model {
				t.Fatalf("model = %v, want %v", got, tc.model)
			}
			got, err := s.Check()
			astTestMust(t, err)
			if got != tc.answer {
				t.Fatalf("solver = %v, want %v", got, tc.answer)
			}
		})
	}
}

func TestPrivatePremiseModelDistinguishedValues(t *testing.T) {
	ctx, s := orderSolver(t, "(declare-sort U 0)(declare-const a U)(declare-fun p (U) Bool)(assert (p a))")
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	a, err := m.GetArg(roots[0], 0)
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkModelValue(0, sort)
	astTestMust(t, err)
	y, err := m.MkModelValue(1, sort)
	astTestMust(t, err)
	eq, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(eq))
	defer func() { astTestMust(t, m.DecRef(eq)) }()
	if hasPrivatePremiseModel(m, []ast.ASTHandle{eq}) {
		t.Fatal("accepted equality of distinct model values")
	}
}

const privateSubsetInput = "(declare-sort U 0)(declare-const a U)(declare-fun p (U) Bool)(assert (forall ((x U)) (p x)))(assert (not (p a)))"

func TestPrivatePremiseModelFallsThroughToFullProof(t *testing.T) {
	ctx, s := orderSolver(t, privateSubsetInput)
	m, err := ctx.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	p := orderProofState{source: s}
	p.plans.enabled = true
	defer func() { astTestMust(t, p.plans.close()) }()
	proved, err := p.checkProofPlan(m, &shortPremisePlan{source: roots, selected: roots[:1]}, 6000)
	astTestMust(t, err)
	if proved || (p.plans.reusable(m) && p.plans.aux != nil) {
		t.Fatal("satisfiable subset should decline the private proof")
	}
	answer, err := s.Check()
	astTestMust(t, err)
	if answer != lbool.False {
		t.Fatalf("full solver returned %v", answer)
	}
}

func BenchmarkPrivateSatisfiablePremisePlan(b *testing.B) {
	ctx, s := orderSolver(b, privateSubsetInput)
	m, err := ctx.Manager()
	astTestMust(b, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(b, err)
	p := orderProofState{source: s}
	p.plans.enabled = true
	b.Cleanup(func() { astTestMust(b, p.plans.close()) })
	plan := shortPremisePlan{source: roots, selected: roots[:1]}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proved, err := p.checkProofPlan(m, &plan, 6000)
		if proved || err != nil {
			b.Fatalf("private check = %v, %v", proved, err)
		}
	}
}
