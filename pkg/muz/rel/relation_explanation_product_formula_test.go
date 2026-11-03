package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationExplanationProductToFormulaGlobalProofRowsAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	a := relationExplanationProof(t, f.ast, "relation-formula-a")
	b := relationExplanationProof(t, f.ast, "relation-formula-b")
	sorts := []ast.ASTHandle{f.boolean, f.boolean}
	target := relationExplanationProductWithProofRow(t, f, sorts, []Fact{{0, 0}}, []ast.ASTHandle{a})
	source := relationExplanationProductWithProofRow(t, f, sorts, []Fact{{1, 1}}, []ast.ASTHandle{b})
	astTestMust(t, target.Union(source, nil))
	global, found, err := target.Explanation([]ast.ASTHandle{f.falsehood, f.falsehood})
	astTestMust(t, err)
	if !found {
		t.Fatal("first finite row lacks global explanation")
	}
	requireRelationUnionProof(t, f.ast, global, a, b)
	other, found, err := target.Explanation([]ast.ASTHandle{f.truth, f.truth})
	astTestMust(t, err)
	if !found || other != global {
		t.Fatal("finite rows do not share the global explanation", found, other, global)
	}

	formula, err := target.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	astTestMust(t, source.Close())
	astTestMust(t, target.Close())
	handle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	used, err := ast.NewASTUsedVars(f.ast)
	astTestMust(t, err)
	astTestMust(t, used.Collect(handle))
	if used.MaxFoundVarIndexPlusOne() != 3 {
		t.Fatal("outer finite/proof variable indices", used.MaxFoundVarIndexPlusOne())
	}

	theory := tableRelationTheoryRewriter(t, f.ast)
	evaluate := func(left, right, proof ast.ASTHandle) bool {
		out, makeErr := f.ast.NewRef(ast.ASTHandle{})
		astTestMust(t, makeErr)
		defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
		// Bindings are supplied in descending de-Bruijn index order.
		astTestMust(t, theory.RewriteWithBindings(handle, []ast.ASTHandle{proof, right, left}, out))
		answer, getErr := f.ast.RefGet(out)
		astTestMust(t, getErr)
		return f.ast.IsTrue(answer)
	}
	if !evaluate(f.falsehood, f.falsehood, global) || !evaluate(f.truth, f.truth, global) {
		t.Fatal("formula rejected a stored row with its global proof")
	}
	if evaluate(f.falsehood, f.truth, global) {
		t.Fatal("formula accepted an absent finite row")
	}
	if evaluate(f.falsehood, f.falsehood, a) {
		t.Fatal("formula accepted a pre-union proof instead of global e_union")
	}
}

func TestRelationExplanationProductToFormulaEmptyNullaryZeroAndUndefinedProof(t *testing.T) {
	f := newTableRelationFixture(t)
	empty, err := NewRelationExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	emptyFormula, err := empty.ToFormula()
	astTestMust(t, err)
	emptyHandle, err := f.ast.RefGet(emptyFormula)
	astTestMust(t, err)
	if !f.ast.IsFalse(emptyHandle) {
		t.Fatal("empty whole-explanation product formula is not false")
	}
	astTestMust(t, f.ast.DelRef(&emptyFormula))
	astTestMust(t, empty.Close())

	finiteOnly, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	_, err = finiteOnly.table.Add(Fact{1})
	astTestMust(t, err)
	finiteOnlyFormula, err := finiteOnly.ToFormula()
	astTestMust(t, err)
	finiteOnlyHandle, err := f.ast.RefGet(finiteOnlyFormula)
	astTestMust(t, err)
	if !f.ast.IsFalse(finiteOnlyHandle) {
		t.Fatal("nonempty finite/empty proof product formula is not false")
	}
	astTestMust(t, f.ast.DelRef(&finiteOnlyFormula))
	astTestMust(t, finiteOnly.Close())

	proofOnly := relationExplanationProductWithProofRow(t, f, nil, nil, []ast.ASTHandle{relationExplanationProof(t, f.ast, "relation-formula-proof-only")})
	proofOnlyFormula, err := proofOnly.ToFormula()
	astTestMust(t, err)
	proofOnlyHandle, err := f.ast.RefGet(proofOnlyFormula)
	astTestMust(t, err)
	if !f.ast.IsFalse(proofOnlyHandle) {
		t.Fatal("empty finite/nonempty proof product formula is not false")
	}
	astTestMust(t, f.ast.DelRef(&proofOnlyFormula))
	astTestMust(t, proofOnly.Close())

	unit := relationExplanationProductWithProofRow(t, f, nil, []Fact{{}}, nil)
	unitFormula, err := unit.ToFormula()
	astTestMust(t, err)
	unitHandle, err := f.ast.RefGet(unitFormula)
	astTestMust(t, err)
	if !f.ast.IsTrue(unitHandle) {
		t.Fatal("nullary zero-proof row formula is not true")
	}
	astTestMust(t, unit.Close())
	if _, err = f.ast.RefGet(unitFormula); err != nil {
		t.Fatal("owning unit formula died with product", err)
	}
	astTestMust(t, f.ast.DelRef(&unitFormula))

	proof := relationExplanationProof(t, f.ast, "relation-formula-nullary")
	nullary := relationExplanationProductWithProofRow(t, f, nil, []Fact{{}}, []ast.ASTHandle{proof})
	nullaryFormula, err := nullary.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&nullaryFormula)) }()
	astTestMust(t, nullary.Close())
	handle, err := f.ast.RefGet(nullaryFormula)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	out, err := f.ast.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
	astTestMust(t, theory.RewriteWithBindings(handle, []ast.ASTHandle{proof}, out))
	answer, err := f.ast.RefGet(out)
	astTestMust(t, err)
	if !f.ast.IsTrue(answer) {
		t.Fatal("nullary proof equality rejected its proof")
	}

	undefined := relationExplanationProductWithProofRow(t, f, nil, []Fact{{}}, []ast.ASTHandle{proof})
	defer func() { astTestMust(t, undefined.Close()) }()
	astTestMust(t, undefined.explanation.assignData([]ast.ASTHandle{{}}))
	if bad, formulaErr := undefined.ToFormula(); !errors.Is(formulaErr, ErrTableRelation) || bad != (ast.ASTRef{}) {
		t.Fatal("undefined global explanation formula", bad, formulaErr)
	}
}
