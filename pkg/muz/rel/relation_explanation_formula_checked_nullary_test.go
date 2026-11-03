package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedRelationProductJoinsPlainNullaryIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	leftProof := relationExplanationProof(t, f.ast, "formula-nullary-left")
	rightProof := relationExplanationProof(t, f.ast, "formula-nullary-right")
	left := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, []ast.ASTHandle{leftProof})
	plain, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(plain, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	if _, err = checked.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, left.table.Close())
	left.table = checked

	identity := relationExplanationProductWithProofRow(t, f, nil, []Fact{{}}, []ast.ASTHandle{rightProof})
	joined, err := left.Join(identity, nil, nil)
	astTestMust(t, err)
	astTestMust(t, left.Close())
	astTestMust(t, identity.Close())
	defer func() { astTestMust(t, joined.Close()) }()
	if checked, err := joined.FormulaChecked(); err != nil || !checked {
		t.Fatal("nullary identity lost formula checker", checked, err)
	}
	rows, err := joined.table.Facts()
	astTestMust(t, err)
	if len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != 1 {
		t.Fatal("nullary identity changed finite rows", rows)
	}
	proofs, err := joined.explanation.Data()
	astTestMust(t, err)
	if len(proofs) != 2 || proofs[0] != leftProof || proofs[1] != rightProof {
		t.Fatal("nullary identity lost independent proof rows", proofs)
	}
}
