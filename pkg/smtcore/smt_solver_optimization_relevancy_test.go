package smtcore

// Source: src/opt/opt_solver.cpp:49-53,83-88.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func optimizationCheckRelevancy(t *testing.T, s *SMTSolver, want uint32) {
	t.Helper()
	if s.kernel.smtParams.state.RelevancyLvl != want || s.kernel.smtState.RelevancyLvl != want || s.kernel.internalizer.relevancyLevel != want || s.kernel.relevancy.enabled != (want > 0) {
		t.Fatal("optimization relevance state", want, s.kernel.smtParams.state.RelevancyLvl, s.kernel.smtState.RelevancyLvl, s.kernel.internalizer.relevancyLevel, s.kernel.relevancy.enabled)
	}
}

func optimizationTrueQuantifier(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: truth})
	astTestMust(t, err)
	astTestPin(t, m, q)
	return q
}

func TestOptimizationSolverConstructorRelevanceFollowsCaseSplit(t *testing.T) {
	for _, optimize := range []bool{false, true} {
		for strategy := uint32(0); strategy <= 6; strategy++ {
			for input := uint32(0); input <= 2; input++ {
				m, config := smtModelGeneratorFixture(t)
				p, err := config.Context.NewValue()
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, config.Context.Del(&p)) })
				key := "case_split"
				astTestMust(t, config.Context.SetUIntName(&p, &key, strategy))
				key = "relevancy"
				astTestMust(t, config.Context.SetUIntName(&p, &key, input))
				key = "auto_config"
				astTestMust(t, config.Context.SetBoolName(&p, &key, false))
				cfg := SMTSolverConfig{Params: config.Context, ModelEvaluator: config}
				var s *SMTSolver
				if optimize {
					s, err = NewOptimizationSMTSolver(m, &p, "AUFLIA", cfg)
				} else {
					s, err = NewSMTSolver(m, &p, "AUFLIA", cfg)
				}
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, s.Close()) })
				want := input
				if optimize && strategy == 1 {
					want = 0
				}
				optimizationCheckRelevancy(t, s, want)
				key = "relevancy"
				got, err := config.Context.GetUIntName(&p, &key, 99)
				astTestMust(t, err)
				if got != input {
					t.Fatal("constructor mutated caller parameter", got, input)
				}
			}
		}
	}
}

func TestOptimizationSolverQuantifiedAssertionEnablesRelevanceBeforeSimplification(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	optimizationCheckRelevancy(t, s, 0)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "pre-quantified-opt-p"), boolean)
	astTestMust(t, err)
	astTestPin(t, m, p)
	astTestMust(t, s.Assert(p))
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	q := optimizationTrueQuantifier(t, m)
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(q))
	optimizationCheckRelevancy(t, s, 2)
	if !s.kernel.repairRelevancy {
		t.Fatal("already internalized roots lack relevance repair")
	}
	astTestMust(t, s.Assert(p))
	optimizationCheckRelevancy(t, s, 2)
	status, err = s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md, err := s.AcquireModel()
	astTestMust(t, err)
	if md == nil {
		t.Fatal("relevance transition lost model")
	}
	decl, err := m.GetDecl(p)
	astTestMust(t, err)
	v, err := md.ConstantInterpretation(decl)
	astTestMust(t, err)
	if !m.IsTrue(v) {
		t.Fatal("relevance transition dropped asserted root")
	}
	astTestMust(t, md.DecRef())
	astTestMust(t, s.Pop(1))
	optimizationCheckRelevancy(t, s, 2)
}

func TestOptimizationSolverParameterUpdateDoesNotReapplyConstructorRule(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	local, err := s.config.Params.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.config.Params.Del(&local)) }()
	key := "relevancy"
	astTestMust(t, s.config.Params.SetUIntName(&local, &key, 1))
	astTestMust(t, s.UpdateParams(&local))
	optimizationCheckRelevancy(t, s, 1)
	astTestMust(t, s.Assert(optimizationTrueQuantifier(t, m)))
	optimizationCheckRelevancy(t, s, 2)
	// The rule belongs to the concrete optimizer wrapper; ordinary SMT leaves
	// this caller-selected level untouched when the same quantified AST arrives.
	_, ordinary := optimizationSMTFixture(t, false)
	other, err := ordinary.config.Params.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, ordinary.config.Params.Del(&other)) }()
	astTestMust(t, ordinary.config.Params.SetUIntName(&other, &key, 1))
	astTestMust(t, ordinary.UpdateParams(&other))
	astTestMust(t, ordinary.Assert(optimizationTrueQuantifier(t, ordinary.manager)))
	optimizationCheckRelevancy(t, ordinary, 1)
}

func TestOptimizationSolverQuantifiedRelevancePrecedesAssertionRefusal(t *testing.T) {
	failures, success := 0, false
	for after := int64(0); after < 128; after++ {
		m, s := optimizationSMTFixture(t, true)
		q := optimizationTrueQuantifier(t, m)
		m.Budget().FailAfter(after)
		err := s.Assert(q)
		m.Budget().FailAfter(-1)
		optimizationCheckRelevancy(t, s, 2)
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
			failures++
		} else {
			success = true
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("assertion allocation frontier", success, failures)
	}
}
