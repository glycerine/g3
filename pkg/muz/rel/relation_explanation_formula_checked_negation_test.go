package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func makeFormulaCheckedRelationProduct(t *testing.T, f *tableRelationFixture, sorts []ast.ASTHandle, facts []Fact, proof ast.ASTHandle) *RelationExplanationProduct {
	t.Helper()
	product := relationExplanationProductWithProofRow(t, f, sorts, nil, []ast.ASTHandle{proof})
	relation, err := NewTableRelation(f.relations, sorts)
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(relation, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	for _, fact := range facts {
		_, err = checked.Add(fact)
		astTestMust(t, err)
	}
	astTestMust(t, product.table.Close())
	product.table = checked
	return product
}

func TestFormulaCheckedRelationProductNegationRejectsEqualDomainDifferentSorts(t *testing.T) {
	f := newTableRelationFixture(t)
	target := makeFormulaCheckedRelationProduct(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, relationExplanationProof(t, f.ast, "formula-negation-sort-target"))
	defer func() { astTestMust(t, target.Close()) }()
	bits, err := f.ast.BitVectorSort(1)
	astTestMust(t, err)
	negative := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{bits}, []Fact{{1}}, []ast.ASTHandle{relationExplanationProof(t, f.ast, "formula-negation-sort-negative")})
	defer func() { astTestMust(t, negative.Close()) }()
	if err = target.FilterByNegation(negative, []int{0}, []int{0}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("equal-domain different sorts accepted", err)
	}
	if rows, err := target.table.Len(); err != nil || rows != 1 {
		t.Fatal("sort rejection mutated target", rows, err)
	}
}

func TestFormulaCheckedRelationProductNegatesExplicitSparse(t *testing.T) {
	f := newTableRelationFixture(t)
	proof := relationExplanationProof(t, f.ast, "formula-negation-target")
	target := makeFormulaCheckedRelationProduct(t, f, []ast.ASTHandle{f.boolean, f.boolean}, []Fact{{0, 0}, {1, 0}, {1, 1}}, proof)
	defer func() { astTestMust(t, target.Close()) }()
	signature, err := NewSignature([]uint64{2})
	astTestMust(t, err)
	sparse, err := NewSparseTable(signature, 0)
	astTestMust(t, err)
	_, err = sparse.Add(Fact{1})
	astTestMust(t, err)
	negative := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, nil, []ast.ASTHandle{relationExplanationProof(t, f.ast, "formula-negation-explicit")})
	astTestMust(t, negative.table.Close())
	negative.table = sparse
	defer func() { astTestMust(t, negative.Close()) }()

	astTestMust(t, target.FilterByNegation(negative, []int{0}, []int{0}))
	if checked, err := target.FormulaChecked(); err != nil || !checked {
		t.Fatal("negation lost formula checker", checked, err)
	}
	rows, err := target.table.Facts()
	astTestMust(t, err)
	if len(rows) != 1 || len(rows[0]) != 2 || rows[0][0] != 0 || rows[0][1] != 0 {
		t.Fatal("mixed formula/sparse negation", rows)
	}
	proofs, err := target.explanation.Data()
	astTestMust(t, err)
	if len(proofs) != 1 || proofs[0] != proof {
		t.Fatal("negation changed whole proof", proofs)
	}
}

func TestFormulaCheckedRelationProductNegatesPlainNullary(t *testing.T) {
	f := newTableRelationFixture(t)
	target := makeFormulaCheckedRelationProduct(t, f, []ast.ASTHandle{f.boolean}, []Fact{{0}, {1}}, relationExplanationProof(t, f.ast, "formula-nullary-negation-target"))
	defer func() { astTestMust(t, target.Close()) }()
	negative := relationExplanationProductWithProofRow(t, f, nil, []Fact{{}}, []ast.ASTHandle{relationExplanationProof(t, f.ast, "formula-nullary-negation-source")})
	defer func() { astTestMust(t, negative.Close()) }()

	astTestMust(t, target.FilterByNegation(negative, nil, nil))
	if checked, err := target.FormulaChecked(); err != nil || !checked {
		t.Fatal("nullary negation lost formula checker", checked, err)
	}
	if rows, err := target.table.Len(); err != nil || rows != 0 {
		t.Fatal("populated nullary negative did not remove target", rows, err)
	}
}
