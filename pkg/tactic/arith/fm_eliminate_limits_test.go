package arith

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMEliminateCutoffPreservesBounds(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x := v.Add(false, false)
	lower := fmSubsumptionConstraint(nil, []uint32{x}, []int64{-1}, 1, false)
	upper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 2, false)
	lower.ID, upper.ID = 1, 2
	fmRegister(t, v, lower)
	fmRegister(t, v, upper)
	e, err := NewFMEliminator(store, v, FMEliminationConfig{})
	equationMust(t, err)
	ok, err := e.Try(x)
	equationMust(t, err)
	if ok || lower.Dead || upper.Dead {
		t.Fatal("cutoff attempt changed source bounds")
	}
}

func TestFMEliminateReportsDerivedContradiction(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x := v.Add(false, false)
	lower, err := store.New(nil, []uint32{x}, []unumeral.Rational{fmCRat(-1, 1)}, fmCRatP(-1, 1), false, ast.ASTDependency{})
	equationMust(t, err)
	upper, err := store.New(nil, []uint32{x}, []unumeral.Rational{fmCRat(1, 1)}, fmCRatP(0, 1), false, ast.ASTDependency{})
	equationMust(t, err)
	fmRegister(t, v, lower)
	fmRegister(t, v, upper)
	e, err := NewFMEliminator(store, v, DefaultFMEliminationConfig())
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, e.Close()) })
	ok, err := e.Try(x)
	equationMust(t, err)
	if !ok || !e.Inconsistent {
		t.Fatal("derived contradiction was not retained")
	}
}
