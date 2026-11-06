package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

const smtHotPathWidth = 128

func benchmarkMust(b *testing.B, err error) {
	b.Helper()
	if err != nil {
		b.Fatal(err)
	}
}

func benchmarkSMTBooleans(b *testing.B, m *ast.ASTManager, s *SMTInternalizerState, prefix string, count int) ([]ast.ASTHandle, []SMTLiteral) {
	b.Helper()
	expressions := make([]ast.ASTHandle, count)
	literals := make([]SMTLiteral, count)
	for i := range expressions {
		expressions[i] = smtRelevancyBool(b, m, fmt.Sprintf("%s-%d", prefix, i))
		benchmarkMust(b, s.Internalize(expressions[i], true))
		variable, found := s.BoolVar(expressions[i])
		if !found {
			b.Fatal("internalized Boolean has no variable")
		}
		literals[i] = NewSMTLiteral(variable, false)
	}
	return expressions, literals
}

// BenchmarkSMTBooleanPropagation times watched-clause BCP only. Each round
// propagates a 128-variable implication chain; scope setup and rollback are
// outside the timed region.
func BenchmarkSMTBooleanPropagation(b *testing.B) {
	m := astTestManager(b)
	_, _, s := smtInternalizerFixture(b, m, nil)
	_, literals := benchmarkSMTBooleans(b, m, s, "bench-bcp", smtHotPathWidth)
	for i := 0; i+1 < len(literals); i++ {
		benchmarkMust(b, s.addGateClause([]SMTLiteral{literals[i].Negated(), literals[i+1]}))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		benchmarkMust(b, s.PushScope())
		benchmarkMust(b, s.AssignLiteral(literals[0], NewSMTBooleanAxiom()))
		b.StartTimer()
		consistent, err := s.Propagate()
		if err != nil || !consistent {
			b.Fatalf("Boolean propagation: consistent=%v err=%v", consistent, err)
		}
		b.StopTimer()
		benchmarkMust(b, s.PopScopes(1))
		b.StartTimer()
	}
	b.ReportMetric(smtHotPathWidth-1, "implications/op")
}

// BenchmarkSMTRelevanceCallbacks times delivery of a completed assignment
// batch to relevance watches. Assignments, batch creation, and scope restore
// are excluded so the result isolates callback dispatch and handlers.
func BenchmarkSMTRelevanceCallbacks(b *testing.B) {
	m := astTestManager(b)
	_, p, s := smtInternalizerFixture(b, m, nil)
	sources, _ := benchmarkSMTBooleans(b, m, s, "bench-relevance-source", smtHotPathWidth)
	targets, _ := benchmarkSMTBooleans(b, m, s, "bench-relevance-target", smtHotPathWidth)
	for i := range sources {
		benchmarkMust(b, p.AddWatchTarget(sources[i], true, targets[i]))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		benchmarkMust(b, s.PushScope())
		start := len(s.assignedLiterals)
		for _, expression := range sources {
			literal, err := s.Literal(expression)
			benchmarkMust(b, err)
			benchmarkMust(b, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		}
		s.relevancyBatch = smtRelevancyAssignmentBatch{
			head: start, end: len(s.assignedLiterals), active: true, caseSplitsDone: true,
		}
		b.StartTimer()
		if err := s.propagateRelevancyAssignments(); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		s.relevancyBatch = smtRelevancyAssignmentBatch{}
		benchmarkMust(b, s.PopScopes(1))
		b.StartTimer()
	}
	b.ReportMetric(smtHotPathWidth, "callbacks/op")
}

// BenchmarkSMTConflictResolution repeatedly resolves the same two-level
// first-UIP conflict, matching the structure of the conflict resolver tests.
func BenchmarkSMTConflictResolution(b *testing.B) {
	m := astTestManager(b)
	_, _, s := smtInternalizerFixture(b, m, nil)
	_, literals := benchmarkSMTBooleans(b, m, s, "bench-conflict", 3)
	benchmarkMust(b, s.PushScope())
	benchmarkMust(b, s.AssignLiteral(literals[0], NewSMTBooleanAxiom()))
	benchmarkMust(b, s.PushScope())
	benchmarkMust(b, s.AssignLiteral(literals[1], NewSMTBooleanAxiom()))
	reason, err := NewSMTClause(m, []SMTLiteral{literals[2], literals[0].Negated()}, SMTClauseLearned, nil, nil, false, nil)
	benchmarkMust(b, err)
	benchmarkMust(b, s.AssignLiteral(literals[2], NewSMTBooleanClause(reason)))
	conflict, err := NewSMTClause(m, []SMTLiteral{literals[2].Negated(), literals[1].Negated()}, SMTClauseAux, nil, nil, false, nil)
	benchmarkMust(b, err)
	resolver, err := NewSMTConflictResolver(s)
	benchmarkMust(b, err)
	b.Cleanup(func() {
		benchmarkMust(b, resolver.Close())
		benchmarkMust(b, conflict.Close())
		benchmarkMust(b, reason.Close())
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolution, err := resolver.Resolve(NewSMTBooleanClause(conflict), SMTNullLiteral, 0)
		if err != nil {
			b.Fatal(err)
		}
		if !resolution.Resolved {
			b.Fatal("benchmark conflict was not resolved")
		}
	}
}

// BenchmarkSMTScopeResolution times rollback of a scope holding 128 Boolean
// assignments. Scope construction and assignment publication are excluded.
func BenchmarkSMTScopeResolution(b *testing.B) {
	m := astTestManager(b)
	_, _, s := smtInternalizerFixture(b, m, nil)
	_, literals := benchmarkSMTBooleans(b, m, s, "bench-scope", smtHotPathWidth)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		benchmarkMust(b, s.PushScope())
		for _, literal := range literals {
			benchmarkMust(b, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		}
		b.StartTimer()
		benchmarkMust(b, s.PopScopes(1))
		b.StopTimer()
		b.StartTimer()
	}
	b.ReportMetric(smtHotPathWidth, "assignments/op")
}
