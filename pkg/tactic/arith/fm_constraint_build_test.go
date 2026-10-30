package arith

import (
	"math/big"
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
	c, e := FMBuildConstraint(store, vars, nil, []uint32{x}, []big.Rat{fmCRat(1, 2)}, big.NewRat(7, 2), true, ast.ASTDependency{})
	equationMust(t, e)
	if c.Strict || c.Coefficients[0].RatString() != "1" || c.Constant.RatString() != "6" {
		t.Fatalf("integer strict normalization = strict %v, %s <= %s", c.Strict, c.Coefficients[0].RatString(), c.Constant.RatString())
	}
}

func TestFMBuildConstraintRetainsRealStrictness(t *testing.T) {
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	vars := NewFMVariableTable()
	x := vars.Add(false, false)
	c, e := FMBuildConstraint(store, vars, nil, []uint32{x}, []big.Rat{fmCRat(1, 1)}, big.NewRat(3, 1), true, ast.ASTDependency{})
	equationMust(t, e)
	if !c.Strict || c.Constant.RatString() != "3" {
		t.Fatal("real strict bound was rounded")
	}
}
