package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArithmeticEqualityAdapterEventsAndScopes(t *testing.T) {
	f := newSMTIntegerDiffFixture(t)
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	left := f.state.context.enodes.get(f.x).TheoryVar(f.family)
	right := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.state.enqueueTheoryEquality(f.family, left, right))
	astTestMust(t, f.state.PropagateTheoryEvents())
	if f.theory.equalityAdapter.NumAxioms() != 1 || len(f.theory.atoms) != 2 || len(f.state.Clauses()) != 3 {
		t.Fatal("integer equality event adapter", f.theory.equalityAdapter.NumAxioms(), len(f.theory.atoms), len(f.state.Clauses()))
	}
	astTestMust(t, f.theory.NewDisequality(left, right))
	if f.theory.equalityAdapter.NumAxioms() != 1 {
		t.Fatal("duplicate pair was processed twice")
	}
	astTestMust(t, f.state.PopScopes(1))
	if len(f.theory.atoms) != 0 || len(f.state.Clauses()) != 0 {
		t.Fatal("adapter products survived scope rollback", len(f.theory.atoms), len(f.state.Clauses()))
	}
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.theory.NewDisequality(left, right))
	if f.theory.equalityAdapter.NumAxioms() != 2 || len(f.theory.atoms) != 2 {
		t.Fatal("rolled-back pair was not recreated", f.theory.equalityAdapter.NumAxioms(), len(f.theory.atoms))
	}
}

func TestSMTRealDifferenceEqualityEventAdapter(t *testing.T) {
	f := newSMTRealDiffFixture(t)
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	left := f.state.context.enodes.get(f.x).TheoryVar(f.family)
	right := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	astTestMust(t, f.state.enqueueTheoryDisequality(f.family, left, right))
	astTestMust(t, f.state.PropagateTheoryEvents())
	// The adapter's three clauses; without m_arith_add_binary_bounds the
	// atoms get no bound axioms.
	if f.theory.equalityAdapter.NumAxioms() != 1 || len(f.theory.realAtoms) != 2 || len(f.state.Clauses()) != 3 {
		t.Fatal("real disequality event adapter", f.theory.equalityAdapter.NumAxioms(), len(f.theory.realAtoms), len(f.state.Clauses()))
	}
}

func TestSMTArithmeticEqualityAdapterRecreatesSearchAxiomsAtRestart(t *testing.T) {
	f := newSMTIntegerDiffFixture(t)
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	left := f.state.context.enodes.get(f.x).TheoryVar(f.family)
	right := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	params := SMTParamsState{AgilityFactor: 0.9999, RestartAdaptive: true}
	astTestMust(t, f.state.InitializeTheorySearch())
	astTestMust(t, f.state.beginSearchPersistence(0, params))
	defer func() { astTestMust(t, f.state.endSearchPersistence()) }()
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.state.enqueueTheoryEquality(f.family, left, right))
	astTestMust(t, f.state.PropagateTheoryEvents())
	if len(f.theory.equalityAdapter.restart) != 1 || len(f.state.Clauses()) != 3 {
		t.Fatal("search equality was not queued for restart", len(f.theory.equalityAdapter.restart), len(f.state.Clauses()))
	}
	astTestMust(t, f.state.PopScopes(1))
	if len(f.state.Clauses()) != 0 {
		t.Fatal("search axioms survived decision backtracking", len(f.state.Clauses()))
	}
	astTestMust(t, f.state.RestartTheories())
	if len(f.theory.equalityAdapter.restart) != 0 || f.theory.equalityAdapter.NumAxioms() != 2 || len(f.state.Clauses()) != 3 {
		t.Fatal("restart did not recreate equality axioms", len(f.theory.equalityAdapter.restart), f.theory.equalityAdapter.NumAxioms(), len(f.state.Clauses()))
	}
}

func TestSMTArithmeticEqualityAdapterBinaryBoundsAndLazyRestart(t *testing.T) {
	f := newSMTIntegerDiffFixture(t)
	astTestMust(t, f.theory.ConfigureArithmetic(SMTArithState{AddBinaryBounds: true, LazyAdapter: true}))
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	left := f.state.context.enodes.get(f.x).TheoryVar(f.family)
	right := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	params := SMTParamsState{AgilityFactor: 0.9999, RestartAdaptive: true}
	astTestMust(t, f.state.beginSearchPersistence(0, params))
	t.Cleanup(func() { astTestMust(t, f.state.endSearchPersistence()) })
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.theory.NewEquality(left, right))
	if len(f.state.Clauses()) != 5 {
		t.Fatal("binary-bound option did not add the source fourth axiom", len(f.state.Clauses()))
	}
	if len(f.theory.equalityAdapter.restart) != 0 {
		t.Fatal("lazy adapter queued eager restart recreation", f.theory.equalityAdapter.restart)
	}
	last := f.state.Clauses()[len(f.state.Clauses())-1]
	if len(last.Literals) != 2 || last.Literals[0].Sign() || last.Literals[1].Sign() {
		t.Fatal("binary-bound axiom is not lower-or-upper", last.Literals)
	}
}

func TestSMTArithmeticEqualityAdapterDefersGeneratedRelevancy(t *testing.T) {
	f := newSMTIntegerDiffFixture(t)
	// mk_axioms registers arith_eq_relevancy_eh only under ctx.relevancy().
	f.state.relevancyLevel = 2
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	left := f.state.context.enodes.get(f.x).TheoryVar(f.family)
	right := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	astTestMust(t, f.theory.NewEquality(left, right))
	targets := append([]ast.ASTHandle(nil), f.state.boolExpr[SMTFirstBoolVar:]...)
	if len(targets) != 3 {
		t.Fatal("generated equality/bound atom count", len(targets))
	}
	for _, target := range targets {
		if f.state.relevancy.IsRelevant(target) {
			t.Fatal("generated adapter atom became relevant before both terms", target)
		}
	}
	astTestMust(t, f.state.relevancy.MarkRelevant(f.x))
	astTestMust(t, f.state.relevancy.Propagate())
	for _, target := range targets {
		if f.state.relevancy.IsRelevant(target) {
			t.Fatal("one relevant term activated adapter atom", target)
		}
	}
	astTestMust(t, f.state.relevancy.MarkRelevant(f.y))
	astTestMust(t, f.state.relevancy.Propagate())
	for _, target := range targets {
		if !f.state.relevancy.IsRelevant(target) {
			t.Fatal("two relevant terms did not activate adapter atom", target)
		}
	}

	for variable := int32(SMTFirstBoolVar); int(variable) < len(f.state.boolExpr); variable++ {
		value, err := f.state.Assignment(NewSMTLiteral(variable, false))
		astTestMust(t, err)
		if value != lbool.Undef {
			t.Fatal("relevancy dependency assigned generated atom", variable, value)
		}
	}
}

func TestSMTKernelAppliesArithmeticConfigurationOnTheoryRegistration(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	kernel, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	kernel.arithState = SMTArithState{AddBinaryBounds: true, LazyAdapter: true}
	theory, err := NewSMTIntegerDiffTheory(family)
	astTestMust(t, err)
	astTestMust(t, kernel.RegisterTheory(theory))
	if !theory.addBinaryBounds || !theory.equalityAdapter.lazy {
		t.Fatal("kernel did not apply arithmetic parameter snapshot")
	}
}
