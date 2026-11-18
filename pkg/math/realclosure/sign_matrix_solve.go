package realclosure

// Source: src/math/realclosure/mpz_matrix.cpp:92-303.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// filterColumns preserves the source's ascending column selection. The
// sign-determination caller retains at least one column.
func (m *signMatrix) filterColumns(src *signMatrix, cols []int) error {
	if len(cols) == 0 || len(cols) > src.cols {
		panic("realclosure: invalid sign columns")
	}
	for i, col := range cols {
		if col < 0 || col >= src.cols || i > 0 && col <= cols[i-1] {
			panic("realclosure: sign columns must be strictly ascending")
		}
	}
	if len(cols) == src.cols {
		return m.copyFrom(src)
	}
	selected := signMatrix{z: src.z}
	if err := selected.make(src.z, src.rows, len(cols)); err != nil {
		return err
	}
	for i := 0; i < src.rows; i++ {
		for j, col := range cols {
			if err := selected.z.Copy(selected.at(i, j), src.at(i, col)); err != nil {
				return errors.Join(err, selected.del())
			}
		}
	}
	if m.z == nil {
		m.z = src.z
	}
	return m.move(&selected)
}

func (m *signMatrix) swapRows(i, j int) {
	if i == j {
		return
	}
	for col := 0; col < m.cols; col++ {
		m.cells[i*m.cols+col], m.cells[j*m.cols+col] = m.cells[j*m.cols+col], m.cells[i*m.cols+col]
	}
}

// normalizeRow divides by the row gcd. With a right-hand side, a gcd that
// does not divide that value proves there is no integer solution.
func (m *signMatrix) normalizeRow(row int, rhs *numeral.Int) (ok bool, err error) {
	z := m.z
	var gcd numeral.Int
	defer func() { err = errors.Join(err, z.Del(&gcd)) }()
	for col := 0; col < m.cols; col++ {
		cell := m.at(row, col)
		zero, e := z.IsZero(cell)
		if e != nil {
			return false, e
		}
		if zero {
			continue
		}
		if e = z.GCD(&gcd, cell, &gcd); e != nil {
			return false, e
		}
		one, e := z.IsOne(&gcd)
		if e != nil {
			return false, e
		}
		if one {
			return true, nil
		}
	}
	zero, err := z.IsZero(&gcd)
	if err != nil || zero {
		return err == nil, err
	}
	if rhs != nil {
		var rem numeral.Int
		err = z.Rem(rhs, &gcd, &rem)
		nonzero := false
		if err == nil {
			var isZero bool
			isZero, err = z.IsZero(&rem)
			nonzero = !isZero
		}
		if err = errors.Join(err, z.Del(&rem)); err != nil {
			return false, err
		}
		if nonzero {
			return false, nil
		}
		if err = z.MachineDiv(rhs, &gcd, rhs); err != nil {
			return false, err
		}
	}
	for col := 0; col < m.cols; col++ {
		if err = z.MachineDiv(m.at(row, col), &gcd, m.at(row, col)); err != nil {
			return false, err
		}
	}
	return true, nil
}

// eliminate follows the source's integer LCM elimination below a pivot.
func (m *signMatrix) eliminate(pivotRow, pivotCol int, rhs []numeral.Int) (ok bool, err error) {
	z := m.z
	var lcm, pivotFactor, rowFactor, a, b numeral.Int
	defer func() {
		for _, v := range []*numeral.Int{&b, &a, &rowFactor, &pivotFactor, &lcm} {
			err = errors.Join(err, z.Del(v))
		}
	}()
	pivot := m.at(pivotRow, pivotCol)
	for row := pivotRow + 1; row < m.rows; row++ {
		cell := m.at(row, pivotCol)
		zero, e := z.IsZero(cell)
		if e != nil {
			return false, e
		}
		if zero {
			continue
		}
		if err = z.LCM(pivot, cell, &lcm); err != nil {
			return false, err
		}
		if err = z.MachineDiv(&lcm, pivot, &pivotFactor); err != nil {
			return false, err
		}
		if err = z.MachineDiv(&lcm, cell, &rowFactor); err != nil {
			return false, err
		}
		for col := pivotCol + 1; col < m.cols; col++ {
			if err = z.Mul(&pivotFactor, m.at(pivotRow, col), &a); err != nil {
				return false, err
			}
			if err = z.Mul(&rowFactor, m.at(row, col), &b); err != nil {
				return false, err
			}
			if err = z.Sub(&b, &a, m.at(row, col)); err != nil {
				return false, err
			}
		}
		if rhs != nil {
			if err = z.Mul(&pivotFactor, &rhs[pivotRow], &a); err != nil {
				return false, err
			}
			if err = z.Mul(&rowFactor, &rhs[row], &b); err != nil {
				return false, err
			}
			if err = z.Sub(&b, &a, &rhs[row]); err != nil {
				return false, err
			}
		}
		if err = z.SetInt64(cell, 0); err != nil {
			return false, err
		}
		if rhs != nil {
			normalized, e := m.normalizeRow(row, &rhs[row])
			if e != nil || !normalized {
				return false, e
			}
		} else if _, err = m.normalizeRow(row, nil); err != nil {
			return false, err
		}
	}
	return true, nil
}

