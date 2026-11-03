package rel

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func directExplanationIndexTable(kind string, lazy *LazySparseFiniteFactory, signature Signature) (FiniteTable, error) {
	switch kind {
	case "hashtable":
		return NewHashTable(signature), nil
	case "sparse":
		return NewSparseTable(signature, 1)
	case "lazy_sparse":
		if lazy == nil {
			return nil, ErrCheckTable
		}
		return lazy.NewFunctional(signature, 1)
	default:
		return nil, ErrCheckTable
	}
}

func directCheckedExplanationIndex(primaryKind, checkerKind string, lazy *LazySparseFiniteFactory, signature Signature) (*CheckedExplanationIndex, error) {
	primary, err := directExplanationIndexTable(primaryKind, lazy, signature)
	if err != nil {
		return nil, err
	}
	checker, err := directExplanationIndexTable(checkerKind, lazy, signature)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptCheckedExplanationIndex(primary, checker, 1)
}

func TestCheckedExplanationIndexFunctionalMatrixAndSuffixMismatch(t *testing.T) {
	signature, err := NewSignature([]uint64{2, math.MaxInt32})
	if err != nil {
		t.Fatal(err)
	}
	for _, primary := range []string{"hashtable", "sparse"} {
		for _, checker := range []string{"hashtable", "sparse"} {
			name := primary + "-checked-by-" + checker
			t.Run(name, func(t *testing.T) {
				index, err := directCheckedExplanationIndex(primary, checker, nil, signature)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := index.Close(); err != nil {
						t.Fatal(err)
					}
				}()

				gotPrimary, gotChecker, err := index.ConcreteKinds()
				if err != nil || gotPrimary != primary || gotChecker != checker {
					t.Fatalf("concrete pair (%q, %q, %v), want (%q, %q)", gotPrimary, gotChecker, err, primary, checker)
				}
				if err = index.EnsureFact(Fact{1, 3}); err != nil {
					t.Fatal(err)
				}
				probe := Fact{1, 0}
				if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 3 {
					t.Fatalf("initial fetch (%v, %v, %v), want found suffix 3", probe, found, err)
				}
				if err = index.EnsureFact(Fact{1, 5}); err != nil {
					t.Fatal(err)
				}
				probe[1] = 0
				if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 5 {
					t.Fatalf("updated fetch (%v, %v, %v), want found suffix 5", probe, found, err)
				}
				if rows, err := index.Facts(); err != nil || len(rows) != 1 || rows[0][0] != 1 || rows[0][1] != 5 {
					t.Fatalf("updated rows %v, %v", rows, err)
				}
				if err = index.EnsureFact(Fact{1, Element(math.MaxInt32)}); err == nil {
					t.Fatal("out-of-domain functional value was accepted")
				}
				probe[1] = 0
				if found, err := index.FetchFact(probe); err != nil || !found || probe[1] != 5 {
					t.Fatalf("invalid update changed prior row (%v, %v, %v)", probe, found, err)
				}
				if removed, err := index.Remove(Fact{1, 4}); err != nil || removed {
					t.Fatalf("mismatched functional suffix removal (%v, %v), want false", removed, err)
				}
				if removed, err := index.Remove(Fact{1, 5}); err != nil || !removed {
					t.Fatalf("remove (%v, %v), want true", removed, err)
				}
				if length, err := index.Len(); err != nil || length != 0 {
					t.Fatalf("length (%d, %v), want zero", length, err)
				}

				if err = index.EnsureFact(Fact{0, 7}); err != nil {
					t.Fatal(err)
				}
				if err = explanationIndexEnsure(index.checker, Fact{0, 9}, 1); err != nil {
					t.Fatal(err)
				}
				if _, err = index.Facts(); !errors.Is(err, ErrCheckTable) {
					t.Fatalf("suffix mismatch facts error %v, want ErrCheckTable", err)
				}
				probe = Fact{0, 0}
				if _, err = index.FetchFact(probe); !errors.Is(err, ErrCheckTable) {
					t.Fatalf("suffix mismatch error %v, want ErrCheckTable", err)
				}
			})
		}
	}
}

func TestCheckedExplanationProductProjectUnionAndNullaryOwnership(t *testing.T) {
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
				checkKinds := func(label string, product *FiniteExplanationProduct) {
					t.Helper()
					gotPrimary, gotChecker, checked, e := product.CheckedConcreteKinds()
					wantChecked := primary != "hashtable" && checker != "hashtable"
					if e != nil {
						t.Fatalf("%s concrete kind error: %v", label, e)
					}
					if wantChecked {
						if !checked || gotPrimary != primary || gotChecker != checker {
							t.Fatalf("%s concrete pair (%q, %q, checked=%t), want (%q, %q)", label, gotPrimary, gotChecker, checked, primary, checker)
						}
					} else if checked {
						t.Fatalf("%s unexpectedly used checked Hash functional pair (%q, %q)", label, gotPrimary, gotChecker)
					} else if _, ok := product.table.(*SparseTable); !ok {
						t.Fatalf("%s source fallback type %T, want *SparseTable", label, product.table)
					}
				}

				first := makeProof("checked-explanation-first")
				second := makeProof("checked-explanation-second")
				target, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
				astTestMust(t, err)
				defer func() { astTestMust(t, target.Close()) }()
				source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
				astTestMust(t, err)
				defer func() { astTestMust(t, source.Close()) }()
				delta, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
				astTestMust(t, err)
				defer func() { astTestMust(t, delta.Close()) }()
				checkKinds("target", target)
				checkKinds("source", source)
				checkKinds("delta", delta)
				astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth, f.falsehood}, first))
				astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, f.truth}, second))
				astTestMust(t, target.Union(source, delta))
				if got, found, e := delta.Explanation([]ast.ASTHandle{f.falsehood, f.truth}); e != nil || !found || got != second {
					t.Fatalf("union delta (%v, %v, %v), want second proof", got, found, e)
				}
				projected, err := target.Project([]int{1})
				astTestMust(t, err)
				defer func() { astTestMust(t, projected.Close()) }()
				checkKinds("projected", projected)
				for _, tc := range []struct {
					key, proof ast.ASTHandle
				}{{f.truth, first}, {f.falsehood, second}} {
					got, found, e := projected.Explanation([]ast.ASTHandle{tc.key})
					if e != nil || !found || got != tc.proof {
						t.Fatalf("projected explanation (%v, %v, %v), want %v", got, found, e, tc.proof)
					}
				}

				nullaryProof := makeProof("checked-explanation-nullary")
				proofRef, err := f.ast.NewRef(nullaryProof)
				astTestMust(t, err)
				nullary, err := NewFiniteExplanationProduct(f.relations, nil)
				astTestMust(t, err)
				checkKinds("nullary", nullary)
				astTestMust(t, nullary.AddFact(nil, nullaryProof))
				astTestMust(t, f.ast.DelRef(&proofRef))
				if got, found, e := nullary.Explanation(nil); e != nil || !found || got != nullaryProof {
					t.Fatalf("nullary explanation (%v, %v, %v), want owned proof", got, found, e)
				}
				astTestMust(t, nullary.Close())
				if _, err = f.ast.GetKind(nullaryProof); err == nil {
					t.Fatal("closed nullary product kept proof alive")
				}
				astTestMust(t, f.relations.SetCheckedFiniteFactory(nil))
				afterRestore, err := target.Project([]int{1})
				astTestMust(t, err)
				checkKinds("projected after factory restore", afterRestore)
				astTestMust(t, afterRestore.Close())
			})
		}
	}
}
