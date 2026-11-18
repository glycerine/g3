package arith

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMBuildConstraintClearsIntegerStrictness(t *testing.T) {
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	vars := NewFMVariableTable()
	x := vars.Add(true, false)
	c, e := FMBuildConstraint(store, vars, nil, []uint32{x}, []unumeral.Rational{fmCRat(1, 2)}, fmCRatP(7, 2), true, ast.ASTDependency{})
	equationMust(t, e)
	if c.Strict || fmText(&c.Coefficients[0]) != "1" || fmText(&c.Constant) != "6" {
		t.Fatalf("integer strict normalization = strict %v, %s <= %s", c.Strict, fmText(&c.Coefficients[0]), fmText(&c.Constant))
	}
}

func TestFMBuildConstraintRetainsRealStrictness(t *testing.T) {
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	vars := NewFMVariableTable()
	x := vars.Add(false, false)
	c, e := FMBuildConstraint(store, vars, nil, []uint32{x}, []unumeral.Rational{fmCRat(1, 1)}, fmCRatP(3, 1), true, ast.ASTDependency{})
	equationMust(t, e)
	if !c.Strict || fmText(&c.Constant) != "3" {
		t.Fatal("real strict bound was rounded")
	}
}
