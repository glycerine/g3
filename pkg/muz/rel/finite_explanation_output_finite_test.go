package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationCheckedConversionProjectsBothPlugins(t *testing.T) {
	for _, kinds := range [][2]string{{"sparse", "sparse"}, {"lazy_sparse", "sparse"}, {"sparse", "lazy_sparse"}} {
		t.Run(kinds[0]+"-"+kinds[1], func(t *testing.T) {
			f := newTableRelationFixture(t)
			lazy := NewLazySparseFiniteFactory()
			defer func() { astTestMust(t, lazy.Close()) }()
			factory, err := NewFiniteCheckedFactoryWithLazy(kinds[0], kinds[1], lazy)
			astTestMust(t, err)
			astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
			sort, err := f.ast.MkDatalogRuleSort()
			astTestMust(t, err)
			name, err := f.ast.Symbols.Intern("checked-conversion-proof")
			astTestMust(t, err)
			proof, err := f.ast.MkConst(name, sort)
			astTestMust(t, err)
			source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
			astTestMust(t, err)
			defer func() { astTestMust(t, source.Close()) }()
			for _, values := range [][]ast.ASTHandle{{f.falsehood, f.truth}, {f.truth, f.falsehood}, {f.truth, f.truth}} {
				astTestMust(t, source.AddFact(values, proof))
			}
			if converted, e := source.ToFiniteTable(); e == nil || converted != nil {
				t.Fatal("unprojected proof column converted", converted, e)
			}
			projected, err := source.ProjectExplanation([]int{0})
			astTestMust(t, err)
			defer func() { astTestMust(t, projected.Close()) }()
			// Conversion first removes a finite key whose inner identity became empty.
			key := Fact{1, 1, 0}
			found, err := projected.table.FetchFact(key)
			astTestMust(t, err)
			if !found {
				t.Fatal("missing staged finite key")
			}
			retired := projected.inner[key[2]]
			astTestMust(t, retired.Reset())
			astTestMust(t, f.relations.SetCheckedFiniteFactory(nil))
			converted, err := projected.ToFiniteTable()
			astTestMust(t, err)
			defer func() { astTestMust(t, converted.Close()) }()
			checked, ok := converted.(*FiniteCheckedPair)
			if !ok {
				t.Fatalf("converted type %T, want checked pair", converted)
			}
			primary, checker, err := checked.ConcreteKinds()
			astTestMust(t, err)
			if primary != kinds[0] || checker != kinds[1] {
				t.Fatalf("converted kinds (%s,%s), want %v", primary, checker, kinds)
			}
			if !retired.closed {
				t.Fatal("conversion did not release empty inner owner")
			}
			for a := 0; a < 2; a++ {
				for b := 0; b < 2; b++ {
					found, e := converted.Contains(Fact{Element(a), Element(b)})
					astTestMust(t, e)
					if found != (a != b) {
						t.Fatalf("converted tuple (%d,%d) membership %t", a, b, found)
					}
				}
			}
			// Actual instruction consumes the typed conversion with a plain destination
			// after the checked factory has ended, without changing destination kinds.
			context, err := NewExecutionContextWithRelations(f.relations, nil)
			astTestMust(t, err)
			defer func() { astTestMust(t, context.Close()) }()
			clone, err := projected.Clone()
			astTestMust(t, err)
			astTestMust(t, context.SetExplanationRegister(0, clone))
			union := NewUnionInstruction(0, 1, 2)
			defer func() { astTestMust(t, union.Close()) }()
			complete, err := union.Perform(context)
			astTestMust(t, err)
			if !complete {
				t.Fatal("converted union did not complete")
			}
			for _, register := range []uint32{1, 2} {
				table, e := context.FiniteRegister(register)
				astTestMust(t, e)
				if _, ok := table.(*HashTable); !ok {
					t.Fatalf("destination type %T, want current Hash favourite", table)
				}
				count, e := table.Len()
				astTestMust(t, e)
				if count != 2 {
					t.Fatalf("destination row count %d, want 2", count)
				}
			}
			astTestMust(t, projected.Close())
			astTestMust(t, source.Close())
			inserted, err := converted.Add(Fact{0, 0})
			astTestMust(t, err)
			if !inserted {
				t.Fatal("converted table did not survive proof owners")
			}
			found, err = converted.Contains(Fact{0, 1})
			astTestMust(t, err)
			if !found {
				t.Fatal("mutating converted table lost an existing tuple")
			}
		})
	}
}
