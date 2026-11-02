package ast

// Source: src/util/sorting_network.h:368-459 weighted comparison network.

import (
	"testing"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestPsortASTNetworkWeightedIndependentClauseOracle(t *testing.T) {
	weights := []uint32{1, 2, 3, 5}
	for mode := 0; mode < 3; mode++ {
		for k := uint32(0); k <= 12; k++ {
			m := astTestManager(t)
			c, e := NewPsortASTContext(m)
			astTestMust(t, e)
			runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
			astTestMust(t, e)
			network, e := NewPsortASTNetwork(c, runtime, m.BudgetField, nil)
			astTestMust(t, e)
			input, e := NewASTLiteralVector(m.BudgetField)
			astTestMust(t, e)
			sort, e := m.MkBoolSort()
			astTestMust(t, e)
			var literals []ASTHandle
			for range weights {
				h, x := m.MkFreshConst("weighted", sort)
				astTestMust(t, x)
				astTestPin(t, m, h)
				literals = append(literals, h)
				astTestMust(t, input.Push(h))
			}
			span, e := input.Span()
			astTestMust(t, e)
			var result ASTHandle
			switch mode {
			case 0:
				result, e = network.WeightedGe(k, weights, span)
			case 1:
				result, e = network.WeightedLe(k, weights, span)
			case 2:
				result, e = network.WeightedEq(k, weights, span)
			}
			astTestMust(t, e)
			oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
			rid := oracle.gate(t, m, result)
			for _, clause := range c.clauses {
				oracle.clauses = append(oracle.clauses, []int{oracle.gate(t, m, clause)})
			}
			for _, literal := range literals {
				oracle.gate(t, m, literal)
			}
			for assignment := 0; assignment < 1<<len(weights); assignment++ {
				values := make([]int8, oracle.count+1)
				var sum uint32
				for i, literal := range literals {
					values[oracle.ids[literal]] = -1
					if assignment&(1<<i) != 0 {
						values[oracle.ids[literal]] = 1
						sum += weights[i]
					}
				}
				want := sum >= k
				if mode == 1 {
					want = sum <= k
				} else if mode == 2 {
					want = sum == k
				}
				values[rid] = 1
				if got := psortOracleSAT(oracle.clauses, values); got != want {
					t.Fatal("weighted oracle", mode, k, assignment, sum, got, want)
				}
			}
			astTestMust(t, network.Close())
			astTestMust(t, input.Close())
			astTestMust(t, runtime.Close())
			astTestMust(t, c.Close())
		}
	}
}

func TestPsortASTNetworkWeightedMaximumBoundOracle(t *testing.T) {
	weights := []uint32{4294967294, 1}
	for mode := 0; mode < 3; mode++ {
		for _, k := range []uint32{4294967295} {
			m := astTestManager(t)
			c, e := NewPsortASTContext(m)
			astTestMust(t, e)
			runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
			astTestMust(t, e)
			network, e := NewPsortASTNetwork(c, runtime, m.BudgetField, nil)
			astTestMust(t, e)
			input, e := NewASTLiteralVector(m.BudgetField)
			astTestMust(t, e)
			sort, e := m.MkBoolSort()
			astTestMust(t, e)
			var literals []ASTHandle
			for range weights {
				h, x := m.MkFreshConst("weighted", sort)
				astTestMust(t, x)
				astTestPin(t, m, h)
				literals = append(literals, h)
				astTestMust(t, input.Push(h))
			}
			span, e := input.Span()
			astTestMust(t, e)
			var result ASTHandle
			switch mode {
			case 0:
				result, e = network.WeightedGe(k, weights, span)
			case 1:
				result, e = network.WeightedLe(k, weights, span)
			case 2:
				result, e = network.WeightedEq(k, weights, span)
			}
			astTestMust(t, e)
			oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
			rid := oracle.gate(t, m, result)
			for _, clause := range c.clauses {
				oracle.clauses = append(oracle.clauses, []int{oracle.gate(t, m, clause)})
			}
			for _, literal := range literals {
				oracle.gate(t, m, literal)
			}
			for assignment := 0; assignment < 1<<len(weights); assignment++ {
				values := make([]int8, oracle.count+1)
				var sum uint64
				for i, literal := range literals {
					values[oracle.ids[literal]] = -1
					if assignment&(1<<i) != 0 {
						values[oracle.ids[literal]] = 1
						sum += uint64(weights[i])
					}
				}
				want := sum >= uint64(k)
				if mode == 1 {
					want = sum <= uint64(k)
				} else if mode == 2 {
					want = sum == uint64(k)
				}
				values[rid] = 1
				if got := psortOracleSAT(oracle.clauses, values); got != want {
					t.Fatal("weighted oracle", mode, k, assignment, sum, got, want)
				}
			}
			astTestMust(t, network.Close())
			astTestMust(t, input.Close())
			astTestMust(t, runtime.Close())
			astTestMust(t, c.Close())
		}
	}
}
