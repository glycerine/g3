package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationExplanationProductSelectedCheckedUDocLifecycle(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	astTestMust(t, f.relations.SetRelationExplanationUDocFactory(factory))
	proof := relationExplanationProof(t, f.ast, "udoc-relation-explanation-proof")
	product, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if product.doc == nil || product.table != nil || !product.doc.Doc().Inner().FormulaChecked() {
		t.Fatal("selected doc relation was not installed as the checked explanation sieve")
	}
	value := f.bv(2)
	astTestMust(t, product.doc.AddFact([]ast.ASTHandle{value}))
	astTestMust(t, product.explanation.AddFact([]ast.ASTHandle{proof}))
	if checked, err := product.FormulaChecked(); err != nil || !checked {
		t.Fatal("doc formula checker state", checked, err)
	}
	if primary, checker, checked, err := product.CheckedConcreteKinds(); err != nil || !checked || primary != "doc" || checker != "formula" {
		t.Fatal("doc checker identity", primary, checker, checked, err)
	}
	if got, found, err := product.Explanation([]ast.ASTHandle{value}); err != nil || !found || got != proof {
		t.Fatal("doc-backed explanation lookup", got, found, err)
	}
	clone, err := product.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	joined, err := product.Join(clone, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	if !joined.doc.Doc().Inner().FormulaChecked() {
		t.Fatal("doc join lost formula checking")
	}
	projected, err := joined.ProjectColumns([]int{2, 3})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if len(projected.finiteSorts) != 2 {
		t.Fatal("proof projection changed doc finite columns", projected.finiteSorts)
	}
	formula, err := projected.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	if _, err := f.ast.RefGet(formula); err != nil {
		t.Fatal("doc relation formula is not readable", err)
	}
}

func TestRelationExplanationProductCheckedUDocNegation(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	astTestMust(t, f.relations.SetRelationExplanationUDocFactory(factory))
	target, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	negative, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, target.Close())
		astTestMust(t, negative.Close())
	}()
	value := f.bv(2)
	astTestMust(t, target.doc.AddFact([]ast.ASTHandle{value}))
	astTestMust(t, negative.doc.AddFact([]ast.ASTHandle{value}))
	astTestMust(t, target.explanation.AddFact([]ast.ASTHandle{relationExplanationProof(t, f.ast, "udoc-neg-target")}))
	astTestMust(t, negative.explanation.AddFact([]ast.ASTHandle{relationExplanationProof(t, f.ast, "udoc-neg-source")}))
	if err = target.FilterByNegation(negative, []int{0}, []int{0}); err != nil {
		t.Fatal("checked UDoc relation negation", err)
	}
	if found, err := target.doc.ContainsFact([]ast.ASTHandle{value}); err != nil || found {
		t.Fatal("negation kept matching UDoc tuple", found, err)
	}
}

func TestRelationExplanationProductUDocMixedFiniteJoinsPreserveOperandOrder(t *testing.T) {
	f := newTableRelationFixture(t)
	// Construct the finite side before selecting DOC as the explanation sieve;
	// this models an already-created ordinary relation paired with a DOC side.
	finite, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	value := f.bv(2)
	valueElement, err := f.relations.EncodeValue(f.bits, value)
	astTestMust(t, err)
	if _, err = finite.table.Add(Fact{valueElement}); err != nil {
		t.Fatal(err)
	}
	proof := relationExplanationProof(t, f.ast, "udoc-mixed-finite-proof")
	astTestMust(t, finite.explanation.AddFact([]ast.ASTHandle{proof}))
	factory, err := NewFormulaCheckedUDocFactory(f.ast, arena.VectorConfig{}, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	astTestMust(t, f.relations.SetRelationExplanationUDocFactory(factory))
	doc, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	astTestMust(t, doc.doc.AddFact([]ast.ASTHandle{value}))
	astTestMust(t, doc.explanation.AddFact([]ast.ASTHandle{relationExplanationProof(t, f.ast, "udoc-mixed-doc-proof")}))
	t.Cleanup(func() {
		astTestMust(t, finite.Close())
		astTestMust(t, doc.Close())
	})
	// dl_sieve_relation.cpp and udoc_relation.cpp cannot construct a negation
	// filter when only one operand has the DOC inner relation. The relation
	// manager tries both plugins and receives nullptr, so Go must refuse this
	// unsupported mixed operation without mutating either operand.
	if err := doc.FilterByNegation(finite, []int{0}, []int{0}); err == nil {
		t.Fatal("DOC target with finite negative unexpectedly constructed a negation filter")
	}
	if found, err := doc.doc.ContainsFact([]ast.ASTHandle{value}); err != nil || !found {
		t.Fatalf("refused mixed negation changed DOC target: found=%t err=%v", found, err)
	}
	if err := finite.FilterByNegation(doc, []int{0}, []int{0}); err == nil {
		t.Fatal("finite target with DOC negative unexpectedly constructed a negation filter")
	}
	if found, err := finite.table.Contains(Fact{valueElement}); err != nil || !found {
		t.Fatalf("refused mixed negation changed finite target: found=%t err=%v", found, err)
	}

	for _, tc := range []struct {
		name        string
		left, right *RelationExplanationProduct
	}{
		{name: "doc_then_finite", left: doc, right: finite},
		{name: "finite_then_doc", left: finite, right: doc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			joined, err := tc.left.Join(tc.right, []int{0}, []int{0})
			astTestMust(t, err)
			defer func() { astTestMust(t, joined.Close()) }()
			if err := joined.doc.open(); err != nil {
				t.Fatalf("mixed result is not a valid DOC/table product: %v; mask=%v sorts=%v", err, joined.doc.tableColumns, joined.doc.doc.sorts)
			}
			if joined.doc == nil || joined.doc.Table() == nil {
				t.Fatal("mixed join did not preserve DOC and table components")
			}
			if found, err := joined.doc.Doc().ContainsFact([]ast.ASTHandle{value, value}); err != nil || !found {
				t.Fatalf("mixed DOC component membership: found=%t err=%v", found, err)
			}
			if found, err := joined.doc.ContainsFact([]ast.ASTHandle{value, value}); err != nil || !found {
				t.Fatalf("mixed product membership: found=%t err=%v mask=%v", found, err, joined.doc.tableColumns)
			}
			formula, err := joined.ToFormula()
			astTestMust(t, err)
			defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
			if _, err := f.ast.RefGet(formula); err != nil {
				t.Fatal("mixed join formula is not readable", err)
			}
		})
	}
}

func TestRelationExplanationProductUDocCapabilityFallsBackForInfiniteSort(t *testing.T) {
	f := newTableRelationFixture(t)
	factory, err := NewUDocRelationFactory(f.ast, arena.VectorConfig{})
	astTestMust(t, err)
	astTestMust(t, f.relations.SetRelationExplanationUDocFactory(factory))
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	if handles, err := factory.CanHandleSignature([]ast.ASTHandle{integer}); err != nil || handles {
		t.Fatal("DOC accepted an infinite integer signature", handles, err)
	}
}
