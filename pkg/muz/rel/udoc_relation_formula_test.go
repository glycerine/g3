package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocRelationFormulaEmptyAndPopulated(t *testing.T) {
	docManager, _ := newTestDocManager(t, 2)
	astManager := docASTManager(t)
	empty, err := NewUDocRelation(docManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, empty)
	emptyFormula, err := empty.ToFormula(astManager)
	if err != nil {
		t.Fatal("empty relation formula", err)
	}
	t.Cleanup(func() { _ = astManager.DelRef(&emptyFormula) })
	emptyHandle, err := astManager.RefGet(emptyFormula)
	if err != nil || !astManager.IsFalse(emptyHandle) {
		t.Fatal("empty relation must map to false", emptyHandle, err)
	}

	populated, err := NewUDocRelation(docManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, populated)
	if inserted, x := populated.AddFact([]uint64{2}); x != nil || !inserted {
		t.Fatal("add row", inserted, x)
	}
	formula, err := populated.ToFormula(astManager)
	if err != nil {
		t.Fatal("populated relation formula", err)
	}
	t.Cleanup(func() { _ = astManager.DelRef(&formula) })
	handle, err := astManager.RefGet(formula)
	if err != nil || astManager.IsFalse(handle) || astManager.IsTrue(handle) {
		t.Fatal("populated relation must map to a nonconstant formula", handle, err)
	}
	kind, err := astManager.GetKind(handle)
	if err != nil || kind != ast.ASTApp {
		t.Fatal("populated relation formula kind", kind, err)
	}
}
