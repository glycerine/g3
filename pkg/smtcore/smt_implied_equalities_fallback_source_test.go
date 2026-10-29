package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSMTImpliedEqualitiesMissingEvaluatorUsesExactFallback(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: config.Context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "fallback-p"), boolean)
	astTestMust(t, err)
	q, err := m.MkConst(astTestSymbol(t, m, "fallback-q"), boolean)
	astTestMust(t, err)
	equal, err := m.MkIff(p, q)
	astTestMust(t, err)
	astTestMust(t, s.Assert(equal))
	status, classes, stats, err := s.ImpliedEqualitiesWithStats([]ast.ASTHandle{p, q})
	astTestMust(t, err)
	if status != lbool.True || classes[0] != classes[1] || stats.FallbackTerms != 2 || stats.PairChecks != 1 || stats.ValueEvaluations != 0 {
		t.Fatal("exact fallback", status, classes, stats)
	}
}
