package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCheckedExplanationUnionInstructionPreservesSourceKindsAfterScope(t *testing.T) {
	for _, primary := range []string{"hashtable", "sparse"} {
		for _, checker := range []string{"hashtable", "sparse"} {
			t.Run(primary+"-checked-by-"+checker, func(t *testing.T) {
				f := newTableRelationFixture(t)
				factory, err := NewFiniteCheckedFactory(primary, checker)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.relations.SetCheckedFiniteFactory(factory); err != nil {
					t.Fatal(err)
				}
				ruleSort, err := f.ast.MkDatalogRuleSort()
				astTestMust(t, err)
				makeProof := func(name string) ast.ASTHandle {
					symbol, e := f.ast.Symbols.Intern(name)
					astTestMust(t, e)
					proof, e := f.ast.MkConst(symbol, ruleSort)
					astTestMust(t, e)
					return proof
				}
				first := makeProof("checked-bytecode-first")
				second := makeProof("checked-bytecode-second")
				source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
				astTestMust(t, err)
				astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, first))
				if err = f.relations.SetCheckedFiniteFactory(nil); err != nil {
					astTestMust(t, source.Close())
					t.Fatal(err)
				}

				context, err := NewExecutionContextWithRelations(f.relations, nil)
				if err != nil {
					astTestMust(t, source.Close())
					t.Fatal(err)
				}
				defer func() { astTestMust(t, context.Close()) }()
				astTestMust(t, context.SetExplanationRegister(0, source))
				union := NewUnionInstruction(0, 1, 2)
				defer func() { astTestMust(t, union.Close()) }()
				complete, err := union.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("checked explanation union did not complete")
				}
				if got, err := context.ExplanationRegister(0); err != nil || got != source {
					t.Fatalf("union changed source owner: got %p, want %p, error %v", got, source, err)
				}
				checkOutput := func(register uint32, label string, want ast.ASTHandle) *FiniteExplanationProduct {
					t.Helper()
					product, e := context.ExplanationRegister(register)
					if e != nil || product == nil {
						t.Fatalf("%s register: product %v, error %v", label, product, e)
					}
					gotPrimary, gotChecker, checked, e := product.CheckedConcreteKinds()
					wantChecked := primary != "hashtable" && checker != "hashtable"
					if e != nil {
						t.Fatalf("%s concrete kind error: %v", label, e)
					}
					if wantChecked {
						if !checked || gotPrimary != primary || gotChecker != checker {
							t.Fatalf("%s pair (%q, %q, checked=%t), want (%q, %q)", label, gotPrimary, gotChecker, checked, primary, checker)
						}
					} else if checked {
						t.Fatalf("%s unexpectedly used checked Hash functional pair (%q, %q)", label, gotPrimary, gotChecker)
					} else if _, ok := product.table.(*SparseTable); !ok {
						t.Fatalf("%s source fallback type %T, want *SparseTable", label, product.table)
					}
					got, found, e := product.Explanation([]ast.ASTHandle{f.truth})
					if e != nil || !found || got != want {
						t.Fatalf("%s proof (%v, found=%t, %v), want %v", label, got, found, e, want)
					}
					return product
				}
				target := checkOutput(1, "target", first)
				delta := checkOutput(2, "delta", first)

				overlap, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
				astTestMust(t, err)
				astTestMust(t, overlap.AddFact([]ast.ASTHandle{f.truth}, second))
				astTestMust(t, context.SetExplanationRegister(0, overlap))
				complete, err = union.Perform(context)
				astTestMust(t, err)
				if !complete {
					t.Fatal("overlap union did not complete")
				}
				if got, err := context.ExplanationRegister(0); err != nil || got != overlap {
					t.Fatalf("overlap union changed source owner: got %p, want %p, error %v", got, overlap, err)
				}
				if got, err := context.ExplanationRegister(1); err != nil || got != target {
					t.Fatal("overlap union replaced target owner", got, target, err)
				}
				if got, err := context.ExplanationRegister(2); err != nil || got != delta {
					t.Fatal("overlap union replaced delta owner", got, delta, err)
				}
				checkOutput(1, "overlap target", first)
				checkOutput(2, "overlap delta", first)
				if got, found, err := overlap.Explanation([]ast.ASTHandle{f.truth}); err != nil || !found || got != second {
					t.Fatalf("overlap source proof (%v, found=%t, %v), want %v", got, found, err, second)
				}
			})
		}
	}
}
