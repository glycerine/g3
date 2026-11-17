package spacer

// Source: src/muz/spacer/spacer_matrix.h and spacer_matrix.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// spacerMatrix is spacer_matrix: vector<vector<rational>>, rationals of the
// AST manager's rational manager q. del frees them.
type spacerMatrix struct {
	q    *numeral.RationalManager
	rows [][]numeral.Rational
	cols uint32
}

func newSpacerMatrix(q *numeral.RationalManager, rows, columns uint32) *spacerMatrix {
	result := &spacerMatrix{q: q, rows: make([][]numeral.Rational, rows), cols: columns}
	for i := range result.rows {
		result.rows[i] = make([]numeral.Rational, columns)
		for j := range result.rows[i] {
			result.rows[i][j] = numeral.NewRational()
		}
	}
	return result
}

// del frees the matrix's rationals.
func (m *spacerMatrix) del() (err error) {
	if m == nil {
		return nil
	}
	for i := range m.rows {
		for j := range m.rows[i] {
			err = errors.Join(err, m.q.Del(&m.rows[i][j]))
		}
	}
	m.rows = nil
	return err
}

func (m *spacerMatrix) numRows() uint32 { return uint32(len(m.rows)) }
func (m *spacerMatrix) numCols() uint32 { return m.cols }

// get is get(i, j): the matrix's own rational, borrowed until the matrix
// changes or is deleted.
func (m *spacerMatrix) get(row, column uint32) (*numeral.Rational, error) {
	if m == nil || row >= uint32(len(m.rows)) || column >= m.cols {
		return nil, ast.ErrASTDomain
	}
	return &m.rows[row][column], nil
}

// set is set(i, j, v): the matrix keeps a copy of value.
func (m *spacerMatrix) set(row, column uint32, value *numeral.Rational) error {
	if m == nil || row >= uint32(len(m.rows)) || column >= m.cols || value == nil {
		return ast.ErrASTDomain
	}
	return m.q.Copy(&m.rows[row][column], value)
}

// gaussianElimination is perform_gaussian_elimination. It preserves the
// source row-selection and subtraction order, including its non-absolute
// maximal-pivot comparison, its row "swap" (m_matrix[i] =
// m_matrix[max_index]; m_matrix[max_index] = m_matrix[i], which leaves
// both rows equal to the maximal row) and its subtraction loop from row 1.
func (m *spacerMatrix) gaussianElimination() (_ uint32, err error) {
	if m == nil || len(m.rows) == 0 || m.cols == 0 {
		return 0, nil
	}
	q := m.q
	// rational max, pivot, factor and the product temporary.
	maxValue, pivot, factor, product := numeral.NewRational(), numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&product), q.Del(&factor), q.Del(&pivot), q.Del(&maxValue))
	}()
	row, column := uint32(0), uint32(0)
	for row < uint32(len(m.rows)) && column < m.cols {
		if err = q.Copy(&maxValue, &m.rows[row][column]); err != nil {
			return row, err
		}
		maxIndex := row
		for candidate := row + 1; candidate < uint32(len(m.rows)); candidate++ {
			c, e := q.Compare(&maxValue, &m.rows[candidate][column])
			if e != nil {
				return row, e
			}
			if c < 0 {
				if err = q.Copy(&maxValue, &m.rows[candidate][column]); err != nil {
					return row, err
				}
				maxIndex = candidate
			}
		}
		if maxValue.IsZero() {
			column++
			continue
		}
		if maxIndex != row {
			for k := range m.rows[row] {
				if err = q.Copy(&m.rows[row][k], &m.rows[maxIndex][k]); err != nil {
					return row, err
				}
			}
		}
		if err = q.Copy(&pivot, &m.rows[row][column]); err != nil {
			return row, err
		}
		if !pivot.IsOne() {
			for k := range m.rows[row] {
				if err = q.Div(&m.rows[row][k], &pivot, &m.rows[row][k]); err != nil {
					return row, err
				}
			}
		}
		for candidate := uint32(1); candidate < uint32(len(m.rows)); candidate++ {
			if candidate == row {
				continue
			}
			if err = q.Copy(&factor, &m.rows[candidate][column]); err != nil {
				return row, err
			}
			for k := range m.rows[candidate] {
				if err = q.Mul(&factor, &m.rows[row][k], &product); err != nil {
					return row, err
				}
				if err = q.Sub(&m.rows[candidate][k], &product, &m.rows[candidate][k]); err != nil {
					return row, err
				}
			}
		}
		row++
		column++
	}
	return row, nil
}

// normalize is spacer_matrix::normalize: every entry is multiplied by the
// lcm of all denominators.
func (m *spacerMatrix) normalize() (err error) {
	if m == nil {
		return ast.ErrASTDomain
	}
	q := m.q
	den, d := smtOneRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&d), q.Del(&den)) }()
	for i := range m.rows {
		for j := range m.rows[i] {
			if err = q.DenominatorRational(&m.rows[i][j], &d); err != nil {
				return err
			}
			if err = q.LCM(&den, &d, &den); err != nil {
				return err
			}
		}
	}
	for i := range m.rows {
		for j := range m.rows[i] {
			if err = q.Mul(&den, &m.rows[i][j], &m.rows[i][j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func smtOneRational() numeral.Rational {
	return numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
}
