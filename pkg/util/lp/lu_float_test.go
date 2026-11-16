package lp

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"
)

func lpFloatLUSettings(density float64) *Settings {
	return &Settings{SimplexStrategy: StrategyLU, PartialPivotingConstant: 10, DensityThreshold: density, DropTolerance: 1e-14}
}

// Build a rectangular source matrix with unrelated nonbasic columns. The
// square oracle follows the caller's basis order, independently of U's maps.
func lpFloatLUSource(t *testing.T, b [][]*big.Rat, basis []uint32) *FloatStaticMatrix {
	return lpFloatLUSourceColumns(t, b, basis, uint32(2*len(b)+1))
}

func lpFloatLUSourceColumns(t *testing.T, b [][]*big.Rat, basis []uint32, columns uint32) *FloatStaticMatrix {
	t.Helper()
	a := NewFloatStaticMatrix(uint32(len(b)), columns)
	for i := range b {
		for j, column := range basis {
			v, _ := b[i][j].Float64()
			lpStaticMust(t, a.Set(uint32(i), column, v))
		}
	}
	return a
}

func lpFloatLUCheckSolutions(t *testing.T, m *FloatLU, b [][]*big.Rat, x []*big.Rat) {
	t.Helper()
	for _, right := range []bool{false, true} {
		rhs := lpExactMatrixVector(b, x, right)
		y := make([]float64, len(x))
		for i := range y {
			y[i], _ = rhs[i].Float64()
		}
		indexed := NewIndexedFloatVector(uint32(len(y)))
		for i, v := range y {
			lpStaticMust(t, indexed.SetValue(v, uint32(i)))
		}
		if right {
			lpStaticMust(t, m.SolveYB(y))
			lpStaticMust(t, m.SolveYBIndexed(indexed))
		} else {
			lpStaticMust(t, m.SolveBy(y))
		}
		for i := range y {
			lpFloatBlockNear(t, y[i], x[i])
			if right {
				lpFloatBlockNear(t, indexed.Data[i], x[i])
			}
		}
		// Verify the recovered solution against the original matrix too.
		for i := range y {
			sum := 0.0
			for j := range y {
				v := b[i][j]
				if right {
					v = b[j][i]
				}
				coefficient, _ := v.Float64()
				sum += float64(coefficient * y[j])
			}
			lpFloatBlockNear(t, sum, rhs[i])
		}
	}
}

func lpFloatLUCheckFactorization(t *testing.T, m *FloatLU, b [][]*big.Rat) {
	t.Helper()
	// Source identity L_k*...*L_0*B = Q*U*R. Form each original
	// column independently; use scalar permutation entries for the right side.
	for j := range b {
		column := make([]float64, len(b))
		for i := range b {
			column[i], _ = b[i][j].Float64()
		}
		for _, tail := range m.Tail {
			lpStaticMust(t, tail.ApplyLeft(column))
		}
		for i := range b {
			sum := 0.0
			for k := range b {
				q, err := m.Q.Entry(uint32(i), uint32(k))
				lpStaticMust(t, err)
				for l := range b {
					r, err := m.R.Entry(uint32(l), uint32(j))
					lpStaticMust(t, err)
					u, err := m.U.Get(uint32(k), uint32(l))
					lpStaticMust(t, err)
					sum += float64(float64(q*r) * u)
				}
			}
			lpFloatBlockNear(t, column[i], new(big.Rat).SetFloat64(sum))
		}
	}
}

