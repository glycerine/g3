package realclosure

import (
	"sync"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var signTestZOnce struct {
	sync.Once
	z *numeral.IntegerManager
}

// signTestZ is the tests' mpz manager for sign matrices without a store.
func signTestZ() *numeral.IntegerManager {
	signTestZOnce.Do(func() {
		q, err := numeral.NewRationalManager(numeral.RationalConfig{})
		if err != nil {
			panic(err)
		}
		signTestZOnce.z = q.Z
	})
	return signTestZOnce.z
}

func mustSign(err error) {
	if err != nil {
		panic(err)
	}
}

func newTestSignDetermination(numRoots int) *signDetermination {
	s, err := newSignDetermination(signTestZ(), numRoots)
	mustSign(err)
	return s
}

func testSignDetMatrix(equal, positive, negative int) (signMatrix, bool) {
	m, ok, err := signDetMatrix(signTestZ(), equal, positive, negative)
	mustSign(err)
	return m, ok
}

func (m *signMatrix) makeT(rows, cols int) {
	z := m.z
	if z == nil {
		z = signTestZ()
	}
	mustSign(m.make(z, rows, cols))
}

func (m *signMatrix) solveIntT(input []int) ([]int, bool) {
	result, ok, err := m.solveInt(input)
	mustSign(err)
	return result, ok
}

func (m *signMatrix) independentRowsT(src *signMatrix) []int {
	rows, err := m.independentRows(src)
	mustSign(err)
	return rows
}

func (s *signDetermination) advanceT(equal, positive, negative int, expandedTarski []int) (bool, bool) {
	kept, allOne, err := s.advance(equal, positive, negative, expandedTarski)
	mustSign(err)
	return kept, allOne
}

func (m *signMatrix) cellString(row, col int) string {
	text, err := m.z.String(m.at(row, col))
	mustSign(err)
	return text
}

func (m *signMatrix) cellInt64(row, col int) int64 {
	v, err := m.int64At(row, col)
	mustSign(err)
	return v
}

func (m *signMatrix) setCell(row, col int, v int64) { mustSign(m.z.SetInt64(m.at(row, col), v)) }

// setCellPow2 sets a cell to 2^k (a multi-limb mpz for k >= 31).
func (m *signMatrix) setCellPow2(row, col int, k int) {
	mustSign(m.z.SetInt64(m.at(row, col), 1))
	var two numeral.Int
	mustSign(m.z.SetInt64(&two, 2))
	for i := 0; i < k; i++ {
		mustSign(m.z.Mul(m.at(row, col), &two, m.at(row, col)))
	}
}

func (m *signMatrix) isPow2Cell(row, col int, k int) bool {
	var want, two numeral.Int
	mustSign(m.z.SetInt64(&want, 1))
	mustSign(m.z.SetInt64(&two, 2))
	for i := 0; i < k; i++ {
		mustSign(m.z.Mul(&want, &two, &want))
	}
	c, err := m.z.Compare(m.at(row, col), &want)
	mustSign(err)
	mustSign(m.z.Del(&want))
	return c == 0
}

func cellsEqual(a *signMatrix, ar, ac int, b *signMatrix, br, bc int) bool {
	c, err := a.z.Compare(a.at(ar, ac), b.at(br, bc))
	mustSign(err)
	return c == 0
}
