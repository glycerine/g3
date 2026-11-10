package lp

import (
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// C++ next assigns into a caller-owned mpq, which may itself be another
// coefficient in the borrowed container. Subsequent traversal must read that
// changed coefficient, while each assignment still produces an independent value.
func TestLPRationalIteratorsBorrowedOutputAlias(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, kind := range []string{"row", "column", "indexed", "pivot", "term"} {
			name := "pooled/" + kind
			if direct {
				name = "direct/" + kind
			}
			t.Run(name, func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				text := strings.Repeat("1234567890", 20) + "/7"
				p := h.pair(text, "-17/13")
				out := h.pair("0", "0")
				m := NewRationalStaticMatrix(h.m, 2, 2)
				v := NewIndexedRationalVector(h.m, 2)
				term := NewLARTerm(h.m)
				var it RationalLinearIterator
				var first, second *numeral.Rational
				var err error
				switch kind {
				case "row", "column":
					h.must(m.Set(0, 0, &p.X))
					if kind == "row" {
						h.must(m.Set(0, 1, &p.Y))
						it, err = NewRationalRowIterator(m, 0)
						second = &m.Rows[0][1].Value
					} else {
						h.must(m.Set(1, 0, &p.Y))
						it, err = NewRationalColumnIterator(m, 0)
						second = &m.Rows[1][0].Value
					}
					first = &m.Rows[0][0].Value
				case "indexed", "pivot":
					h.must(v.SetValue(&p.X, 0))
					h.must(v.SetValue(&p.Y, 1))
					first, second = &v.Data[0], &v.Data[1]
					if kind == "indexed" {
						it = NewRationalIndexedIterator(v)
					} else {
						it = NewRationalPivotRowIterator(v, 9)
					}
				case "term":
					// Insert column 1 first so libstdc++ order visits column 0 first.
					h.must(term.AddCoefficient(1, &p.Y))
					h.must(term.AddCoefficient(0, &p.X))
					first, _ = term.coefficients.Get2(0)
					second, _ = term.coefficients.Get2(1)
					it, err = NewLARTermIterator(term, 9)
				}
				h.must(err)
				if it.NumeralManager() != h.m {
					t.Fatal("iterator lost its borrowed numeral manager")
				}
				t.Cleanup(func() {
					h.must(it.Close())
					h.must(term.Close())
					h.must(v.Close())
					h.must(m.Close())
				})
				if kind == "pivot" || kind == "term" {
					j, ok, err := it.NextIndex()
					h.must(err)
					if !ok || j != 9 {
						t.Fatal("basis prefix", j, ok)
					}
				}
				j, ok, err := it.Next(second)
				h.must(err)
				if !ok || j != 0 {
					t.Fatal("aliased output", j, ok)
				}
				want, _ := new(big.Rat).SetString(text)
				lpSparseRationalCheck(t, h, second, want)
				// Destroy the source's large value before reading the overwritten
				// later coefficient. A shallow ownership copy fails this check.
				h.must(h.m.SetInt64(first, 23))
				j, ok, err = it.Next(&out.X)
				h.must(err)
				if !ok || j != 1 {
					t.Fatal("live later coefficient", j, ok)
				}
				lpSparseRationalCheck(t, h, &out.X, want)
				h.must(it.Close())
				lpSparseRationalCheck(t, h, second, want)
				h.must(term.Close())
				h.must(v.Close())
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}
