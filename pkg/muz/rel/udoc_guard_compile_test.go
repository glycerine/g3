package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func addGuardTestFacts(t *testing.T, relation *UDocRelation) {
	t.Helper()
	for x := uint64(0); x < 4; x++ {
		for b := uint64(0); b < 2; b++ {
			if _, err := relation.AddFact([]uint64{x, b}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestUDocCompileAndFilterGuardBitVectorAndBoolean(t *testing.T) {
	docManager, _ := newTestDocManager(t, 3)
	relation, err := NewUDocRelation(docManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	bv, err := m.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkVar(0, bv)
	if err != nil {
		t.Fatal(err)
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.MkVar(1, boolSort)
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.BitVectorNumeralUint64(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	eq, err := m.MkEq(x, two)
	if err != nil {
		t.Fatal(err)
	}
	addGuardTestFacts(t, relation)
	if err = relation.FilterGuard(m, eq); err != nil {
		t.Fatal(err)
	}
	for xval := uint64(0); xval < 4; xval++ {
		for bval := uint64(0); bval < 2; bval++ {
			requireFact(t, relation, []uint64{xval, bval}, xval == 2)
		}
	}

	negated, err := m.MkNot(eq)
	if err != nil {
		t.Fatal(err)
	}
	negRows, err := NewUDocRelation(docManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, negRows)
	addGuardTestFacts(t, negRows)
	if err = negRows.FilterGuard(m, negated); err != nil {
		t.Fatal(err)
	}
	for xval := uint64(0); xval < 4; xval++ {
		for bval := uint64(0); bval < 2; bval++ {
			requireFact(t, negRows, []uint64{xval, bval}, xval != 2)
		}
	}

	one, err := m.BitVectorNumeralUint64(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	oneEq, err := m.MkEq(x, one)
	if err != nil {
		t.Fatal(err)
	}
	disjunction, err := m.MkOr(oneEq, b)
	if err != nil {
		t.Fatal(err)
	}
	orRows, err := NewUDocRelation(docManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, orRows)
	addGuardTestFacts(t, orRows)
	if err = orRows.FilterGuard(m, disjunction); err != nil {
		t.Fatal(err)
	}
	for xval := uint64(0); xval < 4; xval++ {
		for bval := uint64(0); bval < 2; bval++ {
			requireFact(t, orRows, []uint64{xval, bval}, xval == 1 || bval == 1)
		}
	}

	conjunction, err := m.MkAnd(oneEq, b)
	if err != nil {
		t.Fatal(err)
	}
	andRows, err := NewUDocRelation(docManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, andRows)
	addGuardTestFacts(t, andRows)
	if err = andRows.FilterGuard(m, conjunction); err != nil {
		t.Fatal(err)
	}
	for xval := uint64(0); xval < 4; xval++ {
		for bval := uint64(0); bval < 2; bval++ {
			requireFact(t, andRows, []uint64{xval, bval}, xval == 1 && bval == 1)
		}
	}

	falsity, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	emptyRows, err := NewUDocRelation(docManager, []uint32{2, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, emptyRows)
	addGuardTestFacts(t, emptyRows)
	if err = emptyRows.FilterGuard(m, falsity); err != nil {
		t.Fatal(err)
	}
	if emptyRows.NumRecords() != 0 {
		t.Fatalf("false guard retained %d rows", emptyRows.NumRecords())
	}

	part, err := m.MkBitVectorExtract(1, 1, x)
	if err != nil {
		t.Fatal(err)
	}
	bitOne, err := m.BitVectorNumeralUint64(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	partEq, err := m.MkEq(part, bitOne)
	if err != nil {
		t.Fatal(err)
	}
	if err = orRows.FilterGuard(m, partEq); err != nil {
		t.Fatal(err)
	}
	for xval := uint64(0); xval < 4; xval++ {
		for bval := uint64(0); bval < 2; bval++ {
			requireFact(t, orRows, []uint64{xval, bval}, (xval == 1 || bval == 1) && xval >= 2)
		}
	}
}

func TestUDocCompileVariableRangeEqualities(t *testing.T) {
	docManager, _ := newTestDocManager(t, 4)
	relation, err := NewUDocRelation(docManager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	bv, err := m.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkVar(0, bv)
	if err != nil {
		t.Fatal(err)
	}
	y, err := m.MkVar(1, bv)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := m.MkEq(x, y)
	if err != nil {
		t.Fatal(err)
	}
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			if _, e := relation.AddFact([]uint64{left, right}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if err = relation.FilterGuard(m, equal); err != nil {
		t.Fatal(err)
	}
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			requireFact(t, relation, []uint64{left, right}, left == right)
		}
	}
	if err = relation.Reset(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]uint64{{0, 1}, {1, 1}, {2, 2}, {3, 0}} {
		if _, e := relation.AddFact(fact); e != nil {
			t.Fatal(e)
		}
	}
	destination, _ := newTestDocManager(t, 2)
	projected, err := relation.FilterGuardAndProject(m, equal, []uint32{1}, destination)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	for value := uint64(0); value < 4; value++ {
		requireFact(t, projected, []uint64{value}, value == 1 || value == 2)
	}
	if err = relation.Reset(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]uint64{{2, 2}, {2, 1}, {3, 3}} {
		if _, e := relation.AddFact(fact); e != nil {
			t.Fatal(e)
		}
	}
	two, err := m.BitVectorNumeralUint64(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	equalTwo, err := m.MkEq(x, two)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := m.MkAnd(equal, equalTwo)
	if err != nil {
		t.Fatal(err)
	}
	projected, err = relation.FilterGuardAndProject(m, condition, []uint32{1}, destination)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	for value := uint64(0); value < 4; value++ {
		requireFact(t, projected, []uint64{value}, value == 2)
	}
}

func TestUDocCompileEqualityBetweenExtracts(t *testing.T) {
	docManager, _ := newTestDocManager(t, 2)
	relation, err := NewUDocRelation(docManager, []uint32{2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	bv, err := m.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkVar(0, bv)
	if err != nil {
		t.Fatal(err)
	}
	high, err := m.MkBitVectorExtract(1, 1, x)
	if err != nil {
		t.Fatal(err)
	}
	low, err := m.MkBitVectorExtract(0, 0, x)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := m.MkEq(high, low)
	if err != nil {
		t.Fatal(err)
	}
	for value := uint64(0); value < 4; value++ {
		if _, e := relation.AddFact([]uint64{value}); e != nil {
			t.Fatal(e)
		}
	}
	if err = relation.FilterGuard(m, equal); err != nil {
		t.Fatal(err)
	}
	for value := uint64(0); value < 4; value++ {
		requireFact(t, relation, []uint64{value}, value == 0 || value == 3)
	}
}

func TestUDocCompileConcatenatedBitVectorEqualities(t *testing.T) {
	m := docASTManager(t)
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	bv2, err := m.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkVar(0, bv2)
	if err != nil {
		t.Fatal(err)
	}
	y, err := m.MkVar(1, bv2)
	if err != nil {
		t.Fatal(err)
	}
	concat, err := m.MkBitVectorConcat([]ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	constant, err := m.BitVectorNumeralUint64(10, 4)
	if err != nil {
		t.Fatal(err)
	}
	equalConstant, err := m.MkEq(concat, constant)
	if err != nil {
		t.Fatal(err)
	}
	constantManager, _ := newTestDocManager(t, 4)
	constantRows, err := NewUDocRelation(constantManager, []uint32{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, constantRows)
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			if _, e := constantRows.AddFact([]uint64{left, right}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if err = constantRows.FilterGuard(m, equalConstant); err != nil {
		t.Fatal(err)
	}
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			requireFact(t, constantRows, []uint64{left, right}, left == 2 && right == 2)
		}
	}
	if err = constantRows.Reset(); err != nil {
		t.Fatal(err)
	}
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			if _, e := constantRows.AddFact([]uint64{left, right}); e != nil {
				t.Fatal(e)
			}
		}
	}
	destination, _ := newTestDocManager(t, 2)
	projected, err := constantRows.FilterGuardAndProject(m, equalConstant, []uint32{1}, destination)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	for value := uint64(0); value < 4; value++ {
		requireFact(t, projected, []uint64{value}, value == 2)
	}

	bv4, err := m.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	z, err := m.MkVar(2, bv4)
	if err != nil {
		t.Fatal(err)
	}
	equalVariable, err := m.MkEq(concat, z)
	if err != nil {
		t.Fatal(err)
	}
	variableManager, _ := newTestDocManager(t, 8)
	variableRows, err := NewUDocRelation(variableManager, []uint32{2, 2, 4})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, variableRows)
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			for value := uint64(0); value < 16; value++ {
				if _, e := variableRows.AddFact([]uint64{left, right, value}); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	if err = variableRows.FilterGuard(m, equalVariable); err != nil {
		t.Fatal(err)
	}
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			for value := uint64(0); value < 16; value++ {
				requireFact(t, variableRows, []uint64{left, right, value}, value == (left<<2)|right)
			}
		}
	}
	if err = variableRows.Reset(); err != nil {
		t.Fatal(err)
	}
	for _, fact := range [][]uint64{{1, 2, 6}, {1, 2, 7}, {2, 1, 3}} {
		if _, e := variableRows.AddFact(fact); e != nil {
			t.Fatal(e)
		}
	}
	projectManager, _ := newTestDocManager(t, 4)
	projected, err = variableRows.FilterGuardAndProject(m, equalVariable, []uint32{2}, projectManager)
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, projected)
	for left := uint64(0); left < 4; left++ {
		for right := uint64(0); right < 4; right++ {
			requireFact(t, projected, []uint64{left, right}, left == 1 && right == 2)
		}
	}
}

func TestUDocCompileConcatsWithDifferentSegmentBoundaries(t *testing.T) {
	docManager, _ := newTestDocManager(t, 8)
	relation, err := NewUDocRelation(docManager, []uint32{1, 3, 2, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	s1, err := m.BitVectorSort(1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := m.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	s3, err := m.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.MkVar(0, s1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.MkVar(1, s3)
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.MkVar(2, s2)
	if err != nil {
		t.Fatal(err)
	}
	d, err := m.MkVar(3, s2)
	if err != nil {
		t.Fatal(err)
	}
	left, err := m.MkBitVectorConcat([]ast.ASTHandle{a, b})
	if err != nil {
		t.Fatal(err)
	}
	right, err := m.MkBitVectorConcat([]ast.ASTHandle{c, d})
	if err != nil {
		t.Fatal(err)
	}
	condition, err := m.MkEq(left, right)
	if err != nil {
		t.Fatal(err)
	}
	for av := uint64(0); av < 2; av++ {
		for bv := uint64(0); bv < 8; bv++ {
			for cv := uint64(0); cv < 4; cv++ {
				for dv := uint64(0); dv < 4; dv++ {
					if _, e := relation.AddFact([]uint64{av, bv, cv, dv}); e != nil {
						t.Fatal(e)
					}
				}
			}
		}
	}
	if err = relation.FilterGuard(m, condition); err != nil {
		t.Fatal(err)
	}
	for av := uint64(0); av < 2; av++ {
		for bv := uint64(0); bv < 8; bv++ {
			for cv := uint64(0); cv < 4; cv++ {
				for dv := uint64(0); dv < 4; dv++ {
					requireFact(t, relation, []uint64{av, bv, cv, dv}, (av<<3)|bv == (cv<<2)|dv)
				}
			}
		}
	}
}

func TestUDocCompileBooleanIffAndEquality(t *testing.T) {
	docManager, _ := newTestDocManager(t, 2)
	relation, err := NewUDocRelation(docManager, []uint32{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeUDoc(t, relation)
	m := docASTManager(t)
	boolSort, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	left, err := m.MkVar(0, boolSort)
	if err != nil {
		t.Fatal(err)
	}
	right, err := m.MkVar(1, boolSort)
	if err != nil {
		t.Fatal(err)
	}
	equivalent, err := m.MkIff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	for a := uint64(0); a < 2; a++ {
		for b := uint64(0); b < 2; b++ {
			if _, e := relation.AddFact([]uint64{a, b}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if err = relation.FilterGuard(m, equivalent); err != nil {
		t.Fatal(err)
	}
	for a := uint64(0); a < 2; a++ {
		for b := uint64(0); b < 2; b++ {
			requireFact(t, relation, []uint64{a, b}, a == b)
		}
	}
}