// solveInt is the retained sign-determination integer solver. As in the
// source int-output overload, failure leaves the caller's input untouched.
func (m *signMatrix) solveInt(input []int) (result []int, ok bool, err error) {
	if m.rows != m.cols || len(input) != m.cols {
		panic("realclosure: sign system must be square")
	}
	z := m.z
	a := signMatrix{z: z}
	rhs := make([]numeral.Int, m.rows)
	defer func() {
		err = errors.Join(err, a.del())
		for i := range rhs {
			err = errors.Join(err, z.Del(&rhs[i]))
		}
	}()
	if err = a.copyFrom(m); err != nil {
		return nil, false, err
	}
	for i, value := range input {
		if err = z.SetInt64(&rhs[i], int64(value)); err != nil {
			return nil, false, err
		}
	}
	for col := 0; col < a.cols; col++ {
		pivot := col
		for ; pivot < a.rows; pivot++ {
			zero, e := z.IsZero(a.at(pivot, col))
			if e != nil {
				return nil, false, e
			}
			if !zero {
				break
			}
		}
		if pivot == a.rows {
			return nil, false, nil
		}
		a.swapRows(col, pivot)
		rhs[col], rhs[pivot] = rhs[pivot], rhs[col]
		eliminated, e := a.eliminate(col, col, rhs)
		if e != nil || !eliminated {
			return nil, false, e
		}
	}
	result = make([]int, a.rows)
	var remainder, term numeral.Int
	defer func() { err = errors.Join(err, z.Del(&term), z.Del(&remainder)) }()
	for row := a.rows - 1; row >= 0; row-- {
		pivot := a.at(row, row)
		if err = z.Rem(&rhs[row], pivot, &remainder); err != nil {
			return nil, false, err
		}
		zero, e := z.IsZero(&remainder)
		if e != nil || !zero {
			return nil, false, e
		}
		if err = z.MachineDiv(&rhs[row], pivot, &rhs[row]); err != nil {
			return nil, false, err
		}
		fits, e := z.IsInt64(&rhs[row])
		if e != nil || !fits {
			return nil, false, e
		}
		value, e := z.Int64(&rhs[row])
		if e != nil {
			return nil, false, e
		}
		if int64(int(value)) != value {
			return nil, false, nil
		}
		result[row] = int(value)
		for above := 0; above < row; above++ {
			if err = z.Mul(a.at(above, row), &rhs[row], &term); err != nil {
				return nil, false, err
			}
			if err = z.Sub(&rhs[above], &term, &rhs[above]); err != nil {
				return nil, false, err
			}
		}
	}
	return result, true, nil
}

// independentRows selects source rows in original-index priority at each
// pivot, then sorts the selected indices before publishing their values.
func (m *signMatrix) independentRows(src *signMatrix) (chosen []int, err error) {
	z := src.z
	work := signMatrix{z: z}
	defer func() { err = errors.Join(err, work.del()) }()
	if err = work.copyFrom(src); err != nil {
		return nil, err
	}
	rows := make([]int, work.rows)
	for i := range rows {
		rows[i] = i
	}
	chosen = make([]int, 0, work.cols)
	col := 0
	for row := 0; row < work.rows && col < work.cols; row++ {
		pivot := -1
		for candidate := row; candidate < work.rows; candidate++ {
			zero, e := z.IsZero(work.at(candidate, col))
			if e != nil {
				return nil, e
			}
			if !zero && (pivot < 0 || rows[candidate] < rows[pivot]) {
				pivot = candidate
			}
		}
		if pivot < 0 {
			continue
		}
		work.swapRows(row, pivot)
		rows[row], rows[pivot] = rows[pivot], rows[row]
		chosen = append(chosen, rows[row])
		if len(chosen) == work.cols {
			break
		}
		if _, err = work.eliminate(row, col, nil); err != nil {
			return nil, err
		}
		col++
	}
	sort.Ints(chosen)
	if len(chosen) == 0 {
		return chosen, m.del()
	}
	selected := signMatrix{z: z}
	if err = selected.make(z, len(chosen), src.cols); err != nil {
		return nil, err
	}
	for row, sourceRow := range chosen {
		for col := 0; col < src.cols; col++ {
			if err = z.Copy(selected.at(row, col), src.at(sourceRow, col)); err != nil {
				return nil, errors.Join(err, selected.del())
			}
		}
	}
	if m.z == nil {
		m.z = z
	}
	return chosen, m.move(&selected)
}
