package realclosure

// Source: src/math/realclosure/mpz_matrix.cpp:38-90 and
// src/math/realclosure/realclosure.cpp:1887-1927.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// signMatrix is the mpz_matrix used by real-closure sign determination.
// Cells are mpz values of the store's integer manager z (the source's
// mpz_matrix_manager over qm()); del releases them.
type signMatrix struct {
	z          *numeral.IntegerManager
	rows, cols int
	cells      []numeral.Int
}

// make is mpz_matrix_manager::mk: it releases the previous cells first.
func (m *signMatrix) make(z *numeral.IntegerManager, rows, cols int) error {
	if rows <= 0 || cols <= 0 || z == nil {
		panic("realclosure: empty sign matrix")
	}
	if err := m.del(); err != nil {
		return err
	}
	m.z, m.rows, m.cols = z, rows, cols
	m.cells = make([]numeral.Int, rows*cols)
	return nil
}

// del is mpz_matrix_manager::del.
func (m *signMatrix) del() (err error) {
	for i := range m.cells {
		err = errors.Join(err, m.z.Del(&m.cells[i]))
	}
	z := m.z
	*m = signMatrix{z: z}
	return err
}

// move publishes src into m (releasing m's cells) and leaves src empty.
func (m *signMatrix) move(src *signMatrix) error {
	if m == src {
		return nil
	}
	if err := m.del(); err != nil {
		return err
	}
	*m = *src
	*src = signMatrix{z: m.z}
	return nil
}

func (m *signMatrix) at(row, col int) *numeral.Int {
	return &m.cells[row*m.cols+col]
}

// int64At reads a cell known to hold a sign or small count.
func (m *signMatrix) int64At(row, col int) (int64, error) {
	return m.z.Int64(m.at(row, col))
}

func (m *signMatrix) copyFrom(src *signMatrix) error {
	if m == src {
		return nil
	}
	if err := m.make(src.z, src.rows, src.cols); err != nil {
		return err
	}
	for i := range src.cells {
		if err := m.z.Copy(&m.cells[i], &src.cells[i]); err != nil {
			return err
		}
	}
	return nil
}

// tensorProduct follows mpz_matrix_manager::tensor_product. Building the
// result before publication also permits the destination to alias an input.
func (m *signMatrix) tensorProduct(a, b *signMatrix) error {
	result := signMatrix{z: a.z}
	if err := result.make(a.z, a.rows*b.rows, a.cols*b.cols); err != nil {
		return err
	}
	for i := 0; i < result.rows; i++ {
		for j := 0; j < result.cols; j++ {
			if err := result.z.Mul(a.at(i/b.rows, j/b.cols), b.at(i%b.rows, j%b.cols), result.at(i, j)); err != nil {
				return errors.Join(err, result.del())
			}
		}
	}
	if m.z == nil {
		m.z = a.z
	}
	return m.move(&result)
}

// signDetShape is the shape of imp::mk_sign_det_matrix's M_s: false when
// q's sign cannot distinguish roots.
func signDetShape(equal, positive, negative int) (rows, cols int, ok bool) {
	switch {
	case equal > 0 && positive > 0 && negative == 0,
		equal > 0 && positive == 0 && negative > 0,
		equal == 0 && positive > 0 && negative > 0:
		return 2, 2, true
	case equal > 0 && positive > 0 && negative > 0:
		return 3, 3, true
	}
	return 0, 0, false
}

// signDetMatrix constructs the sign matrix for a derivative's observed zero,
// positive, and negative root counts. A false result means that its sign
// cannot distinguish roots, as in imp::mk_sign_det_matrix. The caller owns
// (and deletes) the matrix.
func signDetMatrix(z *numeral.IntegerManager, equal, positive, negative int) (m signMatrix, ok bool, err error) {
	m.z = z
	rows, cols, ok := signDetShape(equal, positive, negative)
	if !ok {
		return m, false, nil
	}
	if err = m.make(z, rows, cols); err != nil {
		return m, false, err
	}
	switch {
	case equal > 0 && positive > 0 && negative == 0:
		err = m.setSmall(1, 1, 0, 1)
	case equal > 0 && positive == 0 && negative > 0:
		err = m.setSmall(1, 1, 0, -1)
	case equal == 0 && positive > 0 && negative > 0:
		err = m.setSmall(1, 1, 1, -1)
	default:
		err = m.setSmall(1, 1, 1, 0, 1, -1, 0, 1, 1)
	}
	if err != nil {
		return m, false, errors.Join(err, m.del())
	}
	return m, true, nil
}

func (m *signMatrix) setSmall(values ...int64) error {
	for i, v := range values {
		if err := m.z.SetInt64(&m.cells[i], v); err != nil {
			return err
		}
	}
	return nil
}
