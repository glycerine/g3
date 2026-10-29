package ast

// Source: src/util/sorting_network.h selectable cardinality encodings.

import (
	"errors"
	"testing"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestPsortASTEncodingFamiliesIndependentClauseOracle(t *testing.T) {
	encodings := []PsortASTEncoding{PsortASTGrouped, PsortASTBimander, PsortASTOrdered, PsortASTUnate, PsortASTCircuit}
	for _, encoding := range encodings {
		for mode := 0; mode < 3; mode++ {
			for k := uint32(0); k <= 6; k++ {
				if mode == 2 && (encoding == PsortASTCircuit || encoding == PsortASTUnate) && (k == 0 || k == 5) {
					continue // literal source circuit_eq(0) quirk is checked separately.
				}
				m := astTestManager(t)
				context, e := NewPsortASTContext(m)
				astTestMust(t, e)
				runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
				astTestMust(t, e)
				network, e := NewPsortASTNetwork(context, runtime, m.BudgetField, nil)
				astTestMust(t, e)
				astTestMust(t, network.SetEncoding(encoding))
				input, e := NewASTLiteralVector(m.BudgetField)
				astTestMust(t, e)
				sort, e := m.MkBoolSort()
				astTestMust(t, e)
				literals := make([]ASTHandle, 5)
				for i := range literals {
					literals[i], e = m.MkFreshConst("encoding", sort)
					astTestMust(t, e)
					astTestPin(t, m, literals[i])
					astTestMust(t, input.Push(literals[i]))
				}
				span, e := input.Span()
				astTestMust(t, e)
				var result ASTHandle
				switch mode {
				case 0:
					result, e = network.Ge(true, k, 5, span)
				case 1:
					result, e = network.Le(true, k, 5, span)
				case 2:
					result, e = network.Eq(true, k, 5, span)
				}
				if e != nil {
					t.Fatal("encoding call", encoding, mode, k, e)
				}
				oracle := psortOracleCNF{ids: map[ASTHandle]int{}}
				rid := oracle.gate(t, m, result)
				for _, clause := range context.clauses {
					oracle.clauses = append(oracle.clauses, []int{oracle.gate(t, m, clause)})
				}
				for _, literal := range literals {
					oracle.gate(t, m, literal)
				}
				for assignment := 0; assignment < 32; assignment++ {
					values := make([]int8, oracle.count+1)
					count := uint32(0)
					for i, literal := range literals {
						values[oracle.ids[literal]] = -1
						if assignment&(1<<i) != 0 {
							values[oracle.ids[literal]] = 1
							count++
						}
					}
					want := count >= k
					if mode == 1 {
						want = count <= k
					} else if mode == 2 {
						want = count == k
					}
					positive := append([]int8(nil), values...)
					positive[rid] = 1
					if got := psortOracleSAT(oracle.clauses, positive); got != want {
						t.Fatal("encoding oracle", encoding, mode, k, assignment, count, got, want)
					}
					negative := append([]int8(nil), values...)
					negative[rid] = -1
					if got := psortOracleSAT(oracle.clauses, negative); got == want {
						t.Fatal("encoding complement", encoding, mode, k, assignment, count, got, want)
					}
				}
				astTestMust(t, network.Close())
				astTestMust(t, input.Close())
				astTestMust(t, runtime.Close())
				astTestMust(t, context.Close())
			}
		}
	}
}

func TestPsortASTCircuitEqualityZeroLiteralSourceBehavior(t *testing.T) {
	m := astTestManager(t)
	context, e := NewPsortASTContext(m)
	astTestMust(t, e)
	runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	network, e := NewPsortASTNetwork(context, runtime, m.BudgetField, nil)
	astTestMust(t, e)
	astTestMust(t, network.SetEncoding(PsortASTCircuit))
	input, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	for i := 0; i < 3; i++ {
		h, x := m.MkFreshConst("circuit-zero", sort)
		astTestMust(t, x)
		astTestMust(t, input.Push(h))
	}
	span, e := input.Span()
	astTestMust(t, e)
	result, e := network.Eq(true, 0, 3, span)
	astTestMust(t, e)
	if !psortOracleEval(t, m, result, map[ASTHandle]bool{}) {
		t.Fatal("literal source circuit equality-zero branch changed", result)
	}
	astTestMust(t, network.SetEncoding(PsortASTUnate))
	if _, e = network.Eq(true, 0, 3, span); !errors.Is(e, ErrASTDomain) {
		t.Fatal("unate equality-zero source underflow was not checked", e)
	}
	astTestMust(t, network.Close())
	astTestMust(t, input.Close())
	astTestMust(t, runtime.Close())
	astTestMust(t, context.Close())
}
