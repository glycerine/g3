package lp

// Source: src/util/lp/lu.h:308-366, retained mpq/pair extension (lar_solver.h:853,902-925).

import (
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *PairLU) ColumnsToReplaceForAddLastRows(heading []int32) (columns *StdUnorderedUintSet, err error) {
	if err = m.ready(); err != nil {
		return nil, err
	}
	columns = NewStdUnorderedUintSet()
	if uint64(m.A.ColumnCount()) != uint64(len(heading)) {
		if err = sourceAssertionFailure(); err != nil {
			return columns, err
		}
	}
	for i := m.U.Dimension(); i < m.A.RowCount(); i++ {
		for _, c := range m.A.Rows[i] {
			if uint64(c.Column) >= uint64(len(heading)) {
				return columns, ErrBounds
			}
			if heading[c.Column] >= 0 {
				columns.Insert(c.Column)
			}
		}
	}
	return columns, nil
}

func (m *PairLU) AdjustDimensionWithMatrixA() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	n := m.A.RowCount()
	if n < m.Dim {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Dim = n
	if err = m.U.Resize(n); err != nil {
		return err
	}
	if err = m.Q.Resize(n); err != nil {
		return err
	}
	if err = m.R.Resize(n); err != nil {
		return err
	}
	return m.rowEtaWork.Resize(n)
}

func (m *PairLU) AddLastRowsToB(heading []int32, columns *StdUnorderedUintSet) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	n := m.A.RowCount()
	if uint64(m.A.ColumnCount()) != uint64(len(heading)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = m.AdjustDimensionWithMatrixA(); err != nil {
		return err
	}
	if err = m.wExtension.Resize(n); err != nil {
		return err
	}
	// !!! ORDER MATTERS: iterate columns.Keys(), the libstdc++
	// std::unordered_set<unsigned> order (std_unordered_set.go, a deliberate
	// user-approved STL-order exception). z3 replaces the columns in that
	// order and breaks on degeneration, so sorting or using a Go map here
	// would make the factorization diverge from z3. Guarded by
	// TestStdUnorderedUintSetMatchesLibstdcxx - keep it green.
	for _, j := range columns.Keys() {
		if uint64(j) >= uint64(len(heading)) {
			return ErrBounds
		}
		if heading[j] < 0 {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if err = m.ReplaceColumnWithOnlyChangeAtLastRows(j, uint32(heading[j])); err != nil {
			return err
		}
		if m.GetStatus() == LUDegenerated {
			break
		}
	}
	return nil
}

func (m *PairLU) ReplaceColumnWithOnlyChangeAtLastRows(j, position uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.PrepareEntering(j, m.wExtension); err != nil {
		return err
	}
	zero := numeral.NewRational()
	return m.ReplaceColumn(&zero, m.wExtension, position)
}

func (m *PairLU) HasDenseSubmatrix() bool {
	for _, tail := range m.Tail {
		if tail.IsDense() {
			return true
		}
	}
	return false
}
