package lp

// Source: src/util/lp/lu.h:308-366, retained double/double dimension extension.

func (m *FloatLU) AdjustDimensionWithMatrixA() (err error) {
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
	m.Q.Resize(n)
	m.R.Resize(n)
	return m.rowEtaWork.Resize(n)
}

// ColumnsToReplaceForAddLastRows returns the unordered set of basis columns
// present in newly appended rows. Native map iteration replaces unordered_set.
func (m *FloatLU) ColumnsToReplaceForAddLastRows(heading []int32) (columns *StdUnorderedUintSet, err error) {
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

func (m *FloatLU) AddLastRowsToB(heading []int32, columns *StdUnorderedUintSet) (err error) {
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

func (m *FloatLU) ReplaceColumnWithOnlyChangeAtLastRows(j, position uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if err = m.PrepareEntering(j, m.wExtension); err != nil {
		return err
	}
	return m.ReplaceColumn(0, m.wExtension, position)
}

func (m *FloatLU) HasDenseSubmatrix() bool {
	for _, tail := range m.Tail {
		if tail.IsDense() {
			return true
		}
	}
	return false
}
