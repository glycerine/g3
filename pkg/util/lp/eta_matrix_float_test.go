package lp

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
)

func lpFloatIdentity(n int) [][]float64 {
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n)
		m[i][i] = 1
	}
	return m
}
func lpFloatMatrixProduct(a, b [][]float64) [][]float64 {
	out := lpFloatIdentity(len(a))
	for i := range out {
		for j := range out[i] {
			out[i][j] = 0
			for k := range b {
				out[i][j] += float64(a[i][k] * b[k][j])
			}
		}
	}
	return out
}
func lpFloatMatrixTranspose(a [][]float64) [][]float64 {
	out := lpFloatIdentity(len(a))
	for i := range out {
		for j := range out[i] {
			out[i][j] = a[j][i]
		}
	}
	return out
}
func lpFloatMatrixVector(m [][]float64, v []float64, right bool) []float64 {
	out := make([]float64, len(v))
	for i := range out {
		for j := range v {
			coefficient := m[i][j]
			if right {
				coefficient = m[j][i]
			}
			out[i] += float64(coefficient * v[j])
		}
	}
	return out
}
func lpFloatCheckVector(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatal(len(got), len(want))
	}
	for i, v := range got {
		if math.IsNaN(v) || v != want[i] {
			t.Fatal(i, v, want[i], got, want)
		}
	}
}
func lpFloatSparseFromDense(v []float64) *IndexedFloatVector {
	out := NewIndexedFloatVector(uint32(len(v)))
	copy(out.Data, v)
	for i := len(v); i > 0; {
		i--
		if v[i] != 0 {
			out.Index = append(out.Index, uint32(i))
		}
	}
	return out
}
func lpFloatCheckSparse(t *testing.T, v *IndexedFloatVector, want []float64) {
	t.Helper()
	lpFloatCheckVector(t, v.Data, want)
	seen := make(map[uint32]bool)
	for _, i := range v.Index {
		if uint64(i) >= uint64(len(want)) || want[i] == 0 || seen[i] {
			t.Fatal("sparse index", i, v.Index, want)
		}
		seen[i] = true
	}
	for i, value := range want {
		if (value != 0) != seen[uint32(i)] {
			t.Fatal("missing index", i, v.Index, want)
		}
	}
}
func TestLPFloatEtaIndependentMatrixProducts(t *testing.T) {
	s, err := NewSettings()
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(897, 2026))
	for world := 0; world < 2000; world++ {
		n := random.IntN(6) + 1
		pivot := random.IntN(n)
		eta := NewFloatEtaMatrix(uint32(pivot), uint32(n))
		row := NewFloatRowEtaMatrix(0, uint32(pivot), uint32(n))
		diagonal := []float64{0.5, -0.5, 2, -2}[random.IntN(4)]
		if !eta.SetDiagonal(diagonal) {
			t.Fatal(diagonal)
		}
		em, rm := lpFloatIdentity(n), lpFloatIdentity(n)
		em[pivot][pivot] = 1 / diagonal
		for j := 0; j < n; j++ {
			if j == pivot {
				continue
			}
			coefficient := float64(random.IntN(9)-4) / 4
			if err := eta.Append(uint32(j), coefficient); err != nil {
				t.Fatal(err)
			}
			if err := row.Append(uint32(j), coefficient); err != nil {
				t.Fatal(err)
			}
			em[j][pivot] = coefficient
			rm[pivot][j] = coefficient
		}
		vector := make([]float64, n)
		for i := range vector {
			vector[i] = float64(random.IntN(13)-6) / 4
		}
		// Dyadic data make every independent dot product exactly representable.
		for k, tail := range []FloatTailMatrix{eta, row} {
			matrix := em
			if k == 1 {
				matrix = rm
			}
			left, right := append([]float64(nil), vector...), append([]float64(nil), vector...)
			if err := tail.ApplyLeft(left); err != nil {
				t.Fatal(world, err)
			}
			if err := tail.ApplyRight(right); err != nil {
				t.Fatal(world, err)
			}
			wantLeft, wantRight := lpFloatMatrixVector(matrix, vector, false), lpFloatMatrixVector(matrix, vector, true)
			lpFloatCheckVector(t, left, wantLeft)
			lpFloatCheckVector(t, right, wantRight)
			sparseLeft, sparseRight := lpFloatSparseFromDense(vector), lpFloatSparseFromDense(vector)
			if err := tail.ApplyLeftToT(sparseLeft, s); err != nil {
				t.Fatal(world, err)
			}
			if err := tail.ApplyRightIndexed(sparseRight); err != nil {
				t.Fatal(world, err)
			}
			lpFloatCheckSparse(t, sparseLeft, wantLeft)
			lpFloatCheckSparse(t, sparseRight, wantRight)
		}
	}
}
func TestLPFloatEtaPermutationConjugationAndTailComposition(t *testing.T) {
	s, err := NewSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range lpPermutations(4) {
		p, err := FloatPermutationFromValues(4, values)
		if err != nil {
			t.Fatal(err)
		}
		pm := lpFloatIdentity(4)
		for i := range pm {
			clear(pm[i])
			pm[i][values[i]] = 1
		}
		eta := NewFloatEtaMatrix(1, 4)
		eta.SetDiagonal(2)
		row := NewFloatRowEtaMatrix(0, 1, 4)
		em, rm := lpFloatIdentity(4), lpFloatIdentity(4)
		em[1][1] = 0.5
		for i, coefficient := range []float64{-2, 0, 3, 4} {
			if i == 1 {
				continue
			}
			if err := eta.Append(uint32(i), coefficient); err != nil {
				t.Fatal(err)
			}
			if err := row.Append(uint32(i), coefficient); err != nil {
				t.Fatal(err)
			}
			em[i][1] = coefficient
			rm[1][i] = coefficient
		}
		if err := eta.Conjugate(&p.Permutation); err != nil {
			t.Fatal(err)
		}
		if err := row.Conjugate(&p.Permutation); err != nil {
			t.Fatal(err)
		}
		em = lpFloatMatrixProduct(lpFloatMatrixProduct(pm, em), lpFloatMatrixTranspose(pm))
		rm = lpFloatMatrixProduct(lpFloatMatrixProduct(pm, rm), lpFloatMatrixTranspose(pm))
		if eta.RowCount() != 4 || eta.ColumnCount() != 4 || row.RowCount() != 4 || row.ColumnCount() != 4 {
			t.Fatal("dimension")
		}
		for i := uint32(0); i < 4; i++ {
			for j := uint32(0); j < 4; j++ {
				a, err := eta.Element(i, j)
				if err != nil || a != em[i][j] {
					t.Fatal(i, j, a, em, err)
				}
				b, err := row.Element(i, j)
				if err != nil || b != rm[i][j] {
					t.Fatal(i, j, b, rm, err)
				}
			}
		}
		// Exercise the actual mixed tail interface, including permutations, in
		// dense and sparse directions against independent composed matrices.
		matrices := [][][]float64{em, pm, rm}
		tails := []FloatTailMatrix{eta, p, row}
		original := []float64{1, -2, 3, -4}
		left, right := append([]float64(nil), original...), append([]float64(nil), original...)
		sparseLeft, sparseRight := lpFloatSparseFromDense(original), lpFloatSparseFromDense(original)
		wantLeft, wantRight := append([]float64(nil), original...), append([]float64(nil), original...)
		for i, tail := range tails {
			if tail.IsDense() {
				t.Fatal("sparse tail reports dense")
			}
			if err := tail.ApplyLeft(left); err != nil {
				t.Fatal(err)
			}
			if err := tail.ApplyRight(right); err != nil {
				t.Fatal(err)
			}
			if err := tail.ApplyLeftToT(sparseLeft, s); err != nil {
				t.Fatal(err)
			}
			if err := tail.ApplyRightIndexed(sparseRight); err != nil {
				t.Fatal(err)
			}
			wantLeft = lpFloatMatrixVector(matrices[i], wantLeft, false)
			wantRight = lpFloatMatrixVector(matrices[i], wantRight, true)
		}
		lpFloatCheckVector(t, left, wantLeft)
		lpFloatCheckVector(t, right, wantRight)
		lpFloatCheckSparse(t, sparseLeft, wantLeft)
		lpFloatCheckSparse(t, sparseRight, wantRight)
	}
}
func TestLPSparseFloatVectorAndEtaLiteralStorage(t *testing.T) {
	var v SparseFloatVector
	v.Append(2, 4)
	v.Append(2, 8)
	v.Append(1, 0)
	if v.Size() != 3 || v.At(2) != 4 || v.At(99) != 0 {
		t.Fatal(v)
	}
	if err := v.Divide(2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(v.Data, []FloatSparseEntry{{2, 2}, {2, 4}, {1, 0}}) {
		t.Fatal(v.Data)
	}
	eta := NewFloatEtaMatrix(1, 3)
	if !eta.SetDiagonal(1) {
		t.Fatal("diagonal assignment")
	}
	if err := eta.Append(2, 0); err != nil || eta.Column.Size() != 1 {
		t.Fatal(err, eta)
	}
	if !eta.SetDiagonal(2) {
		t.Fatal("diagonal")
	}
	if err := eta.Append(0, 8); err != nil {
		t.Fatal(err)
	}
	if err := eta.DivideByDiagonal(); err != nil || eta.Column.At(0) != 4 {
		t.Fatal(err, eta.Column)
	}
	if eta.SetDiagonal(math.Nextafter(1e-12, 0)) || !eta.SetDiagonal(1e-12) || !eta.SetDiagonal(math.NaN()) {
		t.Fatal("diagonal acceptance boundary")
	}
	eta = NewFloatEtaMatrix(1, 3)
	eta.SetDiagonal(1)
	if err := eta.Append(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := eta.Append(0, 3); err != nil {
		t.Fatal(err)
	}
	if value, err := eta.Element(0, 1); err != nil || value != 2 {
		t.Fatal(value, err)
	}
	w := []float64{0, 1, 0}
	if err := eta.ApplyLeft(w); err != nil || w[0] != 5 {
		t.Fatal(err, w)
	}
}
