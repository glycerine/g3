package lp

import (
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"
)

func lpRefinementIdentity(t *testing.T, n uint32) *FloatSparseMatrix {
	t.Helper()
	m := newFloatSparseMatrix(n)
	for i := uint32(0); i < n; i++ {
		lpStaticMust(t, m.AddNewElement(i, i, 1))
	}
	return m
}

func TestLPSparseIndexedRefinementIndependentOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	rng := rand.New(rand.NewPCG(123741, 87513))
	improved := 0
	for world := 0; world < 240; world++ {
		t.Run(fmt.Sprint(world), func(t *testing.T) {
			n := uint32(world % 13)
			m := newFloatSparseMatrix(n)
			rows, cols := make([]uint32, n), make([]uint32, n)
			for i := range rows {
				rows[i], cols[i] = uint32(i), uint32(i)
			}
			for k := uint32(0); k < n && n > 1; k++ {
				i, j := rng.Uint32N(n), rng.Uint32N(n)
				if i == j {
					j = (j + 1) % n
				}
				lpStaticMust(t, m.SwapRows(i, j))
				rows[i], rows[j] = rows[j], rows[i]
				i, j = rng.Uint32N(n), rng.Uint32N(n)
				if i == j {
					j = (j + 1) % n
				}
				lpStaticMust(t, m.SwapColumns(i, j))
				cols[i], cols[j] = cols[j], cols[i]
			}
			a := make([][]*big.Rat, n)
			for i := range a {
				a[i] = make([]*big.Rat, n)
				for j := range a[i] {
					a[i][j] = new(big.Rat)
				}
			}
			for _, off := range rng.Perm(int(n * n)) {
				i, j := uint32(off)/n, uint32(off)%n
				if i > j || (i != j && rng.IntN(3) == 0) {
					continue
				}
				x := float64(rng.IntN(19)-9) / 19
				if i == j {
					x = 1
				}
				a[i][j].SetFloat64(x)
				lpStaticMust(t, m.AddNewElement(rows[i], cols[j], x))
			}
			b := make([]*big.Rat, n)
			y := NewIndexedFloatVector(n)
			for _, i := range rng.Perm(int(n)) {
				x := float64(0)
				if rng.IntN(3) == 0 {
					x = float64(rng.IntN(17)-8) / 7
				}
				b[i] = new(big.Rat).SetFloat64(x)
				y.Data[i] = x
				if x != 0 {
					y.Index = append(y.Index, uint32(i))
				}
			}
			want := lpNilpotentSolve(a, b, false)
			single := &IndexedFloatVector{Data: append([]float64(nil), y.Data...), Index: append([]uint32(nil), y.Index...)}
			var active []uint32
			lpStaticMust(t, m.SolveUYIndexed(single, &Settings{DropTolerance: 1e-16}, &active))
			lpStaticMust(t, m.DoubleSolveUYIndexed(y, &Settings{DropTolerance: 1e-16}))
			before, after := new(big.Rat), new(big.Rat)
			for i := range want {
				d := new(big.Rat).Sub(new(big.Rat).SetFloat64(single.Data[i]), want[i])
				before.Add(before, d.Abs(d))
				d.Sub(new(big.Rat).SetFloat64(y.Data[i]), want[i])
				after.Add(after, d.Abs(d))
			}
			if after.Cmp(before) < 0 {
				improved++
			}
			membership := map[uint32]bool{}
			for _, i := range y.Index {
				if membership[i] || y.Data[i] == 0 {
					t.Fatalf("invalid index %v", y.Index)
				}
				membership[i] = true
			}
			for i, v := range y.Data {
				w, _ := want[i].Float64()
				if math.Abs(v-w) > 2e-13*(1+math.Abs(w)) {
					t.Fatalf("coordinate %d: got %.17g want %.17g", i, v, w)
				}
				if (v != 0) != membership[uint32(i)] || m.Processed[i] {
					t.Fatalf("membership/flag at %d", i)
				}
			}
			// Independently multiply the original logical matrix by the answer.
			for i := range a {
				sum := float64(0)
				for j := range a[i] {
					v, _ := a[i][j].Float64()
					sum += float64(v * y.Data[j])
				}
				rhs, _ := b[i].Float64()
				if math.Abs(sum-rhs) > 3e-13*(1+math.Abs(rhs)) {
					t.Fatalf("row %d residual %.17g", i, sum-rhs)
				}
			}
		})
	}
	if improved == 0 {
		t.Fatal("correction never reduced error against the exact oracle")
	}
}

func TestLPSparseIndexedRefinementBranchBoundaryAndCopies(t *testing.T) {
	for _, count := range []int{3, 4, 5} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := lpRefinementIdentity(t, 15)
			y := NewIndexedFloatVector(17)
			for i := count - 1; i >= 0; i-- {
				y.Data[i] = 1e-6
				y.Index = append(y.Index, uint32(i))
			}
			// Hidden dense slots prove copy-construction and branch-specific scans.
			y.Data[14], y.Data[16] = 7, 9
			lpStaticMust(t, m.DoubleSolveUYIndexed(y, &Settings{DropTolerance: 1e-3}))
			var wantIndex []uint32
			for i := 0; i < count; i++ {
				want := float64(0)
				if count >= 4 { // 4*120 == 32*15 must select dense correction.
					want = 1e-6
					wantIndex = append(wantIndex, uint32(i))
				}
				if y.Data[i] != want {
					t.Fatalf("coordinate %d got %g want %g", i, y.Data[i], want)
				}
			}
			wantHidden := float64(7)
			if count >= 4 {
				wantHidden = 14
				wantIndex = append(wantIndex, 14, 16)
			}
			if y.Data[14] != wantHidden || y.Data[16] != 9 || !reflect.DeepEqual(y.Index, wantIndex) {
				t.Fatalf("data/index %v / %v", y.Data, y.Index)
			}
		})
	}
}

