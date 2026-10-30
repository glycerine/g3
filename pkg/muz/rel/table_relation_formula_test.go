package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTableRelationToFormulaEmptyAndBooleanRows(t *testing.T) {
	f := newTableRelationFixture(t)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, relation)

	formula, err := relation.ToFormula()
	if err != nil {
		t.Fatal("empty relation formula", err)
	}
	handle, err := f.ast.RefGet(formula)
	if err != nil || !f.ast.IsFalse(handle) {
		t.Fatal("empty relation formula", handle, err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release empty formula", err)
	}

	if _, err = relation.AddFact([]ast.ASTHandle{f.truth}); err != nil {
		t.Fatal("add true row", err)
	}
	formula, err = relation.ToFormula()
	if err != nil {
		t.Fatal("true row formula", err)
	}
	handle, err = f.ast.RefGet(formula)
	if err != nil {
		t.Fatal("read true row formula", err)
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal("expected variable", err)
	}
	variableRef, err := f.ast.NewRef(variable)
	if err != nil {
		t.Fatal("pin expected variable", err)
	}
	assertBooleanRow := func(formula, expectedVariable, value ast.ASTHandle) {
		t.Helper()
		kind, kindErr := f.ast.GetDeclKind(formula)
		arguments, argsErr := f.ast.AppArgumentView(formula)
		if kindErr != nil || argsErr != nil || kind != ast.ASTOpIff || arguments.Count() != 2 {
			t.Fatal("Boolean table row equality", kind, arguments.Count(), kindErr, argsErr)
		}
		first, firstErr := arguments.At(0)
		second, secondErr := arguments.At(1)
		if firstErr != nil || secondErr != nil || !((first == expectedVariable && second == value) || (first == value && second == expectedVariable)) {
			t.Fatal("Boolean table row operands", first, second, value, firstErr, secondErr)
		}
	}
	assertBooleanRow(handle, variable, f.truth)
	if err = f.ast.DelRef(&variableRef); err != nil {
		t.Fatal("release expected variable", err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release true formula", err)
	}

	if err = relation.Reset(); err != nil {
		t.Fatal("reset table", err)
	}
	if _, err = relation.AddFact([]ast.ASTHandle{f.falsehood}); err != nil {
		t.Fatal("add false row", err)
	}
	formula, err = relation.ToFormula()
	if err != nil {
		t.Fatal("false row formula", err)
	}
	handle, err = f.ast.RefGet(formula)
	if err != nil {
		t.Fatal("read false row formula", err)
	}
	falseVariable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal("false row variable", err)
	}
	falseVariableRef, err := f.ast.NewRef(falseVariable)
	if err != nil {
		t.Fatal("pin false row variable", err)
	}
	assertBooleanRow(handle, falseVariable, f.falsehood)
	if err = f.ast.DelRef(&falseVariableRef); err != nil {
		t.Fatal("release false row variable", err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release false formula", err)
	}
}

func TestTableRelationToFormulaZeroArityAndMixedSortRows(t *testing.T) {
	f := newTableRelationFixture(t)
	zero, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal("zero arity relation", err)
	}
	closeTableRelations(t, zero)
	formula, err := zero.ToFormula()
	if err != nil {
		t.Fatal("empty zero arity formula", err)
	}
	handle, err := f.ast.RefGet(formula)
	if err != nil || !f.ast.IsFalse(handle) {
		t.Fatal("empty zero arity formula", handle, err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release zero arity false", err)
	}
	if _, err = zero.AddTableFact(Fact{}); err != nil {
		t.Fatal("add zero arity row", err)
	}
	formula, err = zero.ToFormula()
	if err != nil {
		t.Fatal("true zero arity formula", err)
	}
	handle, err = f.ast.RefGet(formula)
	if err != nil || !f.ast.IsTrue(handle) {
		t.Fatal("true zero arity formula", handle, err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release zero arity true", err)
	}

	mixed, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal("mixed relation", err)
	}
	closeTableRelations(t, mixed)
	if _, err = mixed.AddFact([]ast.ASTHandle{f.truth, f.bv(2)}); err != nil {
		t.Fatal("add mixed row", err)
	}
	formula, err = mixed.ToFormula()
	if err != nil {
		t.Fatal("mixed row formula", err)
	}
	handle, err = f.ast.RefGet(formula)
	if err != nil {
		t.Fatal("read mixed formula", err)
	}
	kind, err := f.ast.GetDeclKind(handle)
	if err != nil || kind != ast.ASTOpAnd {
		t.Fatal("mixed row conjunction", kind, err)
	}
	bitVariable, err := f.ast.MkVar(1, f.bits)
	if err != nil {
		t.Fatal("bit-vector variable", err)
	}
	bitVariableRef, err := f.ast.NewRef(bitVariable)
	if err != nil {
		t.Fatal("pin bit-vector variable", err)
	}
	bitValueRef, err := f.relations.DecodeValue(f.bits, 2)
	if err != nil {
		t.Fatal("decode expected bit-vector value", err)
	}
	bitValue, err := f.ast.RefGet(bitValueRef)
	if err != nil {
		t.Fatal("read expected bit-vector value", err)
	}
	arguments, err := f.ast.AppArgumentView(handle)
	if err != nil || arguments.Count() != 2 {
		t.Fatal("mixed row conjuncts", arguments.Count(), err)
	}
	foundEquality := false
	for index := uint32(0); index < arguments.Count(); index++ {
		conjunct, getErr := arguments.At(index)
		if getErr != nil {
			t.Fatal("read mixed conjunct", getErr)
		}
		conjunctKind, kindErr := f.ast.GetDeclKind(conjunct)
		if kindErr == nil && conjunctKind == ast.ASTOpEq {
			equalityArguments, argsErr := f.ast.AppArgumentView(conjunct)
			if argsErr != nil || equalityArguments.Count() != 2 {
				t.Fatal("bit-vector equality arguments", equalityArguments.Count(), argsErr)
			}
			left, leftErr := equalityArguments.At(0)
			right, rightErr := equalityArguments.At(1)
			foundEquality = leftErr == nil && rightErr == nil && ((left == bitVariable && right == bitValue) || (left == bitValue && right == bitVariable))
		}
	}
	if !foundEquality {
		t.Fatal("mixed formula lost the exact bit-vector value equality")
	}
	if err = f.ast.DelRef(&bitValueRef); err != nil {
		t.Fatal("release expected bit-vector value", err)
	}
	if err = f.ast.DelRef(&bitVariableRef); err != nil {
		t.Fatal("release bit-vector variable", err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal("release mixed formula", err)
	}
}
