package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTypedUDocFiniteEqualityFilterProjectAndOwnership(t *testing.T) {
	factory, m, bv, finite := typedUDocFixture(t)
	source, err := factory.NewEmpty([]ast.ASTHandle{finite, bv, finite})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	finiteValues := make([]ast.ASTHandle, 5)
	for index := range finiteValues {
		finiteValues[index], err = m.MkDatalogNumeral(uint64(index), finite)
		astTestMust(t, err)
		astTestPin(t, m, finiteValues[index])
	}
	bits := make([]ast.ASTHandle, 8)
	for index := range bits {
		bits[index], err = m.BitVectorNumeralUint64(uint64(index), 3)
		astTestMust(t, err)
		astTestPin(t, m, bits[index])
	}
	for _, row := range [][3]int{{0, 1, 0}, {1, 2, 2}, {3, 4, 3}} {
		inserted, e := source.AddFact([]ast.ASTHandle{finiteValues[row[0]], bits[row[1]], finiteValues[row[2]]})
		astTestMust(t, e)
		if !inserted {
			t.Fatal("source fact was not inserted", row)
		}
	}
	left, err := m.MkVar(0, finite)
	astTestMust(t, err)
	right, err := m.MkVar(2, finite)
	astTestMust(t, err)
	equality, err := m.MkEq(left, right)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	projected, err := source.FilterInterpretedAndProject(equality, []uint32{2})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if sorts := projected.Sorts(); len(sorts) != 2 || sorts[0] != finite || sorts[1] != bv {
		t.Fatal("fused projection lost original column sorts", sorts)
	}
	for a := 0; a < 5; a++ {
		for b := 0; b < 8; b++ {
			found, e := projected.ContainsFact([]ast.ASTHandle{finiteValues[a], bits[b]})
			astTestMust(t, e)
			if found != (a == 0 && b == 1 || a == 3 && b == 4) {
				t.Fatal("finite equality projection tuple", a, b, found)
			}
		}
	}
	if found, e := source.ContainsFact([]ast.ASTHandle{finiteValues[1], bits[2], finiteValues[2]}); e != nil || !found {
		t.Fatal("fused projection modified source tuple", found, e)
	}
	nullary, err := source.FilterInterpretedAndProject(equality, []uint32{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, nullary.Close()) }()
	if found, e := nullary.ContainsFact(nil); e != nil || !found {
		t.Fatal("finite equality nullary projection", found, e)
	}
	// A BV guard shares the fused invocation with the finite equality roots.
	bitColumn, err := m.MkVar(1, bv)
	astTestMust(t, err)
	bitEquality, err := m.MkEq(bitColumn, bits[4])
	astTestMust(t, err)
	condition, err := m.MkAnd(equality, bitEquality)
	astTestMust(t, err)
	filtered, err := source.FilterInterpretedAndProject(condition, []uint32{1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, filtered.Close()) }()
	for value := range finiteValues {
		found, e := filtered.ContainsFact([]ast.ASTHandle{finiteValues[value]})
		astTestMust(t, e)
		if found != (value == 3) {
			t.Fatal("fused finite equality and BV guard", value, found)
		}
	}
	astTestMust(t, source.Close())
	if found, e := projected.ContainsFact([]ast.ASTHandle{finiteValues[3], bits[4]}); e != nil || !found {
		t.Fatal("projected owner did not survive source close", found, e)
	}
}

func TestTypedUDocFiniteEqualityKeepsOrdinaryGuardBoundary(t *testing.T) {
	factory, m, _, finite := typedUDocFixture(t)
	source, err := factory.NewFull([]ast.ASTHandle{finite, finite})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	left, err := m.MkVar(0, finite)
	astTestMust(t, err)
	right, err := m.MkVar(1, finite)
	astTestMust(t, err)
	equality, err := m.MkEq(left, right)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	if err = source.FilterInterpreted(equality); !errors.Is(err, ErrDOC) {
		t.Fatal("ordinary finite equality guard boundary changed", err)
	}
	one, err := m.MkDatalogNumeral(1, finite)
	astTestMust(t, err)
	equalNumeral, err := m.MkEq(left, one)
	astTestMust(t, err)
	if got, e := source.FilterInterpretedAndProject(equalNumeral, []uint32{1}); !errors.Is(e, ErrDOC) || got != nil {
		t.Fatal("unsupported finite numeral guard accepted", got, e)
	}
	otherSort, err := m.MkDatalogFiniteSort(astTestSymbol(t, m, "other-fused-finite"), 6)
	astTestMust(t, err)
	other, err := m.MkVar(0, otherSort)
	astTestMust(t, err)
	otherEquality, err := m.MkEq(other, other)
	astTestMust(t, err)
	if got, e := source.FilterInterpretedAndProject(otherEquality, nil); !errors.Is(e, ast.ErrASTDomain) || got != nil {
		t.Fatal("same-width foreign sort accepted", got, e)
	}
}
