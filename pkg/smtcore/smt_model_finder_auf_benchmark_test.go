package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Model construction revisits the quantifiers registered during internalization.
// Seven predicates are shared and one is distinct for each quantified formula.
func BenchmarkSMTModelFinderAUFGraph(b *testing.B) {
	benchmarkSMTModelFinderAUFGraph(b, 0)
}

func BenchmarkSMTModelFinderSharedGroundArguments(b *testing.B) {
	benchmarkSMTModelFinderAUFGraph(b, 32)
}

func benchmarkSMTModelFinderAUFGraph(b *testing.B, groundArguments int) {
	const relations = 8
	for _, count := range []int{8, 32, 128, 512} {
		b.Run(fmt.Sprintf("Quantifiers%d", count), func(b *testing.B) {
			m, state, _, manager := smtQuantifierManagerFixture(b, 10, 20, SMTQuantifierManagerConfig{})
			sort, err := m.MkSort(astTestSymbol(b, m, "auf-bench-sort"), ast.ASTSortInfo{})
			astTestMust(b, err)
			boolean, err := m.MkBoolSort()
			astTestMust(b, err)
			x, err := m.MkVar(0, sort)
			astTestMust(b, err)
			for i := 0; i < groundArguments; i++ {
				_, child := smtEnodeConstant(b, m, uint32(i+1), sort, uint32(i))
				astTestMust(b, state.context.RegisterEnode(child, true))
				for j := 0; j < relations-1; j++ {
					decl := smtQuickPredicate(b, m, fmt.Sprintf("auf-bench-p%d", j), sort, boolean)
					smtQuickApplicationNode(b, m, state.context, decl, child, true)
				}
			}
			predicate := func(i int) ast.ASTHandle {
				f := smtQuickPredicate(b, m, fmt.Sprintf("auf-bench-p%d", i), sort, boolean)
				h, err := m.MkApp(f, []ast.ASTHandle{x})
				astTestMust(b, err)
				return h
			}
			common := predicate(0)
			for i := 1; i < relations-1; i++ {
				common, err = m.MkAnd(common, predicate(i))
				astTestMust(b, err)
			}
			quantifiers := make([]ast.ASTHandle, count)
			for i := range quantifiers {
				body, err := m.MkAnd(common, predicate(relations-1+i))
				astTestMust(b, err)
				h, err := m.MkQuantifier(ast.ASTQuantifierSpec{
					Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
				})
				astTestMust(b, err)
				astTestMust(b, manager.modelFinder.RegisterQuantifier(h))
				if info := manager.modelFinder.qinfos[h]; len(info.fvars) != relations {
					b.Fatalf("got %d function-variable records, want %d", len(info.fvars), relations)
				}
				quantifiers[i] = h
			}
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			defer func() { astTestMust(b, graph.close()) }()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := graph.reset(); err != nil {
					b.Fatal(err)
				}
				if err := graph.build(quantifiers); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(count*relations), "records/op")
			if got := graph.uvars[smtModelFinderAUFKey{quantifiers[0], 0}].root().set.size(); got != groundArguments {
				b.Fatalf("got %d distinct ground arguments, want %d", got, groundArguments)
			}
			if got := len(graph.uvars); got != count {
				b.Fatalf("got %d variable nodes, want %d", got, count)
			}
		})
	}
}
