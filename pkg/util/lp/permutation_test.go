package lp

import (
	"math"
	"reflect"
	"testing"
)

func lpPermutations(n int) [][]uint32 {
	values := make([]uint32, n)
	for i := range values {
		values[i] = uint32(i)
	}
	var out [][]uint32
	var walk func(int)
	walk = func(i int) {
		if i == n {
			out = append(out, append([]uint32(nil), values...))
			return
		}
		for j := i; j < n; j++ {
			values[i], values[j] = values[j], values[i]
			walk(i + 1)
			values[i], values[j] = values[j], values[i]
		}
	}
	walk(0)
	return out
}
func lpPermutationMatrix(values []uint32) [][]int {
	m := make([][]int, len(values))
	for i, j := range values {
		m[i] = make([]int, len(values))
		m[i][j] = 1
	}
	return m
}
func lpMatrixProduct(a, b [][]int) [][]int {
	out := make([][]int, len(a))
	for i := range out {
		out[i] = make([]int, len(b))
		for j := range out[i] {
			for k := range b {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}
func lpMatrixTranspose(a [][]int) [][]int {
	out := make([][]int, len(a))
	for i := range out {
		out[i] = make([]int, len(a))
		for j := range out[i] {
			out[i][j] = a[j][i]
		}
	}
	return out
}
func lpPermutationCheckMatrix(t *testing.T, p *Permutation, want [][]int) {
	t.Helper()
	if p.Size() != uint32(len(want)) {
		t.Fatal(p.Size(), want)
	}
	for i := range want {
		for j := range want[i] {
			got, err := p.Entry(uint32(i), uint32(j))
			if err != nil || int(got) != want[i][j] {
				t.Fatal(i, j, got, want, err)
			}
		}
	}
	for i, j := range p.Values() {
		back, err := p.Reverse(j)
		if err != nil || back != uint32(i) {
			t.Fatal(i, j, back, err)
		}
	}
}

func TestLPPermutationIndependentMatrixProducts(t *testing.T) {
	permutations := lpPermutations(4)
	for _, a := range permutations {
		for _, b := range permutations {
			am, bm := lpPermutationMatrix(a), lpPermutationMatrix(b)
			for _, op := range []string{"left", "right", "reverse-left", "reverse-right"} {
				p, err := PermutationFromValues(4, a)
				if err != nil {
					t.Fatal(err)
				}
				q, err := PermutationFromValues(4, b)
				if err != nil {
					t.Fatal(err)
				}
				var want [][]int
				switch op {
				case "left":
					err = p.MultiplyLeft(q)
					want = lpMatrixProduct(bm, am)
				case "right":
					err = p.MultiplyRight(q)
					want = lpMatrixProduct(am, bm)
				case "reverse-left":
					err = p.MultiplyReverseLeft(q)
					want = lpMatrixProduct(lpMatrixTranspose(bm), am)
				case "reverse-right":
					err = p.MultiplyReverseRight(q)
					want = lpMatrixProduct(am, lpMatrixTranspose(bm))
				}
				if err != nil {
					t.Fatal(op, a, b, err)
				}
				lpPermutationCheckMatrix(t, p, want)
			}
		}
	}
}

func TestLPPermutationResizeTransposeAndAliasContracts(t *testing.T) {
	p := NewPermutation(4)
	if !p.IsIdentity() || p.IsDense() {
		t.Fatal(p)
	}
	if err := p.TransposeRight(0, 2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Values(), []uint32{2, 1, 0, 3}) {
		t.Fatal(p.Values())
	}
	if err := p.TransposeLeft(0, 3); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Values(), []uint32{2, 1, 3, 0}) {
		t.Fatal(p.Values())
	}
	for _, values := range lpPermutations(4) {
		for i := uint32(0); i < 4; i++ {
			for j := i + 1; j < 4; j++ {
				p, err := PermutationFromValues(4, values)
				if err != nil {
					t.Fatal(err)
				}
				transposition := NewPermutation(4)
				if err := transposition.TransposeRight(i, j); err != nil {
					t.Fatal(err)
				}
				want := lpMatrixProduct(lpPermutationMatrix(transposition.Values()), lpPermutationMatrix(values))
				if err := p.TransposeRight(i, j); err != nil {
					t.Fatal(err)
				}
				lpPermutationCheckMatrix(t, p, want)
				p, err = PermutationFromValues(4, values)
				if err != nil {
					t.Fatal(err)
				}
				want = lpMatrixProduct(lpPermutationMatrix(values), lpPermutationMatrix(transposition.Values()))
				if err := p.TransposeLeft(i, j); err != nil {
					t.Fatal(err)
				}
				lpPermutationCheckMatrix(t, p, want)
			}
		}
	}
	// Shrink preserves the stale prefix; growing adds identity entries only.
	p, err := PermutationFromValues(3, []uint32{2, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	p.Resize(2)
	if !reflect.DeepEqual(p.Values(), []uint32{2, 0}) || !reflect.DeepEqual(p.reverse, []uint32{1, 2}) {
		t.Fatal(p)
	}
	p.Resize(4)
	if !reflect.DeepEqual(p.Values(), []uint32{2, 0, 2, 3}) || !reflect.DeepEqual(p.reverse, []uint32{1, 2, 2, 3}) {
		t.Fatal(p)
	}
	p.Init(3)
	if !p.IsIdentity() {
		t.Fatal(p)
	}
	// Source reads the other operand while mutating itself; right self-product
	// is intentionally not repaired into a mathematical square.
	for _, tc := range []struct {
		op   string
		want []uint32
	}{
		{"left", []uint32{2, 0, 1}}, {"right", []uint32{1, 1, 1}},
		{"reverse-left", []uint32{0, 1, 2}}, {"reverse-right", []uint32{0, 2, 2}},
	} {
		p, _ := PermutationFromValues(3, []uint32{1, 2, 0})
		switch tc.op {
		case "left":
			err = p.MultiplyLeft(p)
		case "right":
			err = p.MultiplyRight(p)
		case "reverse-left":
			err = p.MultiplyReverseLeft(p)
		case "reverse-right":
			err = p.MultiplyReverseRight(p)
		}
		if err != nil || !reflect.DeepEqual(p.Values(), tc.want) {
			t.Fatal(tc.op, p.Values(), tc.want, err)
		}
	}
}

func TestLPFloatPermutationIndependentDenseAndSparseProducts(t *testing.T) {
	dense := []float64{0, 2, -3, 0, 5}
	for _, values := range lpPermutations(5) {
		matrix := lpPermutationMatrix(values)
		for _, op := range []string{"left", "right", "reverse-left-t", "reverse-left-x", "reverse-right-t", "reverse-right-x"} {
			p, err := FloatPermutationFromValues(5, values)
			if err != nil {
				t.Fatal(err)
			}
			got := append([]float64(nil), dense...)
			want := make([]float64, 5)
			// Compute row/column products with a separate dense 0/1 matrix.
			for i := range want {
				for j := range dense {
					switch op {
					case "left", "reverse-right-t", "reverse-right-x":
						want[i] += float64(float64(matrix[i][j]) * dense[j])
					default:
						want[i] += float64(dense[j] * float64(matrix[j][i]))
					}
				}
			}
			switch op {
			case "left":
				err = p.ApplyLeft(got)
			case "right":
				err = p.ApplyRight(got)
			case "reverse-left-t":
				err = p.ApplyReverseLeftToT(got)
			case "reverse-left-x":
				err = p.ApplyReverseLeftToX(got)
			case "reverse-right-t":
				err = p.ApplyReverseRightToT(got)
			case "reverse-right-x":
				err = p.ApplyReverseRightToX(got)
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal(op, values, got, want, err)
			}
		}
		for _, op := range []string{"left", "right", "reverse-left", "reverse-right"} {
			p, err := FloatPermutationFromValues(5, values)
			if err != nil {
				t.Fatal(err)
			}
			v := NewIndexedFloatVector(5)
			// Deliberately nonascending sparse order must survive as a mapping.
			for _, i := range []uint32{4, 1, 2} {
				if err := v.SetValue(dense[i], i); err != nil {
					t.Fatal(err)
				}
			}
			want := make([]float64, 5)
			wantIndex := make([]uint32, len(v.Index))
			for k, i := range v.Index {
				var j uint32
				if op == "left" || op == "reverse-right" {
					for row := range matrix {
						if matrix[row][i] == 1 {
							j = uint32(row)
						}
					}
				} else {
					for col := range matrix[i] {
						if matrix[i][col] == 1 {
							j = uint32(col)
						}
					}
				}
				want[j] = dense[i]
				wantIndex[k] = j
			}
			switch op {
			case "left":
				err = p.ApplyLeftIndexed(v)
			case "right":
				err = p.ApplyRightIndexed(v)
			case "reverse-left":
				err = p.ApplyReverseLeftIndexed(v)
			case "reverse-right":
				err = p.ApplyReverseRightIndexed(v)
			}
			if err != nil || !reflect.DeepEqual(v.Data, want) || !reflect.DeepEqual(v.Index, wantIndex) {
				t.Fatal(op, values, v, want, wantIndex, err)
			}
		}
	}
}

// Source: src/test/lp.cpp test_permutations (--test_perm), including both
// vector_of_permutaions cases. Independent matrix products replace debug-only
// dense_matrix assertions, so all source checks also execute in native Go.
func TestLPFloatPermutationSource(t *testing.T) {
	original := []uint32{4, 2, 0, 3, 1}
	for _, values := range [][]uint32{{1, 2, 3, 4, 0}, {2, 0, 1, 4, 3}} {
		p, err := FloatPermutationFromValues(5, original)
		if err != nil {
			t.Fatal(err)
		}
		l, err := FloatPermutationFromValues(5, values)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.MultiplyReverseRight(&l.Permutation); err != nil {
			t.Fatal(err)
		}
		lpPermutationCheckMatrix(t, &p.Permutation, lpMatrixProduct(lpPermutationMatrix(original), lpMatrixTranspose(lpPermutationMatrix(values))))
	}
	p, err := FloatPermutationFromValues(5, original)
	if err != nil {
		t.Fatal(err)
	}
	dense := []float64{0, 1, 0, 3, 0}
	sparse := NewIndexedFloatVector(5)
	if err := sparse.SetValue(1, 1); err != nil {
		t.Fatal(err)
	}
	if err := sparse.SetValue(3, 3); err != nil {
		t.Fatal(err)
	}
	if err := p.ApplyReverseRightToT(dense); err != nil {
		t.Fatal(err)
	}
	if err := p.ApplyReverseRightIndexed(sparse); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dense, sparse.Data) || !reflect.DeepEqual(dense, []float64{0, 0, 0, 3, 1}) {
		t.Fatal(dense, sparse)
	}
	inverse, err := p.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	lpPermutationCheckMatrix(t, &inverse.Permutation, lpMatrixTranspose(lpPermutationMatrix(original)))
	if p.RowCount() != 5 || p.ColumnCount() != 5 {
		t.Fatal(p)
	}
	if x, err := p.Element(0, 4); err != nil || x != 1 {
		t.Fatal(x, err)
	}
	if x, err := p.Element(0, 999); err != nil || x != 0 {
		t.Fatal(x, err)
	}
	p.Resize(8)
	if len(p.tBuffer) != 8 || len(p.xBuffer) != 8 {
		t.Fatal(p)
	}
	p.Init(2)
	if !p.IsIdentity() || len(p.tBuffer) != 2 || len(p.xBuffer) != 2 {
		t.Fatal(p)
	}
}

func TestLPPermutationGuardPrefixesAndFloatingComparisons(t *testing.T) {
	p := NewPermutation(2)
	if _, err := p.Value(2); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := p.Reverse(2); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := p.Entry(2, 0); err != ErrBounds {
		t.Fatal(err)
	}
	if err := p.Set(2, 0); err != ErrBounds {
		t.Fatal(err)
	}
	if err := p.Set(0, 2); err != ErrBounds {
		t.Fatal(err)
	}
	if err := p.TransposeLeft(0, 2); err != ErrBounds {
		t.Fatal(err)
	}
	if err := p.TransposeRight(0, 2); err != ErrBounds {
		t.Fatal(err)
	}
	for _, op := range []func(uint32, uint32) error{p.TransposeLeft, p.TransposeRight} {
		err := op(0, 0)
		if sourceDebugTests {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if partial, err := PermutationFromValues(3, []uint32{2, 0}); err != ErrBounds || !reflect.DeepEqual(partial.Values(), []uint32{2, 0, 0}) {
		t.Fatal(partial, err)
	}
	if _, err := FloatPermutationFromValues(2, []uint32{0, 2}); err != ErrBounds {
		t.Fatal(err)
	}
	short := NewPermutation(1)
	for _, op := range []struct {
		name string
		call func(*Permutation) error
	}{
		{"left", p.MultiplyLeft}, {"right", p.MultiplyRight}, {"reverse-right", p.MultiplyReverseRight}, {"reverse-left", p.MultiplyReverseLeft},
	} {
		err := op.call(short)
		if sourceDebugTests && op.name != "reverse-left" {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != ErrBounds {
			t.Fatal(err)
		}
	}
	f := NewFloatPermutation(2)
	for _, op := range []func([]float64) error{f.ApplyLeft, f.ApplyRight, f.ApplyReverseLeftToT, f.ApplyReverseLeftToX, f.ApplyReverseRightToT, f.ApplyReverseRightToX} {
		err := op([]float64{1})
		if sourceDebugTests {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != ErrBounds {
			t.Fatal(err)
		}
		longer := []float64{1, 2, 99}
		err = op(longer)
		if sourceDebugTests {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != nil || longer[2] != 99 {
			t.Fatal(err, longer)
		}
	}
	for _, op := range []func(*IndexedFloatVector) error{f.ApplyLeftIndexed, f.ApplyRightIndexed, f.ApplyReverseLeftIndexed, f.ApplyReverseRightIndexed} {
		v := NewIndexedFloatVector(2)
		v.Index = []uint32{9}
		if err := op(v); err != ErrBounds {
			t.Fatal(err)
		}
	}
	v := NewIndexedFloatVector(2)
	v.Data[0] = 3 // unindexed data reaches source debug comparison
	if err := f.TransposeRight(0, 1); err != nil {
		t.Fatal(err)
	}
	err := f.ApplyRightIndexed(v)
	if sourceDebugTests {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if floatPermutationVectorsEqual([]float64{1}, nil) || floatPermutationVectorsEqual([]float64{1}, []float64{2}) {
		t.Fatal("unequal")
	}
	if !floatPermutationVectorsEqual([]float64{1e12, math.NaN(), math.Inf(1)}, []float64{1e12 + 1, 0, math.Inf(1)}) {
		t.Fatal("source relative/NaN/infinity comparison")
	}
}

func TestLPPermutationShrunkDomainPreservesReachedPrefixes(t *testing.T) {
	broken := func() *Permutation {
		p, err := PermutationFromValues(3, []uint32{2, 0, 1})
		if err != nil {
			t.Fatal(err)
		}
		p.Resize(2)
		return p
	}
	p := broken()
	if err := p.TransposeLeft(0, 1); err != ErrBounds || !reflect.DeepEqual(p.Values(), []uint32{2, 1}) {
		t.Fatal(err, p.Values())
	}
	p = broken()
	if err := p.TransposeRight(0, 1); err != ErrBounds || !reflect.DeepEqual(p.Values(), []uint32{0, 0}) {
		t.Fatal(err, p.Values())
	}
	for _, op := range []string{"left", "right", "reverse-left", "reverse-right"} {
		p = NewPermutation(2)
		q := broken()
		var err error
		switch op {
		case "left":
			err = p.MultiplyLeft(q)
		case "right":
			err = p.MultiplyRight(q)
		case "reverse-left":
			err = p.MultiplyReverseLeft(q)
		case "reverse-right":
			err = p.MultiplyReverseRight(q)
		}
		want := []uint32{0, 0}
		if op == "reverse-left" || op == "reverse-right" {
			want = []uint32{0, 1}
		}
		if err != ErrBounds || !reflect.DeepEqual(p.Values(), want) {
			t.Fatal(op, err, p.Values(), want)
		}
	}
	makeFloat := func() *FloatPermutation {
		p, err := FloatPermutationFromValues(3, []uint32{2, 0, 1})
		if err != nil {
			t.Fatal(err)
		}
		p.Resize(2)
		return p
	}
	for _, op := range []string{"left", "right", "reverse-left-t", "reverse-left-x", "reverse-right-t", "reverse-right-x"} {
		p := makeFloat()
		w := []float64{7, 11}
		var err error
		switch op {
		case "left":
			err = p.ApplyLeft(w)
		case "right":
			err = p.ApplyRight(w)
		case "reverse-left-t":
			err = p.ApplyReverseLeftToT(w)
		case "reverse-left-x":
			err = p.ApplyReverseLeftToX(w)
		case "reverse-right-t":
			err = p.ApplyReverseRightToT(w)
		case "reverse-right-x":
			err = p.ApplyReverseRightToX(w)
		}
		if err != ErrBounds || !reflect.DeepEqual(w, []float64{7, 11}) {
			t.Fatal(op, err, w)
		}
	}
	p2 := makeFloat()
	v := NewIndexedFloatVector(2)
	if err := v.SetValue(7, 1); err != nil {
		t.Fatal(err)
	}
	if err := p2.ApplyLeftIndexed(v); err != ErrBounds || !reflect.DeepEqual(v.Data, []float64{0, 0}) || !reflect.DeepEqual(v.Index, []uint32{1}) {
		t.Fatal(err, v)
	}
	p2 = makeFloat()
	v = NewIndexedFloatVector(2)
	if err := v.SetValue(7, 0); err != nil {
		t.Fatal(err)
	}
	if err := p2.ApplyReverseLeftIndexed(v); err != ErrBounds || !reflect.DeepEqual(v.Data, []float64{0, 0}) || !reflect.DeepEqual(v.Index, []uint32{0}) {
		t.Fatal(err, v)
	}
	p2 = makeFloat()
	v = NewIndexedFloatVector(2)
	if err := v.SetValue(7, 0); err != nil {
		t.Fatal(err)
	}
	if err := p2.ApplyRightIndexed(v); err != ErrBounds {
		t.Fatal(err)
	}
	if sourceDebugTests {
		// The upstream diagnostic dense application fails before sparse copy.
		if !reflect.DeepEqual(v.Data, []float64{7, 0}) || !reflect.DeepEqual(v.Index, []uint32{0}) {
			t.Fatal(v)
		}
	} else if !reflect.DeepEqual(v.Data, []float64{0, 0}) || len(v.Index) != 0 {
		t.Fatal(v)
	}
	p2 = makeFloat()
	v = NewIndexedFloatVector(2)
	if err := v.SetValue(7, 1); err != nil {
		t.Fatal(err)
	}
	if err := p2.ApplyReverseRightIndexed(v); err != ErrBounds || !reflect.DeepEqual(v.Data, []float64{0, 0}) || len(v.Index) != 0 {
		t.Fatal(err, v)
	}
	// A duplicate source sparse index reaches SetValue's publication prefix.
	p2 = NewFloatPermutation(2)
	v = NewIndexedFloatVector(2)
	v.Data[0] = 9
	v.Index = []uint32{0, 0}
	err := p2.ApplyRightIndexed(v)
	if sourceDebugTests {
		if err != ErrAssertion || !reflect.DeepEqual(v.Index, []uint32{0}) || v.Data[0] != 9 {
			t.Fatal(err, v)
		}
	} else if err != nil || !reflect.DeepEqual(v.Index, []uint32{0, 0}) || v.Data[0] != 9 {
		t.Fatal(err, v)
	}
}

// The indexed vector can be larger than the permutation while all of its
// indices remain valid for its own storage. Check the reached source mutation
// prefix when the later permutation lookup is outside the smaller mapping.
func TestLPFloatPermutationLargerSparseInputPrefixes(t *testing.T) {
	for _, op := range []string{"left", "reverse-left", "right", "reverse-right"} {
		t.Run(op, func(t *testing.T) {
			p, err := FloatPermutationFromValues(2, []uint32{1, 0})
			if err != nil {
				t.Fatal(err)
			}
			if p.IsDense() {
				t.Fatal("permutation tail reported dense")
			}
			w := NewIndexedFloatVector(3)
			w.Data[2], w.Data[0] = 9, 3
			w.Index = []uint32{2, 0}
			switch op {
			case "left":
				err = p.ApplyLeftIndexed(w)
			case "reverse-left":
				err = p.ApplyReverseLeftIndexed(w)
			case "right":
				err = p.ApplyRightIndexed(w)
			case "reverse-right":
				err = p.ApplyReverseRightIndexed(w)
			}
			wantErr := ErrBounds
			wantData := []float64{0, 0, 0}
			wantIndex := []uint32{}
			if op == "left" || op == "reverse-left" {
				wantData = []float64{0, 3, 0}
				wantIndex = []uint32{2, 1}
			} else if op == "right" && sourceDebugTests {
				wantErr = ErrAssertion
				wantData = []float64{3, 0, 9}
				wantIndex = []uint32{2, 0}
			}
			if err != wantErr || !reflect.DeepEqual(w.Data, wantData) || !reflect.DeepEqual(w.Index, wantIndex) {
				t.Fatalf("got %v data=%v indices=%v; want %v data=%v indices=%v", err, w.Data, w.Index, wantErr, wantData, wantIndex)
			}
		})
	}
}
