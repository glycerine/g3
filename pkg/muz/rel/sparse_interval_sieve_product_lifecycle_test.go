package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalSieveProductFormulaCloneToAndReset(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sourceManager, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	destinationManager, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, destinationManager.Close()) }()
	sorts := []ast.ASTHandle{f.boolean, integer}
	source, err := NewSparseIntervalSieveProduct(sourceManager, sorts, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	two, four, five := numeral("2"), numeral("4"), numeral("5")
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, two}))
	clone, err := source.CloneTo(destinationManager)
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, four}))
	found, err := clone.ContainsFact([]ast.ASTHandle{f.truth, two})
	if err != nil || found {
		t.Fatal("clone observed later source finite mutation", found, err)
	}
	astTestMust(t, source.Close())
	astTestMust(t, sourceManager.Close())
	found, err = clone.ContainsFact([]ast.ASTHandle{f.falsehood, two})
	if err != nil || !found {
		t.Fatal("destination clone borrowed source manager lifecycle", found, err)
	}

	astTestMust(t, clone.AddFact([]ast.ASTHandle{f.truth, four}))
	formula, err := clone.ToFormula()
	astTestMust(t, err)
	formulaHandle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	evaluate := func(boolean, arithmetic ast.ASTHandle) bool {
		out, cause := f.ast.NewRef(ast.ASTHandle{})
		astTestMust(t, cause)
		defer func() { astTestMust(t, f.ast.DelRef(&out)) }()
		// Standard-order beta bindings place variable zero last.
		astTestMust(t, theory.RewriteWithBindings(formulaHandle, []ast.ASTHandle{arithmetic, boolean}, out))
		answer, cause := f.ast.RefGet(out)
		astTestMust(t, cause)
		return f.ast.IsTrue(answer)
	}
	if !evaluate(f.falsehood, four) || !evaluate(f.truth, two) {
		t.Fatal("product formula lost independent-component cross products")
	}
	if evaluate(f.falsehood, five) {
		t.Fatal("product formula omitted interval conjunct")
	}
	astTestMust(t, f.ast.DelRef(&formula))

	astTestMust(t, clone.Reset())
	empty, err := clone.Empty()
	if err != nil || !empty {
		t.Fatal("reset did not empty both product components", empty, err)
	}
	formula, err = clone.ToFormula()
	astTestMust(t, err)
	emptyFormula, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	if !f.ast.IsFalse(emptyFormula) {
		t.Fatal("reset product formula is not false")
	}
	astTestMust(t, f.ast.DelRef(&formula))
}

func TestSparseIntervalSieveProductCloneToRejectsForeignASTManager(t *testing.T) {
	f, integer, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	p, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	foreign := newTableRelationFixture(t)
	if clone, cause := p.CloneTo(foreign.relations); cause == nil || clone != nil {
		t.Fatal("clone accepted a foreign AST manager", clone, cause)
	}
}