func TestLPSparseIndexedRefinementCleanupAndOrder(t *testing.T) {
	m := lpRefinementIdentity(t, 15)
	y := NewIndexedFloatVector(17)
	y.Data[1], y.Data[3], y.Data[7] = 2, 1e-15, -1e-14
	y.Data[16] = 1e-15
	y.Index = []uint32{7, 3, 1}
	lpStaticMust(t, m.DoubleSolveUYIndexed(y, &Settings{}))
	if !reflect.DeepEqual(y.Index, []uint32{7, 1}) || y.Data[3] != 0 || y.Data[16] != 1e-15 {
		t.Fatalf("sparse cleanup/order: %v %v", y.Data, y.Index)
	}
	// Dimension zero selects dense cleanup even for an empty residual index.
	y = &IndexedFloatVector{Data: []float64{1e-15, 1e-14, -1e-14, math.Copysign(0, -1), math.NaN(), math.Inf(1)}}
	lpStaticMust(t, newFloatSparseMatrix(0).DoubleSolveUYIndexed(y, &Settings{}))
	if !reflect.DeepEqual(y.Index, []uint32{1, 2, 4, 5}) || y.Data[0] != 0 || math.Signbit(y.Data[3]) {
		t.Fatalf("dense cleanup/IEEE: %v %v", y.Data, y.Index)
	}
}

func TestLPSparseIndexedRefinementResidualAndErrorPrefixes(t *testing.T) {
	m := lpRefinementIdentity(t, 2)
	original := &IndexedFloatVector{Data: []float64{1, 2}, Index: []uint32{0, 1}}
	y := &IndexedFloatVector{Data: []float64{1 - 1e-15, 2}}
	lpStaticMust(t, m.findErrorUYIndexed(original, y, []uint32{0, 1}))
	if original.Data[0] == 0 || original.Data[0] >= 1e-14 || !reflect.DeepEqual(original.Index, []uint32{0}) {
		t.Fatalf("residual rounded: %v %v", original.Data, original.Index)
	}
	// Failure after the first row leaves its completed residual publication.
	original = &IndexedFloatVector{Data: []float64{1}, Index: []uint32{0}}
	if err := m.findErrorUYIndexed(original, y, []uint32{0, 1}); err != ErrBounds || original.Data[0] == 1 {
		t.Fatalf("residual add prefix: %v %v", err, original.Data)
	}
	if err := m.findErrorUYIndexed(original, y, []uint32{2}); err != ErrBounds {
		t.Fatalf("dot error: %v", err)
	}
	for _, delta := range []*IndexedFloatVector{
		{Data: []float64{3}, Index: []uint32{0, 1}},
		{Data: []float64{3, 4}, Index: []uint32{0, 1}},
	} {
		dst := &IndexedFloatVector{Data: []float64{2}, Index: []uint32{0}}
		if err := addFloatIndexedDelta(delta, dst); err != ErrBounds || dst.Data[0] != 5 {
			t.Fatalf("delta prefix: %v %v", err, dst.Data)
		}
	}
	bad := &IndexedFloatVector{Data: []float64{2, 3}, Index: []uint32{2}}
	if err := m.DoubleSolveUYIndexed(bad, &Settings{}); err != ErrBounds || bad.Data[0] != 2 {
		t.Fatalf("graph failure: %v %v", err, bad.Data)
	}
	// Graph and first solve consume columns; the residual then reaches bad row
	// metadata. The completed first solve must remain published in the caller.
	m.Rows[0] = append(m.Rows[0], FloatIndexedValue{Value: 1, Index: 2})
	bad = &IndexedFloatVector{Data: []float64{1e-6, 0}, Index: []uint32{0}}
	if err := m.DoubleSolveUYIndexed(bad, &Settings{DropTolerance: 1e-3}); err != ErrBounds || bad.Data[0] != 0 || len(bad.Index) != 0 {
		t.Fatalf("residual failure after drop: %v %v %v", err, bad.Data, bad.Index)
	}
	// Dense correction reads a hidden nonzero anchor that the first graph did
	// not visit; its malformed column fails after the first solve's publication.
	m = lpRefinementIdentity(t, 15)
	m.Columns[14].Values = append(m.Columns[14].Values, FloatIndexedValue{Value: 1, Index: 15})
	bad = NewIndexedFloatVector(15)
	bad.Data[14] = 7
	for i := uint32(0); i < 4; i++ {
		bad.Data[i] = 1e-6
		bad.Index = append(bad.Index, i)
	}
	if err := m.DoubleSolveUYIndexed(bad, &Settings{DropTolerance: 1e-3}); err != ErrBounds || len(bad.Index) != 0 || bad.Data[14] != 7 {
		t.Fatalf("dense correction failure: %v %v %v", err, bad.Data, bad.Index)
	}
}