func TestLPFloatLUIndependentFactorizationAndSolves(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewPCG(792173, 619313))
	denseCount, rowSwaps, columnSwaps := 0, 0, 0
	for world := 0; world < 180; world++ {
		t.Run(fmt.Sprint(world), func(t *testing.T) {
			n := 1 + world%9
			b := lpExactIdentity(n)
			order := rng.Perm(n)
			basis := make([]uint32, n)
			columns := rng.Perm(2*n + 1)
			x := make([]*big.Rat, n)
			for i := range b {
				basis[i] = uint32(columns[i])
				x[i] = big.NewRat(int64(rng.IntN(13)-6), 4)
				for j := range b[i] {
					v := int64(rng.IntN(5) - 2)
					if world%3 == 0 && rng.IntN(2) == 0 {
						v = 0
					}
					if j == order[i] {
						v = int64(10*n + i)
					}
					b[i][j].SetInt64(v)
				}
			}
			a := lpFloatLUSource(t, b, basis)
			originalBasis := append([]uint32(nil), basis...)
			originalRows := make([][]FloatRowCell, n)
			for i := range a.Rows {
				originalRows[i] = append([]FloatRowCell(nil), a.Rows[i]...)
			}
			// Both runs factor exactly the same B; only the switch policy differs.
			for _, density := range []float64{math.Inf(1), 0} {
				s := lpFloatLUSettings(density)
				m, err := NewFloatLU(a, basis, s)
				lpStaticMust(t, err)
				defer m.Close()
				if m.GetStatus() != LUOK || m.Failure || s.Stats.Factorizations != 1 {
					t.Fatalf("status=%v failure=%v stats=%+v", m.GetStatus(), m.Failure, s.Stats)
				}
				wantDense := density == 0 && n >= 5
				if (m.Dense != nil) != wantDense {
					t.Fatalf("dense switch=%v want %v", m.Dense != nil, wantDense)
				}
				if wantDense {
					denseCount++
					if len(m.Tail) == 0 || m.Tail[len(m.Tail)-1] != m.Dense {
						t.Fatal("dense tail was not published last")
					}
				}
				for i := 0; i < n; i++ {
					q, e := m.Q.Value(uint32(i))
					lpStaticMust(t, e)
					r, e := m.R.Value(uint32(i))
					lpStaticMust(t, e)
					if q != uint32(i) {
						rowSwaps++
					}
					if r != uint32(i) {
						columnSwaps++
					}
					for j := 0; j < n; j++ {
						u, e := m.U.Get(uint32(i), uint32(j))
						lpStaticMust(t, e)
						if (j < i && math.Abs(u) > 1e-12) || (i == j && math.Abs(u-1) > 1e-12) {
							t.Fatalf("U[%d,%d]=%g", i, j, u)
						}
					}
				}
				lpFloatLUCheckFactorization(t, m, b)
				lpFloatLUCheckSolutions(t, m, b, x)
				if m.A != a || !reflect.DeepEqual(basis, originalBasis) || !reflect.DeepEqual(a.Rows, originalRows) {
					t.Fatal("borrowed source/basis changed")
				}
			}
		})
	}
	if denseCount != 100 || rowSwaps == 0 || columnSwaps == 0 {
		t.Fatalf("dense=%d row swaps=%d column swaps=%d", denseCount, rowSwaps, columnSwaps)
	}
}

func TestLPFloatLUSourceSmallInitialBasis(t *testing.T) {
	// Initial construction from src/test/lp.cpp:test_small_lu; update coverage
	// lives in lu_update_float_test.go.
	b := [][]*big.Rat{
		{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(0, 1)},
		{big.NewRat(0, 1), big.NewRat(4, 1), big.NewRat(0, 1)},
		{big.NewRat(18, 10), big.NewRat(0, 1), big.NewRat(11, 1)},
	}
	a := lpFloatLUSourceColumns(t, b, []uint32{0, 1, 3}, 6)
	for _, c := range []struct {
		r, c uint32
		v    float64
	}{{0, 2, 3.9}, {0, 5, -3}, {1, 4, 7}, {2, 2, 5}, {2, 4, 2}, {2, 5, 8}} {
		lpStaticMust(t, a.Set(c.r, c.c, c.v))
	}
	m, err := NewFloatLU(a, []uint32{0, 1, 3}, lpFloatLUSettings(.7))
	lpStaticMust(t, err)
	defer m.Close()
	lpFloatLUCheckSolutions(t, m, b, []*big.Rat{big.NewRat(3, 4), big.NewRat(-2, 1), big.NewRat(5, 2)})
}

func TestLPFloatLUSourceInitialBasisWithHoles(t *testing.T) {
	// src/test/lp.cpp:test_larger_lu_with_holes, before its column update.
	rows := [][]int64{
		{1, 2, 3, 4, 5, 0, 0, 0},
		{0, -6, 7, 8, 9, 0, 0, 0},
		{0, 0, 10, 0, 0, 0, 0, 0},
		{0, 0, 11, -12, 0, 0, 0, 0},
		{0, 0, 13, 14, 15, 0, 0, 0},
		{0, 0, 0, 0, 28, -18, 19, 25},
		{0, 0, 0, 0, 0, 20, -21, 0},
		{0, 0, 0, 0, 0, 22, 23, 24},
	}
	b := lpExactIdentity(8)
	basis := make([]uint32, 8)
	x := make([]*big.Rat, 8)
	for i := range rows {
		basis[i] = uint32(i)
		x[i] = big.NewRat(int64(i-3), 2)
		for j, value := range rows[i] {
			b[i][j].SetInt64(value)
		}
	}
	a := lpFloatLUSourceColumns(t, b, basis, 9)
	lpStaticMust(t, a.Set(0, 8, 99))
	lpStaticMust(t, a.Set(7, 8, 88))
	for _, density := range []float64{.7, 0, math.Inf(1)} {
		m, err := NewFloatLU(a, basis, lpFloatLUSettings(density))
		lpStaticMust(t, err)
		defer m.Close()
		lpFloatLUCheckFactorization(t, m, b)
		lpFloatLUCheckSolutions(t, m, b, x)
	}
}

