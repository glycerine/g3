package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type quantifierCandidateFixture struct {
	manager          *SMTQuantifierManager
	body, quantifier ast.ASTHandle
	bindings         []*SMTEnode
}

func newQuantifierCandidateFixture(b testing.TB, width int) *quantifierCandidateFixture {
	b.Helper()
	m, state, _, qm := smtQuantifierManagerFixture(b, 10, 20, SMTQuantifierManagerConfig{})
	sort, err := m.MkUninterpretedSort(astTestSymbol(b, m, "candidate-sort"), nil)
	astTestMust(b, err)
	f := smtQuickPredicate(b, m, "candidate-function", sort, sort)
	x, err := m.MkVar(0, sort)
	astTestMust(b, err)
	fx, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(b, err)
	equalities := make([]ast.ASTHandle, width)
	bindings := make([]*SMTEnode, width+1)
	for i := range bindings {
		a, err := m.MkConst(astTestSymbol(b, m, fmt.Sprintf("candidate-%d", i)), sort)
		astTestMust(b, err)
		bindings[i], err = state.MkEnode(a, false, false, true)
		astTestMust(b, err)
		astTestMust(b, state.context.SetRelevant(a, true))
		if i == width {
			continue
		}
		fa, err := m.MkApp(f, []ast.ASTHandle{a})
		astTestMust(b, err)
		_, err = state.MkEnode(fa, false, false, true)
		astTestMust(b, err)
		astTestMust(b, state.context.SetRelevant(fa, true))
		equalities[i], err = m.MkEq(fx, fa)
		astTestMust(b, err)
	}
	body, err := m.MkOrArray(equalities)
	astTestMust(b, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort},
		Names: []symbol.Symbol{astTestSymbol(b, m, "x")}, Body: body, Weight: 1,
	})
	astTestMust(b, err)
	astTestMust(b, m.IncRef(quantifier))
	b.Cleanup(func() { astTestMust(b, m.DecRef(quantifier)) })
	return &quantifierCandidateFixture{manager: qm, body: body, quantifier: quantifier, bindings: bindings}
}

// The same f(x) occurs in each equality. Looking up f(binding) exercises the
// congruence table. The first and last bindings force different stop positions.
func BenchmarkSMTQuantifierCandidate(b *testing.B) {
	for _, width := range []int{4, 16} {
		b.Run(fmt.Sprintf("Width%d", width), func(b *testing.B) {
			fixture := newQuantifierCandidateFixture(b, width)
			qm, body, quantifier, bindings := fixture.manager, fixture.body, fixture.quantifier, fixture.bindings
			astTestMust(b, qm.state.Internalize(quantifier, true))
			stat, err := qm.registry.Stat(quantifier)
			astTestMust(b, err)
			// The extra binding has no congruent application. It must fail
			// even after a successful query with the same body.
			for _, index := range []int{0, width - 1, width} {
				got, err := qm.checker.IsSatisfiable(body, bindings[index:index+1])
				if err != nil || got != (index < width) {
					b.Fatalf("binding %d returned %t, %v", index, got, err)
				}
			}
			for _, position := range []struct {
				name  string
				index int
			}{{"First", 0}, {"Last", width - 1}} {
				args := bindings[position.index : position.index+1]
				b.Run(position.name+"/Checker", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if yes, err := qm.checker.IsSatisfiable(body, args); err != nil || !yes {
							b.Fatalf("checker returned %t, %v", yes, err)
						}
					}
				})
				b.Run(position.name+"/RegisteredChecker", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if yes, err := qm.checker.run(body, true, args, stat); err != nil || !yes {
							b.Fatalf("registered checker returned %t, %v", yes, err)
						}
					}
				})
				entry := []SMTQIQueueEntry{{Fingerprint: &SMTFingerprint{data: quantifier, args: args}}}
				b.Run(position.name+"/Queue", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if err := qm.instantiate(entry); err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					if stat.NumInstances() != 0 {
						b.Fatal("queue instantiated a satisfied body")
					}
				})
			}
		})
	}
}

// Keep 1,024 independent solvers alive. Each holds its quantified AST before
// the first measurement, registers it in a fresh scope, checks 32 candidates,
// and pops the scope. Run with -benchtime=1x for a single population.
func BenchmarkSMTQuantifierCandidateWorkerMemory(b *testing.B) {
	const workers, width = 1024, 16
	fixtures := make([]*quantifierCandidateFixture, workers)
	for i := range fixtures {
		fixtures[i] = newQuantifierCandidateFixture(b, width)
	}
	before := scopeBenchmarkLiveHeap()
	for i := 0; i < b.N; i++ {
		for _, f := range fixtures {
			astTestMust(b, f.manager.state.PushScope())
			astTestMust(b, f.manager.state.Internalize(f.quantifier, true))
			for j := 0; j < 32; j++ {
				n := j % width
				entry := SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: f.quantifier, args: f.bindings[n : n+1]}}
				astTestMust(b, f.manager.instantiate([]SMTQIQueueEntry{entry}))
			}
			stat, err := f.manager.registry.Stat(f.quantifier)
			astTestMust(b, err)
			if stat.NumInstances() != 0 {
				b.Fatal("queue instantiated a satisfied body")
			}
		}
		warm := scopeBenchmarkLiveHeap()
		for _, f := range fixtures {
			astTestMust(b, f.manager.state.PopScopes(1))
			if len(f.manager.registry.quantifiers) != 0 {
				b.Fatal("popped quantifier is still registered")
			}
		}
		popped := scopeBenchmarkLiveHeap()
		b.ReportMetric(float64(warm-before)/workers, "active-B/worker")
		b.ReportMetric(float64(popped-before)/workers, "popped-B/worker")
	}
	b.ReportMetric(workers, "workers")
	runtime.KeepAlive(fixtures)
}
