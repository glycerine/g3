package smtcore

import (
	"fmt"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
)

const smtHotPathWidth = 128

// benchmarkMust stays free of b.Helper, whose stack walk would dominate the
// whole-process profiles taken around these benchmarks.
func benchmarkMust(b *testing.B, err error) {
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
func BenchmarkSMTBooleanPropagation(b *testing.B) { benchmarkSMTBooleanPropagation(b, 0) }

// Same chain, with IDs above the shared binary reason page.
func BenchmarkSMTBooleanPropagationLargeIDs(b *testing.B) { benchmarkSMTBooleanPropagation(b, 1024) }

func benchmarkSMTBooleanPropagation(b *testing.B, offset int) {
	m := astTestManager(b)
	_, _, s := smtInternalizerFixture(b, m, nil)
	_, literals := benchmarkSMTBooleans(b, m, s, "bench-bcp", offset+smtHotPathWidth)
	literals = literals[offset:]
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

// Each round marks one batch, delivers it to the theory layer, and pops the
// scope. Each atom remains rooted so every round can use the same handles.
func BenchmarkSMTTheoryRelevanceBatch(b *testing.B) {
	for _, width := range []int{1, 32, 512, 1024} {
		b.Run(fmt.Sprintf("Marks%d", width), func(b *testing.B) {
			m := astTestManager(b)
			_, p, s := smtInternalizerFixture(b, m, nil)
			atoms, _ := benchmarkSMTBooleans(b, m, s, "batch", width)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkMust(b, s.PushScope())
				for _, atom := range atoms {
					benchmarkMust(b, p.MarkRelevant(atom))
				}
				benchmarkMust(b, s.propagateTheoryRelevancy())
				benchmarkMust(b, s.PopScopes(1))
			}
		})
	}
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

/* it is Restoration not Resolution that is super slow. comment this one out to avoid confusion.
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
*/

// BenchmarkSMTScopeRestoration times a search backtrack that preserves and
// rebuilds 64 learned clauses over 128 atoms. SurvivingAtoms exercises cached
// literal reuse; ScopedAtoms also retires and reinternalizes the Boolean vars.
// Each round has an outer user scope so its restored clauses can be discarded
// outside the timer, keeping the workload constant rather than accumulating
// learned clauses. Formula construction, search setup, checks, and reset are
// excluded from timings (but still appear in whole-process CPU profiles).
func BenchmarkSMTScopeRestoration(b *testing.B) {
	benchmarkSMTScopeRestoration(b, false)
}

// BenchmarkSMTScopeRestorationWithQueue runs the native benchmark's default
// delayed activity queue. pop-ns/op uses a monotonic clock around PopScopes
// like the C++ fixture. ns/op and allocation metrics cover the entire round.
func BenchmarkSMTScopeRestorationWithQueue(b *testing.B) {
	benchmarkSMTScopeRestoration(b, true)
}

func benchmarkSMTScopeRestoration(b *testing.B, withQueue bool) {
	for _, scoped := range []bool{false, true} {
		name := "SurvivingAtoms"
		if scoped {
			name = "ScopedAtoms"
		}
		b.Run(name, func(b *testing.B) {
			m := astTestManager(b)
			expressions := make([]ast.ASTHandle, smtHotPathWidth)
			refs := make([]ast.ASTRef, len(expressions))
			b.Cleanup(func() {
				for i := range refs {
					benchmarkMust(b, m.DelRef(&refs[i]))
				}
			})
			for i := range expressions {
				expressions[i] = smtRelevancyBool(b, m, fmt.Sprintf("bench-restore-%d", i))
				var err error
				refs[i], err = m.NewRef(expressions[i])
				benchmarkMust(b, err)
			}
			_, p, s := smtInternalizerFixture(b, m, nil)
			if withQueue {
				benchmarkMust(b, s.ensureDelayedActivityQueue())
			}
			literals := make([]SMTLiteral, len(expressions))
			internalize := func() {
				for i, expression := range expressions {
					benchmarkMust(b, s.Internalize(expression, true))
					benchmarkMust(b, p.MarkRelevant(expression))
					var err error
					literals[i], err = s.Literal(expression)
					benchmarkMust(b, err)
				}
				benchmarkMust(b, p.Propagate())
			}
			const clauses = smtHotPathWidth / 2
			var original [clauses]*SMTClause
			var elapsed time.Duration
			b.ReportAllocs()
			b.ResetTimer()
			if !withQueue {
				b.StopTimer()
			}
			for i := 0; i < b.N; i++ {
				benchmarkMust(b, s.PushScope()) // user scope owns restored clauses
				s.pushBaseScope()               // context::push: m_base_lvl++
				if !scoped {
					internalize()
				}
				benchmarkMust(b, s.beginSearchPersistence(1, SMTParamsState{}))
				benchmarkMust(b, s.PushScope()) // search decision scope
				if scoped {
					internalize()
				}
				madeVars, deletedVars := s.statistics.NumMkBoolVar, s.statistics.NumDelBoolVar
				var wantChanged uint32
				if scoped {
					wantChanged = uint32(len(expressions))
				}
				for j := 0; j < clauses; j++ {
					benchmarkMust(b, s.AddLearnedClauseWithProof([]SMTLiteral{
						literals[2*j], literals[2*j+1].Negated(), literals[(2*j+2)%len(literals)],
					}, ast.ASTHandle{}))
					original[j] = s.lemmas[j].clause
				}
				var err error
				if withQueue {
					start := time.Now()
					err = s.PopScopes(1)
					elapsed += time.Since(start)
				} else {
					b.StartTimer()
					err = s.PopScopes(1)
					b.StopTimer()
				}
				benchmarkMust(b, err)
				if s.scopeLevel != 1 || s.inconsistent || len(s.lemmas) != clauses || s.numClauseRecords() != clauses {
					b.Fatal("incorrect search backtrack", s.scopeLevel, s.inconsistent, len(s.lemmas), s.numClauseRecords())
				}
				for j, expression := range expressions {
					variable, found := s.BoolVar(expression)
					if !found || s.booleanData[variable].InternalLevel() != 1 || p.IsRelevant(expression) == scoped {
						b.Fatal("restoration lost a learned atom or changed source relevance")
					}
					literals[j] = NewSMTLiteral(variable, false)
				}
				for j, record := range s.lemmas {
					if record.clause != original[j] {
						b.Fatal("backtrack replaced a learned clause object")
					}
					got := record.clause.literals
					if len(got) != 3 {
						b.Fatal("backtrack changed clause length", j, got)
					}
					for _, want := range [...]SMTLiteral{literals[2*j], literals[2*j+1].Negated(), literals[(2*j+2)%len(literals)]} {
						if got[0] != want && got[1] != want && got[2] != want {
							b.Fatal("backtrack lost clause literal", j, want, got)
						}
					}
				}
				if s.statistics.NumMkBoolVar-madeVars != wantChanged || s.statistics.NumDelBoolVar-deletedVars != wantChanged {
					b.Fatal("benchmark did not exercise the selected literal restoration path")
				}
				benchmarkMust(b, s.endSearchPersistence())
				benchmarkMust(b, s.PopScopes(1))
				if s.numClauseRecords() != 0 || len(s.booleanTrail()) != 0 {
					b.Fatal("benchmark round left scoped clauses or atoms behind", s.numClauseRecords(), len(s.booleanTrail()))
				}
			}
			if withQueue {
				b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "pop-ns/op")
			}
			b.ReportMetric(clauses, "clauses/op")
			b.ReportMetric(smtHotPathWidth, "atoms/op")
		})
	}
}
