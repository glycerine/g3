package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveRelationToFormulaUsesOuterColumnsAndLeavesIgnoredFree(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits, f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := NewSieveRelation(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = relation.AddFact(Fact{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	formula, err := relation.ToFormula(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	formulaHandle, err := f.ast.RefGet(formula)
	if err != nil {
		t.Fatal("caller formula did not survive relation disposal", err)
	}
	theory := tableRelationTheoryRewriter(t, f.ast)
	check := func(first, ignored, third ast.ASTHandle, want bool) {
		t.Helper()
		out, newErr := f.ast.NewRef(ast.ASTHandle{})
		if newErr != nil {
			t.Fatal(newErr)
		}
		bindings := []ast.ASTHandle{third, ignored, first}
		if newErr = theory.RewriteWithBindings(formulaHandle, bindings, out); newErr != nil {
			t.Fatal(newErr)
		}
		got, getErr := f.ast.RefGet(out)
		isTrue, isFalse := f.ast.IsTrue(got), f.ast.IsFalse(got)
		newErr = errors.Join(getErr, f.ast.DelRef(&out))
		if newErr != nil {
			t.Fatal(newErr)
		}
		if isTrue != want || isFalse == want {
			t.Fatal("formula evaluation", want, got)
		}
	}
	check(f.truth, f.bv(0), f.falsehood, true)
	check(f.truth, f.bv(3), f.falsehood, true)
	check(f.falsehood, f.bv(0), f.falsehood, false)
	check(f.truth, f.bv(0), f.truth, false)
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal(err)
	}
}

func TestSieveRelationToFormulaEmptyFullAndShapeValidation(t *testing.T) {
	f := newTableRelationFixture(t)
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := NewSieveRelation(signature, []bool{true, false})
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	formula, err := empty.ToFormula(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := f.ast.RefGet(formula)
	if err != nil || !f.ast.IsFalse(handle) {
		t.Fatal("empty sieve formula", handle, err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal(err)
	}
	full, err := NewFullSieve(signature)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	formula, err = full.ToFormula(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	handle, err = f.ast.RefGet(formula)
	if err != nil || !f.ast.IsTrue(handle) {
		t.Fatal("full sieve formula", handle, err)
	}
	if err = f.ast.DelRef(&formula); err != nil {
		t.Fatal(err)
	}
	if _, err = empty.ToFormula(f.relations, sorts[:1]); !errors.Is(err, ErrSieve) {
		t.Fatal("sort arity mismatch", err)
	}
	if _, err = empty.ToFormula(f.relations, []ast.ASTHandle{f.bits, f.boolean}); !errors.Is(err, ErrSieve) {
		t.Fatal("sort domain mismatch", err)
	}
}
