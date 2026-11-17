package smtcore

import (
	"math/big"
	"testing"
)

func TestSMTArithmeticRowHolesPositionsAndLCM(t *testing.T) {
	row := newSMTArithmeticRow()
	for variable, coefficient := range []*big.Rat{big.NewRat(1, 2), big.NewRat(2, 3), big.NewRat(3, 5)} {
		index, entry := row.addEntry()
		if index != variable {
			t.Fatal("append index", index, variable)
		}
		entry.variable = SMTTheoryVar(variable)
		entry.coefficient = *testQR(coefficient)
		entry.columnIndex = 0
	}
	if row.size != 3 || !testDenLCM(&row, 30) || row.indexOf(1) != 1 {
		t.Fatal("row metadata")
	}
	if err := row.deleteEntry(testQ(), 1); err != nil {
		t.Fatal(err)
	}
	// Source row::del_row_entry never makes the hole the free-list head,
	// so the next entry is appended rather than placed in the hole.
	index, entry := row.addEntry()
	if index != 3 || row.firstFree != -1 || !row.entries[1].dead() {
		t.Fatal("row reused a hole before compress", index)
	}
	entry.variable = 3
	entry.coefficient = smtQInt(7)
	entry.columnIndex = 0
	positions := []int{-1, -1, -1, -1}
	if err := row.saveVariablePositions(positions); err != nil {
		t.Fatal(err)
	}
	if positions[0] != 0 || positions[1] != -1 || positions[2] != 2 || positions[3] != 3 {
		t.Fatal("saved positions", positions)
	}
	if err := row.resetVariablePositions(positions); err != nil {
		t.Fatal(err)
	}
	for _, variable := range []int{0, 2, 3} {
		if positions[variable] != -1 {
			t.Fatal("position reset", variable)
		}
	}
	row.reset(testQ())
	if row.size != 0 || row.base != SMTNullTheoryVar || row.firstFree != -1 || len(row.entries) != 0 {
		t.Fatal("row reset")
	}
}

func TestSMTArithmeticRowAndColumnCompressionRepairCrossIndices(t *testing.T) {
	rows := []smtArithmeticRow{newSMTArithmeticRow(), newSMTArithmeticRow()}
	columns := []smtArithmeticColumn{newSMTArithmeticColumn(), newSMTArithmeticColumn()}
	add := func(rowID int, variable SMTTheoryVar) (int, int) {
		ri, re := rows[rowID].addEntry()
		ci, ce := columns[variable].addEntry()
		re.variable, re.columnIndex = variable, ci
		re.coefficient = smtQInt(1)
		ce.rowID, ce.rowIndex = rowID, ri
		return ri, ci
	}
	_, firstColumn := add(0, 0)
	rowHole, columnHole := add(0, 1)
	movedRow, movedColumn := add(1, 1)
	if err := rows[0].deleteEntry(testQ(), rowHole); err != nil {
		t.Fatal(err)
	}
	if err := columns[1].deleteEntry(columnHole); err != nil {
		t.Fatal(err)
	}
	if err := rows[0].compress(columns); err != nil {
		t.Fatal(err)
	}
	if len(rows[0].entries) != 1 || columns[0].entries[firstColumn].rowIndex != 0 {
		t.Fatal("row compression cross index")
	}
	if err := columns[1].compress(rows); err != nil {
		t.Fatal(err)
	}
	if len(columns[1].entries) != 1 || rows[1].entries[movedRow].columnIndex != 0 || movedColumn != 1 {
		t.Fatal("column compression cross index")
	}
	if first := columns[1].firstEntry(); first == nil || first.rowID != 1 {
		t.Fatal("first column entry")
	}
}

func TestSMTArithmeticColumnHoleReuseAndSingletonCompression(t *testing.T) {
	rows := []smtArithmeticRow{newSMTArithmeticRow()}
	column := newSMTArithmeticColumn()
	for index := 0; index < 3; index++ {
		ri, re := rows[0].addEntry()
		ci, ce := column.addEntry()
		re.variable, re.columnIndex = 0, ci
		ce.rowID, ce.rowIndex = 0, ri
	}
	if err := column.deleteEntry(0); err != nil {
		t.Fatal(err)
	}
	if err := column.deleteEntry(1); err != nil {
		t.Fatal(err)
	}
	if err := column.compressSingleton(rows, 2); err != nil {
		t.Fatal(err)
	}
	if len(column.entries) != 1 || rows[0].entries[2].columnIndex != 0 {
		t.Fatal("singleton compression")
	}
	if err := column.deleteEntry(0); err != nil {
		t.Fatal(err)
	}
	index, entry := column.addEntry()
	if index != 0 || !entry.dead() {
		t.Fatal("column hole reuse", index)
	}
	column.reset()
	if column.size != 0 || column.firstFree != -1 || len(column.entries) != 0 {
		t.Fatal("column reset")
	}
}

func TestSMTArithmeticTableauAddRowEntryAndDuplicateMerge(t *testing.T) {
	var tableau smtArithmeticTableau
	first, second := tableau.addVariable(), tableau.addVariable()
	row := tableau.makeRow()
	if err := tableau.addRowEntry(testQ(), row, testQR(big.NewRat(1, 1)), first, false); err == nil {
		t.Fatal("entry accepted without row scope")
	}
	tableau.pushRowScope()
	if err := tableau.addRowEntry(testQ(), row, testQR(big.NewRat(2, 1)), first, false); err != nil {
		t.Fatal(err)
	}
	if err := tableau.addRowEntry(testQ(), row, testQR(big.NewRat(3, 1)), first, true); err != nil {
		t.Fatal(err)
	}
	if err := tableau.addRowEntry(testQ(), row, testQR(big.NewRat(5, 2)), second, false); err != nil {
		t.Fatal(err)
	}
	if tableau.rows[row].size != 2 || tableau.rows[row].entries[tableau.rows[row].indexOf(first)].coefficient.IsMinusOne() == false {
		t.Fatal("duplicate coefficient merge")
	}
	for variable := range tableau.columns {
		if tableau.columns[variable].size != 1 {
			t.Fatal("duplicate allocated extra column entry", variable)
		}
		entry := tableau.columns[variable].firstEntry()
		if entry == nil || tableau.rows[entry.rowID].entries[entry.rowIndex].variable != SMTTheoryVar(variable) {
			t.Fatal("tableau cross index", variable)
		}
	}
	if !tableau.popRowScope() || tableau.popRowScope() {
		t.Fatal("row scope stack")
	}
}

// testDenLCM reports row.get_denominators_lcm() == want.
func testDenLCM(row *smtArithmeticRow, want int64) bool {
	lcm := smtQZero()
	if err := row.denominatorLCM(testQ(), &lcm); err != nil {
		return false
	}
	return testBig(&lcm).Cmp(big.NewRat(want, 1)) == 0
}
