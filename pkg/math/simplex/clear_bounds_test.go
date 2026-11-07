package simplex

import (
	"math/big"
	"testing"
)

func TestClearBoundsKeepsRowsAndValues(t *testing.T) {
	s := NewSolver(nil)
	if _, err := s.AddRationalRow(2, []uint32{2, 0, 1}, []big.Rat{*big.NewRat(1, 1), *big.NewRat(-1, 1), *big.NewRat(-1, 1)}); err != nil {
		t.Fatal(err)
	}
	five := RationalInfinitesimal(big.NewRat(5, 1))
	if err := s.SetLowerWithDependencies(0, five, []BoundDependency{{Literal: 7, Coefficient: *big.NewRat(1, 1)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUpper(1, five); err != nil {
		t.Fatal(err)
	}
	if status, err := s.MakeFeasible(); err != nil || status <= 0 {
		t.Fatal(status, err)
	}
	before := s.Value(2)
	rows := len(s.BasicRows())

	s.ClearBounds()

	if _, found := s.Lower(0); found {
		t.Fatal("lower bound survived")
	}
	if _, found := s.Upper(1); found {
		t.Fatal("upper bound survived")
	}
	if _, known := s.BoundDependencies(0, true); known {
		t.Fatal("explanation survived")
	}
	after := s.Value(2)
	if after.Cmp(&before) != 0 || len(s.BasicRows()) != rows {
		t.Fatal("rows or values changed")
	}
	if status, err := s.MakeFeasible(); err != nil || status <= 0 {
		t.Fatal("unbounded tableau is feasible", status, err)
	}
	// A tighter bound still repairs through the kept row.
	ten := RationalInfinitesimal(big.NewRat(10, 1))
	if err := s.SetLower(2, ten); err != nil {
		t.Fatal(err)
	}
	if status, err := s.MakeFeasible(); err != nil || status <= 0 {
		t.Fatal(status, err)
	}
	value := s.Value(2)
	if value.Cmp(&ten) < 0 {
		t.Fatal("bound after clearing not enforced", value)
	}
}
