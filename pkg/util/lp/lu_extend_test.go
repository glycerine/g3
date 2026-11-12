package lp

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"
)

func TestLPLUExtensionIndependentBasisOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewPCG(11937, 8313))
	for world := 0; world < 24; world++ {
		t.Run(fmt.Sprint(world), func(t *testing.T) {
			h := newLPPairHarness(t)
			n := 1 + world%5
			b := lpExactIdentity(n)
			order := rng.Perm(n)
			basis := make([]uint32, n)
			for i := range b {
				basis[i] = uint32(2*i + 1)
				for j := range b[i] {
					v := int64(rng.IntN(5) - 2)
					if j == order[i] {
						v = int64(20*n + i)
					}
					b[i][j].SetFrac64(v, 4)
				}
			}
			a := lpExactLUStatic(t, h, b, basis)
			fa := lpFloatLUSource(t, b, basis)
			s := lpFloatLUSettings(math.Inf(1))
			f, err := NewFloatLU(fa, basis, s)
			lpStaticMust(t, err)
			defer f.Close()
			p, err := NewPairLU(a, basis, s)
			h.must(err)
			defer func() { h.must(p.Close()) }()
			if f.HasDenseSubmatrix() || p.HasDenseSubmatrix() {
				t.Fatal("sparse initial policy")
			}
			value := h.pair("0", "0")
			for round := 0; round < 2; round++ {
				old := n
				added := 1 + (world+round)%2
				n += added
				full := lpExactIdentity(n)
				for i := 0; i < old; i++ {
					for j := 0; j < n; j++ {
						full[i][j].SetInt64(0)
						if j < old {
							full[i][j].Set(b[i][j])
						}
					}
				}
				for i := old; i < n; i++ {
					for j := 0; j < n; j++ {
						v := int64(rng.IntN(5) - 2)
						if i == j {
							v = int64(20*n + i)
						}
						full[i][j].SetFrac64(v, 8)
					}
				}
				for i := 0; i < added; i++ {
					basis = append(basis, a.ColumnCount())
					h.must(a.AddColumn())
					fa.AddColumn()
					h.must(a.AddRow())
					fa.AddRow()
				}
				for i := old; i < n; i++ {
					for j, c := range basis {
						h.must(h.m.SetString(&value.X, full[i][j].RatString()))
						h.must(a.Set(uint32(i), c, &value.X))
						v, _ := full[i][j].Float64()
						lpStaticMust(t, fa.Set(uint32(i), c, v))
					}
				}
				// Include a real nonbasic entry in each new row; it must be skipped.
				h.must(h.m.SetInt64(&value.X, 99))
				h.must(a.Set(uint32(old), 0, &value.X))
				lpStaticMust(t, fa.Set(uint32(old), 0, 99))
				heading := lpLUHeading(a.ColumnCount(), basis)
				fc, err := f.ColumnsToReplaceForAddLastRows(heading)
				lpStaticMust(t, err)
				pc, err := p.ColumnsToReplaceForAddLastRows(heading)
				h.must(err)
				want := map[uint32]struct{}{}
				for j, c := range basis {
					for i := old; i < n; i++ {
						if full[i][j].Sign() != 0 {
							want[c] = struct{}{}
						}
					}
				}
				asSet := func(c *StdUnorderedUintSet) map[uint32]struct{} {
					out := map[uint32]struct{}{}
					for _, k := range c.Keys() {
						out[k] = struct{}{}
					}
					return out
				}
				if !reflect.DeepEqual(asSet(fc), want) || !reflect.DeepEqual(asSet(pc), want) || !reflect.DeepEqual(fc.Keys(), pc.Keys()) {
					t.Fatal("affected set", fc, pc, want)
				}
				lpStaticMust(t, f.AddLastRowsToB(heading, fc))
				h.must(p.AddLastRowsToB(heading, pc))
				if f.GetStatus() != LUOK || p.GetStatus() != LUOK {
					t.Fatal("nonsingular extension degenerated")
				}
				for _, dim := range []uint32{f.Dim, p.Dim, f.U.Dimension(), p.U.Dimension(), f.Q.Size(), p.Q.Size(), f.R.Size(), p.R.Size()} {
					if dim != uint32(n) {
						t.Fatal("extension extent", dim, n)
					}
				}
				if len(f.wExtension.Index) != 0 || len(p.wExtension.Index) != 0 {
					t.Fatal("extension work not consumed")
				}
				x, y := make([]*big.Rat, n), make([]*big.Rat, n)
				for i := range x {
					x[i] = big.NewRat(int64(rng.IntN(9)-4), 11)
					y[i] = big.NewRat(int64(rng.IntN(9)-4), 13)
				}
				lpFloatLUCheckSolutions(t, f, full, x)
				lpExactLUUpdateCheckSolutions(t, h, nil, p, full, x, y)
				lpFloatLUCheckFactorization(t, f, full)
				b = full
			}
			h.must(p.Close())
			lpExactVectorNamedCensus(t, h, lpExactLUStaticCells(a))
		})
	}
}

