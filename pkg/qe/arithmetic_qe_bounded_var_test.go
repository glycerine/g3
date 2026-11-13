package qe

import (
	"math/big"
	"testing"
)

// mk_bounded_var sizes the fresh bit-vector by halving n until it is zero:
// one bit for n <= 1, otherwise the bit length of n.
func TestArithmeticQEBoundedVarWidths(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	u, err := NewArithmeticQETermUtil(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, u.Close()) }()
	for _, c := range []struct {
		n     int64
		width uint32
	}{{0, 1}, {1, 1}, {2, 2}, {3, 2}, {4, 3}, {7, 3}, {8, 4}, {12, 4}} {
		z, err := u.mkBoundedVar(big.NewRat(c.n, 1))
		fmMust(t, err)
		ref, err := m.NewRef(z)
		fmMust(t, err)
		bits, err := m.GetArg(z, 0)
		fmMust(t, err)
		width, err := m.BitVectorSize(bits)
		fmMust(t, err)
		if width != c.width {
			t.Fatal("bounded variable width", c.n, width, c.width)
		}
		if sort, e := m.GetSort(z); e != nil || sort != f.integer {
			t.Fatal("bounded variable is not an integer", sort, e)
		}
		fmMust(t, m.DelRef(&ref))
	}
}
