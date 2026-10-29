package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSMTImpliedEqualitiesModelBucketsReduceExactProbes(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "bucket-p"), boolean)
	astTestMust(t, err)
	q, err := m.MkConst(astTestSymbol(t, m, "bucket-q"), boolean)
	astTestMust(t, err)
	r, err := m.MkConst(astTestSymbol(t, m, "bucket-r"), boolean)
	astTestMust(t, err)
	iff, err := m.MkIff(p, q)
	astTestMust(t, err)
	notR, err := m.MkNot(r)
	astTestMust(t, err)
	for _, assertion := range []ast.ASTHandle{p, iff, notR} {
		astTestMust(t, s.Assert(assertion))
	}
	status, classes, stats, err := s.ImpliedEqualitiesWithStats([]ast.ASTHandle{p, q, r})
	astTestMust(t, err)
	if status != lbool.True || classes[0] != classes[1] || classes[2] == classes[0] {
		t.Fatal("model-bucket classes", status, classes)
	}
	if stats.ValueEvaluations != 3 || stats.ValueBuckets != 2 || stats.PairChecks != 1 || stats.FallbackTerms != 0 {
		t.Fatal("model-bucket work", stats)
	}
}
