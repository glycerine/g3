package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQEModelProjectEliminatesModelEquality(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("qe-project-x", true)
	two := f.pin(f.m.ArithmeticIntegerNumeralText("2"))
	equality := f.pin(f.m.MkEq(x, two))
	owner := apiQECheckedModel(t, f, equality)
	result, err := f.c.QEModelProject(owner.core, []ast.ASTHandle{x}, equality)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	projected, err := f.m.RefGet(result)
	astTestMust(t, err)
	if !f.m.IsTrue(projected) {
		t.Fatal("model equality was not eliminated", projected)
	}
}