func TestLPFloatLUDenseSingularPrefix(t *testing.T) {
	b := lpExactIdentity(6)
	basis := []uint32{0, 1, 2, 3, 4, 5}
	for i := range b {
		for j := range b[i] {
			b[i][j].SetInt64(1)
		}
		b[i][i].SetInt64(20)
	}
	for j := range b[5] {
		b[5][j].Set(b[4][j])
	}
	m, err := NewFloatLU(lpFloatLUSource(t, b, basis), basis, lpFloatLUSettings(0))
	lpStaticMust(t, err)
	defer m.Close()
	if m.GetStatus() != LUDegenerated || !m.Failure || m.Dense == nil {
		t.Fatal("dense degeneration not reached")
	}
	for _, tail := range m.Tail {
		if tail == m.Dense {
			t.Fatal("failed dense factorization published its tail")
		}
	}
	// Dense failure retains the earlier sparse factor and the attempted block.
	if len(m.Tail) != 1 {
		t.Fatalf("completed sparse tail prefix %d", len(m.Tail))
	}
}

func TestLPFloatLUDegenerationAndLifetime(t *testing.T) {
	// Dependent nonzero rows fail during eta-driven active shortening. The
	// unsuccessful eta is not appended to the published tail prefix.
	b := [][]*big.Rat{{big.NewRat(2, 1), big.NewRat(2, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}}
	m, err := NewFloatLU(lpFloatLUSource(t, b, []uint32{0, 1}), []uint32{0, 1}, lpFloatLUSettings(math.Inf(1)))
	lpStaticMust(t, err)
	if m.GetStatus() != LUDegenerated || !m.Failure || len(m.Tail) != 0 || m.Dense != nil {
		t.Fatal("failed sparse elimination published an eta")
	}
	lpStaticMust(t, m.Close())
	for _, density := range []float64{math.Inf(1), 0} {
		for _, n := range []int{0, 1, 6} {
			b := lpExactIdentity(n)
			basis := make([]uint32, n)
			for i := range basis {
				basis[i] = uint32(i)
			}
			if n > 0 {
				for j := range b[n-1] {
					b[n-1][j].SetInt64(0)
				}
			}
			m, err := NewFloatLU(lpFloatLUSource(t, b, basis), basis, lpFloatLUSettings(density))
			lpStaticMust(t, err)
			if n > 0 && m.GetStatus() != LUDegenerated {
				t.Fatal("singular basis accepted")
			}
			if n == 0 {
				lpStaticMust(t, m.SolveBy(nil))
				lpStaticMust(t, m.SolveYB(nil))
				lpStaticMust(t, m.SolveYBIndexed(NewIndexedFloatVector(0)))
			}
			m.SetStatus(LUOK)
			if m.GetStatus() != LUOK {
				t.Fatal("status setter")
			}
			for _, c := range []uint32{0, 199, 200, 201, ^uint32(0)} {
				m.RefactorCounter = c
				if m.NeedToRefactor() != (c >= 200) {
					t.Fatal("refactor boundary")
				}
			}
			lpStaticMust(t, m.Close())
			lpStaticMust(t, m.Close())
			if !errors.Is(m.SolveBy(nil), ErrLUClosed) || !errors.Is(m.SolveYB(nil), ErrLUClosed) || !errors.Is(m.SolveYBIndexed(nil), ErrLUClosed) {
				t.Fatal("closed solve")
			}
		}
	}
}

func TestLPFloatLUInputFailureAndStoppedSolve(t *testing.T) {
	for _, basis := range [][]uint32{nil, {0}, {0, 2}, {0, 0}} {
		a := NewFloatStaticMatrix(2, 2)
		lpStaticMust(t, a.Set(0, 0, 1))
		lpStaticMust(t, a.Set(1, 1, 1))
		s := lpFloatLUSettings(1)
		m, err := NewFloatLU(a, basis, s)
		if len(basis) == 2 && basis[1] == 0 && !sourceDebugEnabled {
			lpStaticMust(t, err)
			if m.GetStatus() != LUDegenerated {
				t.Fatal("duplicate basis")
			}
		} else {
			if err == nil || s.Stats.Factorizations != 0 {
				t.Fatalf("basis=%v err=%v stats=%+v", basis, err, s.Stats)
			}
			if !errors.Is(m.SolveBy(nil), ErrLUStopped) {
				t.Fatal("partial constructor stayed usable")
			}
		}
		lpStaticMust(t, m.Close())
	}
	for _, direction := range []int{0, 1, 2, 3} {
		b := lpExactIdentity(2)
		if direction == 3 {
			b[0][0].SetInt64(2)
		}
		m, err := NewFloatLU(lpFloatLUSource(t, b, []uint32{0, 1}), []uint32{0, 1}, lpFloatLUSettings(1))
		lpStaticMust(t, err)
		switch direction {
		case 0, 3:
			err = m.SolveBy(nil)
		case 1:
			err = m.SolveYB(nil)
		case 2:
			v := NewIndexedFloatVector(2)
			v.Index = []uint32{2}
			err = m.SolveYBIndexed(v)
		}
		if !errors.Is(err, ErrBounds) && !(sourceDebugEnabled && errors.Is(err, ErrAssertion)) {
			t.Fatalf("direction %d: %v", direction, err)
		}
		if !errors.Is(m.SolveBy([]float64{1, 2}), ErrLUStopped) {
			t.Fatal("failed solve stayed usable")
		}
		lpStaticMust(t, m.Close())
	}
}

func TestLPFloatLUDensityUnsignedAndDropBoundaries(t *testing.T) {
	s := lpFloatLUSettings(.5)
	if luTooDense(4, 0, 100, s) || luTooDense(5, 0, 12, s) || !luTooDense(5, 0, 13, s) {
		t.Fatal("density boundary")
	}
	if !luTooDense(65536, 0, 0, s) {
		t.Fatal("source uint32 square wrap lost")
	}
	s.DensityThreshold = math.NaN()
	if luTooDense(5, 0, 100, s) {
		t.Fatal("NaN density")
	}
	s = lpFloatLUSettings(1)
	s.DropTolerance = 1e-8
	b := lpExactIdentity(3)
	m, err := NewFloatLU(lpFloatLUSource(t, b, []uint32{0, 1, 2}), []uint32{0, 1, 2}, s)
	lpStaticMust(t, err)
	defer m.Close()
	y := []float64{math.Nextafter(1e-8, 0), 1e-8, -math.Nextafter(1e-8, 0)}
	lpStaticMust(t, m.SolveBy(y))
	if !reflect.DeepEqual(y, []float64{0, 1e-8, 0}) {
		t.Fatalf("drop %v", y)
	}
}

func TestLPFloatLUTinyPivotAndUnitColumnShortening(t *testing.T) {
	for _, value := range []float64{math.Nextafter(1e-12, 0), -math.Nextafter(1e-12, 0), 1e-12, -1e-12} {
		a := NewFloatStaticMatrix(1, 1)
		lpStaticMust(t, a.Set(0, 0, value))
		m, err := NewFloatLU(a, []uint32{0}, lpFloatLUSettings(1))
		lpStaticMust(t, err)
		rejected := math.Abs(value) < 1e-12
		if (m.GetStatus() == LUDegenerated) != rejected || m.Failure != rejected || (len(m.Tail) == 0) != rejected {
			t.Fatalf("source fixed small-pivot boundary: %g, status=%v", value, m.GetStatus())
		}
		lpStaticMust(t, m.Close())
	}
	// Source pivot_the_row returns true for a unit column even when shortening
	// discovers that another column has no remaining active entries.
	a := NewFloatStaticMatrix(2, 2)
	lpStaticMust(t, a.Set(0, 0, 1))
	lpStaticMust(t, a.Set(0, 1, 1))
	u, err := NewFloatSparseBasisMatrix(a, []uint32{0, 1})
	lpStaticMust(t, err)
	lpStaticMust(t, u.PrepareForFactorization())
	m := &FloatLU{Dim: 2, U: u, Q: NewFloatPermutation(2), Settings: lpFloatLUSettings(1)}
	ok, err := m.pivotRow(0)
	lpStaticMust(t, err)
	// Basis columns are copied backwards: column 1 becomes empty first, so
	// shortening returns before decrementing the pivot column's active count.
	if !ok || m.GetStatus() != LUOK || u.active != 1 || len(m.Tail) != 0 {
		t.Fatalf("unit shortening result=%v status=%v active=%d", ok, m.GetStatus(), u.active)
	}
	lpStaticMust(t, m.Close())
}
