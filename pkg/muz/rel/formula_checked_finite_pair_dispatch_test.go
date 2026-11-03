package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationExplanationNestedCheckerConstructionAndMixedDispatch(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFiniteCheckedFactory("hashtable", "sparse")
	astTestMust(t, err)
	astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
	t.Cleanup(func() { astTestMust(t, f.relations.SetCheckedFiniteFactory(nil)) })
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	source := NewHashTable(signature)
	_, err = source.Add(Fact{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	nested, err := newRelationExplanationProductFromFiniteChecked(f.relations, []ast.ASTHandle{f.boolean}, source, relationExplanationProof(t, f.ast, "nested-checker-source"), true, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	defer func() { astTestMust(t, nested.Close()) }()
	if _, ok := nested.table.(*FormulaCheckedFinitePair); !ok {
		t.Fatalf("formula checker did not wrap checked finite pair: %T", nested.table)
	}
	primary, checker, checked, err := nested.CheckedConcreteKinds()
	if err != nil || !checked || primary != "hashtable" || checker != "sparse" {
		t.Fatal("nested checker kinds", primary, checker, checked, err)
	}
	if formula, err := nested.FormulaChecked(); err != nil || !formula {
		t.Fatal("missing outer formula checker", formula, err)
	}
	outerOnly := makeFormulaCheckedRelationProduct(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, relationExplanationProof(t, f.ast, "nested-checker-outer-only"))
	outerJoined, err := outerOnly.Join(nested, []int{0}, []int{0})
	astTestMust(t, err)
	astTestMust(t, outerOnly.Close())
	defer func() { astTestMust(t, outerJoined.Close()) }()
	if _, ok := outerJoined.table.(*FormulaCheckedFinitePair); !ok {
		t.Fatalf("outer-only/nested join discarded inner checker: %T", outerJoined.table)
	}

	explicitTable, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	_, err = explicitTable.Add(Fact{1})
	astTestMust(t, err)
	explicit := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, nil, []ast.ASTHandle{relationExplanationProof(t, f.ast, "nested-checker-explicit")})
	astTestMust(t, explicit.table.Close())
	explicit.table = explicitTable
	defer func() { astTestMust(t, explicit.Close()) }()
	joined, err := nested.Join(explicit, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	if _, ok := joined.table.(*FormulaCheckedFinitePair); !ok {
		t.Fatalf("typed mixed join lost nested checker: %T", joined.table)
	}

	target, err := nested.EmptyLike()
	astTestMust(t, err)
	delta, err := explicit.EmptyLike()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()); astTestMust(t, delta.Close()) }()
	astTestMust(t, target.Union(explicit, delta))
	if _, ok := target.table.(*FormulaCheckedFinitePair); !ok {
		t.Fatalf("typed mixed union lost nested checker: %T", target.table)
	}
	if rows, err := delta.table.Len(); err != nil || rows != 1 {
		t.Fatal("mixed nested delta publication", rows, err)
	}

	empty, err := target.table.(*FormulaCheckedFinitePair).EmptyLike()
	astTestMust(t, err)
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	astTestMust(t, setNonemptyFiniteRegister(context, 7, empty))
	stored, err := context.FiniteRegister(7)
	astTestMust(t, err)
	if _, ok := stored.(*FormulaCheckedFinitePair); !ok {
		t.Fatalf("empty nested owner discarded: %T", stored)
	}
}

func TestNestedCheckerLiftRejectsCorruptedOuterFormula(t *testing.T) {
	f := newTableRelationFixture(t)
	template := newFormulaCheckedFinitePairFixture(t, f, Fact{0})
	defer func() { astTestMust(t, template.Close()) }()
	outer := makeFormulaCheckedRelationProduct(t, f, []ast.ASTHandle{f.boolean}, []Fact{{0}}, relationExplanationProof(t, f.ast, "nested-corrupt-outer"))
	checked := outer.table.(*FormulaCheckedTable)
	if _, err := checked.Table().Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	lifted, err := liftFormulaCheckedFinitePairWithSorts(template, checked, []ast.ASTHandle{f.boolean})
	if !errors.Is(err, ErrRelationVerification) || lifted != nil {
		t.Fatal("corrupted outer formula crossed nested lift", lifted, err)
	}
	// Restore concrete/formula agreement so the original owner can close
	// through its normal lifecycle.
	if _, err = checked.Table().Remove(Fact{1}); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, outer.Close())
}

func TestRelationExplanationNestedCheckerNullaryCapabilityFallback(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFiniteCheckedFactory("hashtable", "sparse")
	astTestMust(t, err)
	astTestMust(t, f.relations.SetCheckedFiniteFactory(factory))
	t.Cleanup(func() { astTestMust(t, f.relations.SetCheckedFiniteFactory(nil)) })
	signature, err := NewSignature(nil)
	astTestMust(t, err)
	source := NewHashTable(signature)
	_, err = source.Add(Fact{})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	product, err := newRelationExplanationProductFromFiniteChecked(f.relations, nil, source, relationExplanationProof(t, f.ast, "nested-nullary-fallback"), true, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if _, ok := product.table.(*FormulaCheckedTable); !ok {
		t.Fatalf("nullary sparse checker did not use formula/hash fallback: %T", product.table)
	}
}
