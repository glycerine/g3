package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationUnionAndWidenEndpointOpenness(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
		astTestMust(t, cause)
		astTestPin(t, m, h)
		return h
	}
	x := own(m.MkVar(0, real))
	zero := own(m.ArithmeticRealNumeralText("0"))
	two := own(m.ArithmeticRealNumeralText("2"))
	openLowerClosedUpper := func() *IntervalRelation {
		r := NewIntervalFull(1)
		astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticGT(x, zero))))
		astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLE(x, two))))
		return r
	}
	closedLowerOpenUpper := func() *IntervalRelation {
		r := NewIntervalFull(1)
		astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticGE(x, zero))))
		astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLT(x, two))))
		return r
	}

	t.Run("union-hull", func(t *testing.T) {
		target := openLowerClosedUpper()
		changed, cause := target.Union(closedLowerOpenUpper(), false)
		astTestMust(t, cause)
		lower, lowerOpen, upper, upperOpen, cause := target.Bounds(0)
		astTestMust(t, cause)
		if !changed || lower == nil || lower.Sign() != 0 || lowerOpen || upper == nil || upper.RatString() != "2" || upperOpen {
			t.Fatal("source union endpoint hull", changed, lower, lowerOpen, upper, upperOpen)
		}
	})

	t.Run("widen-equal-endpoints", func(t *testing.T) {
		target := openLowerClosedUpper()
		changed, cause := target.Union(closedLowerOpenUpper(), true)
		astTestMust(t, cause)
		lower, _, upper, _, cause := target.Bounds(0)
		astTestMust(t, cause)
		if !changed || lower != nil || upper != nil {
			t.Fatal("source widening should expand both equal open/closed endpoints", changed, lower, upper)
		}
	})
}
