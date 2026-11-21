package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Time the assignment callback for an asserted disjunction whose one true
// child is already relevant. Apple rfn2 takes this path millions of times.
func BenchmarkSMTAssertedOrWitness(b *testing.B) {
	for _, width := range []int{4, 16, 64} {
		for _, index := range []int{0, width - 1} {
			b.Run(fmt.Sprintf("Width%d/Child%d", width, index), func(b *testing.B) {
				m := astTestManager(b)
				_, p, state := smtInternalizerFixture(b, m, nil)
				atoms, literals := benchmarkSMTBooleans(b, m, state, "root-witness", width)
				children := append([]ast.ASTHandle(nil), atoms...)
				for i := 1; i < width; i += 2 {
					child, err := m.MkNot(atoms[i])
					benchmarkMust(b, err)
					children[i] = child
				}
				root, err := m.MkOrArray(children)
				benchmarkMust(b, err)
				benchmarkMust(b, state.Assert(root, ast.ASTHandle{}, 0))
				if _, found := state.BoolVar(root); found || !p.IsRelevant(root) || p.IsRelevant(children[index]) {
					b.Fatal("expected a relevant root with unmarked children and no Boolean variable")
				}
				value := index%2 == 0
				variable := literals[index].Var()
				benchmarkMust(b, state.SetAssignment(variable, lbool.FromBool(value)))
				benchmarkMust(b, p.assignmentVariableEvent(variable, value))
				if !p.IsRelevant(children[index]) {
					b.Fatal("callback did not mark the witness")
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := p.assignmentVariableEvent(variable, value); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// root-B/solver measures live heap above a solver with rooted ASTs after it
// asserts its disjunctions. idle-B/solver is the growth after popping them.
// Run in a fresh process with -benchtime=1x to compare worker footprints.
func BenchmarkSMTAssertedOrWorkerMemory(b *testing.B) {
	for _, count := range []int{32, 1024} {
		b.Run(fmt.Sprintf("Workers%d", count), func(b *testing.B) {
			workers := make([]*scopeBenchmarkWorker, count)
			clauses := make([][]ast.ASTHandle, count)
			for i := range workers {
				w := newScopeBenchmarkWorker(b, 16)
				workers[i] = w
				for j := 0; j < 16; j += 2 {
					handles := []ast.ASTHandle{w.expressions[j], w.expressions[j+1],
						w.expressions[(j+2)%16], w.expressions[(j+3)%16]}
					gate, err := w.s.manager.MkOrArray(handles)
					benchmarkMust(b, err)
					ref, err := w.s.manager.NewRef(gate)
					benchmarkMust(b, err)
					b.Cleanup(func() { benchmarkMust(b, w.s.manager.DelRef(&ref)) })
					clauses[i] = append(clauses[i], gate)
				}
			}
			before := scopeBenchmarkLiveHeap()
			var asserted, idle int64
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for i, w := range workers {
					benchmarkMust(b, w.s.PushScope())
					for _, gate := range clauses[i] {
						benchmarkMust(b, w.s.Assert(gate, ast.ASTHandle{}, 0))
					}
				}
				asserted = scopeBenchmarkLiveHeap()
				for _, w := range workers {
					benchmarkMust(b, w.s.PopScopes(1))
				}
				idle = scopeBenchmarkLiveHeap()
			}
			b.ReportMetric(float64(asserted-before)/float64(count), "root-B/solver")
			b.ReportMetric(float64(idle-before)/float64(count), "idle-B/solver")
			runtime.KeepAlive(clauses)
			runtime.KeepAlive(workers)
		})
	}
}
