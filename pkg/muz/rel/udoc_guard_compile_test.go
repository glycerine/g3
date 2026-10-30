package rel

import "testing"

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
