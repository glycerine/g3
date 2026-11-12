package lp

// Source: src/util/lp/lu.h:308-366, retained mpq/pair extension (lar_solver.h:853,902-925).

import (
	"slices"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// addLastRowsOrder is the replacement order for add_last_rows_to_B. The
// source iterates a std::unordered_set<unsigned> (libstdc++ bucket order);
// that order is not reproduced (STL port), so ascending column order is used
// to keep the factorization deterministic (Go map order is random).
func addLastRowsOrder(columns map[uint32]struct{}) []uint32 {
	keys := make([]uint32, 0, len(columns))
	for j := range columns {
		keys = append(keys, j)
	}
	slices.Sort(keys)
	return keys
}

func (m *PairLU) ColumnsToReplaceForAddLastRows(heading []int32) (columns map[uint32]struct{}, err error) {
	if err = m.ready(); err != nil {
		return nil, err
	}
	columns = make(map[uint32]struct{})
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
				columns[c.Column] = struct{}{}
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

func (m *PairLU) AddLastRowsToB(heading []int32, columns map[uint32]struct{}) (err error) {
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
	for _, j := range addLastRowsOrder(columns) {
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
