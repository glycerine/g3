package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckedLazyExplanationOutputUnionUsesConfiguredTables(t *testing.T) {
	for _, pair := range [][2]string{{"lazy_sparse", "hashtable"}, {"sparse", "lazy_sparse"}, {"lazy_sparse", "lazy_sparse"}} {
		for _, nullary := range []bool{false, true} {
			name := pair[0] + "-" + pair[1]
			if nullary {
				name += "-nullary"
			}
			t.Run(name, func(t *testing.T) {
				f := newTableRelationFixture(t)
				lazy := NewLazySparseFiniteFactory()
				defer func() { astTestMust(t, lazy.Close()) }()
				factory, err := NewFiniteCheckedFactoryWithLazy(pair[0], pair[1], lazy)
				astTestMust(t, err)
				astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
				ruleSort, err := f.ast.MkDatalogRuleSort()
				astTestMust(t, err)
				symbol, err := f.ast.Symbols.Intern("lazy-output-proof")
				astTestMust(t, err)
				proof, err := f.ast.MkConst(symbol, ruleSort)
				astTestMust(t, err)
				sorts, values, want := []ast.ASTHandle{f.boolean}, []ast.ASTHandle{f.truth}, Fact{1}
				if nullary {
					sorts, values, want = nil, nil, Fact{}
				}
				source, err := NewFiniteExplanationProduct(f.relations, sorts)
				astTestMust(t, err)
				defer func() { astTestMust(t, source.Close()) }()
				astTestMust(t, source.AddFact(values, proof))
				projected, err := source.ProjectExplanation([]int{0})
				astTestMust(t, err)
				context, err := NewExecutionContextWithRelations(f.relations, nil)
				astTestMust(t, err)
				defer func() { astTestMust(t, context.Close()) }()
				astTestMust(t, context.SetExplanationRegister(0, projected))
				union := NewUnionInstruction(0, 1, 2)
				defer func() { astTestMust(t, union.Close()) }()
				complete, err := union.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("projected explanation union did not complete")
				}
				for _, register := range []uint32{1, 2} {
					table, e := context.FiniteRegister(register)
					astTestMust(t, e)
					if nullary {
						if _, ok := table.(*HashTable); !ok {
							t.Fatalf("register %d nullary fallback %T, want Hash", register, table)
						}
					} else {
						checked, ok := table.(*FiniteCheckedPair)
						if !ok {
							t.Fatalf("register %d type %T, want checked", register, table)
						}
						primary, checker, e := checked.ConcreteKinds()
						astTestMust(t, e)
						if primary != pair[0] || checker != pair[1] {
							t.Fatalf("register %d pair (%s,%s), want %v", register, primary, checker, pair)
						}
					}
					rows, e := table.Facts()
					astTestMust(t, e)
					if len(rows) != 1 || len(rows[0]) != len(want) || len(want) != 0 && rows[0][0] != want[0] {
						t.Fatalf("register %d rows %v, want [%v]", register, rows, want)
					}
				}
				got, found, err := source.Explanation(values)
				astTestMust(t, err)
				if !found || got != proof {
					t.Fatal("finite output conversion changed source proof")
				}
			})
		}
	}
}