func TestLPFloatLUExtensionUpstreamSmallLU(t *testing.T) {
	a := NewFloatStaticMatrix(3, 6)
	for _, e := range []struct {
		i, j uint32
		v    float64
	}{{0, 0, 1}, {0, 2, 3.9}, {2, 3, 11}, {0, 5, -3}, {1, 1, 4}, {1, 4, 7}, {2, 0, 1.8}, {2, 2, 5}, {2, 4, 2}, {2, 5, 8}} {
		lpStaticMust(t, a.Set(e.i, e.j, e.v))
	}
	basis := []uint32{0, 1, 3}
	s := lpFloatLUSettings(math.Inf(1))
	s.PivotTolerance = 1e-6
	m, err := NewFloatLU(a, basis, s)
	lpStaticMust(t, err)
	defer m.Close()
	w := NewIndexedFloatVector(3)
	for _, u := range []struct{ entering, leaving uint32 }{{2, 0}, {4, 3}, {5, 1}, {3, 2}} {
		pos := -1
		for i, c := range basis {
			if c == u.leaving {
				pos = i
			}
		}
		if pos < 0 {
			t.Fatal("basis")
		}
		lpStaticMust(t, m.PrepareEntering(u.entering, w))
		lpStaticMust(t, m.ReplaceColumn(0, w, uint32(pos)))
		basis[pos] = u.entering
	}
	a.AddRow()
	a.AddColumn()
	a.AddRow()
	a.AddColumn()
	for j := uint32(0); j < a.ColumnCount(); j++ {
		lpStaticMust(t, a.Set(3, j, float64(j)))
		lpStaticMust(t, a.Set(4, j, float64(j*j)))
	}
	basis = append(basis, 6, 7)
	heading := lpLUHeading(8, basis)
	columns, err := m.ColumnsToReplaceForAddLastRows(heading)
	lpStaticMust(t, err)
	lpStaticMust(t, m.AddLastRowsToB(heading, columns))
	if m.GetStatus() != LUOK {
		t.Fatal("source extension degenerated")
	}
	b := lpExactIdentity(5)
	for i := range b {
		for j, c := range basis {
			v, e := a.Element(uint32(i), c)
			lpStaticMust(t, e)
			b[i][j].SetFloat64(v)
		}
	}
	lpFloatLUCheckSolutions(t, m, b, []*big.Rat{big.NewRat(3, 4), big.NewRat(-5, 2), big.NewRat(7, 8), big.NewRat(2, 3), big.NewRat(-1, 5)})
	lpFloatLUCheckFactorization(t, m, b)
}

func TestLPLUExtensionDenseRefactorQuery(t *testing.T) {
	h := newLPPairHarness(t)
	b := lpExactIdentity(6)
	for i := range b {
		for j := range b[i] {
			if i == j {
				b[i][j].SetInt64(80)
			} else {
				b[i][j].SetInt64(1)
			}
		}
	}
	basis := []uint32{0, 1, 2, 3, 4, 5}
	a := lpExactLUStatic(t, h, b, basis)
	fa := lpFloatLUSource(t, b, basis)
	s := lpFloatLUSettings(0)
	f, err := NewFloatLU(fa, basis, s)
	lpStaticMust(t, err)
	defer f.Close()
	p, err := NewPairLU(a, basis, s)
	h.must(err)
	defer func() { h.must(p.Close()) }()
	if !f.HasDenseSubmatrix() || !p.HasDenseSubmatrix() {
		t.Fatal("actual dense factorization must refactor before extension")
	}
}
