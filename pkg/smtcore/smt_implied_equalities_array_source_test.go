package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSMTImpliedEqualitiesArraysUseExactExtensionalFallback(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "array-eq-a"), arraySort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "array-eq-b"), arraySort)
	astTestMust(t, err)
	equal, err := m.MkEq(a, b)
	astTestMust(t, err)
	s, err := NewSMTSolver(m, nil, "QF_AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.Assert(equal))
	status, classes, stats, err := s.ImpliedEqualitiesWithStats([]ast.ASTHandle{a, b})
	astTestMust(t, err)
	if status != lbool.True || classes[0] != classes[1] || stats.FallbackTerms != 2 || stats.PairChecks != 1 || stats.ValueEvaluations != 0 {
		t.Fatal("array fallback", status, classes, stats)
	}
}
