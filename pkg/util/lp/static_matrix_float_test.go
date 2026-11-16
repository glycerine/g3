package lp

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
)

func lpFloatStaticCheck(t *testing.T, m *FloatStaticMatrix, want map[IndexPair]float64) {
	t.Helper()
	seen := make(map[IndexPair]bool)
	for i, row := range m.Rows {
		for k, r := range row {
			p := IndexPair{uint32(i), r.Column}
			value, exists := want[p]
			if !exists || seen[p] || value != r.Value {
				t.Fatal("row terms", p, r.Value, value, exists)
			}
			seen[p] = true
			if uint64(r.Column) >= uint64(len(m.Columns)) || uint64(r.Offset) >= uint64(len(m.Columns[r.Column])) {
				t.Fatal("row link", r)
			}
			if c := m.Columns[r.Column][r.Offset]; c.Row != uint32(i) || c.Offset != uint32(k) {
				t.Fatal("row back-link", r, c)
			}
		}
	}
	if len(seen) != len(want) || int(m.Size()) != len(want) {
		t.Fatal("domain size", len(seen), len(want), m.Size())
	}
	for j, column := range m.Columns {
		for k, c := range column {
			if uint64(c.Row) >= uint64(len(m.Rows)) || uint64(c.Offset) >= uint64(len(m.Rows[c.Row])) {
				t.Fatal("column link", c)
			}
			if r := m.Rows[c.Row][c.Offset]; r.Column != uint32(j) || r.Offset != uint32(k) {
				t.Fatal("column back-link", c, r)
			}
		}
	}
	var domain []IndexPair
	for i := uint32(0); i < m.RowCount(); i++ {
		for j := uint32(0); j <= m.ColumnCount(); j++ {
			p := IndexPair{i, j}
			v, err := m.Element(i, j)
			if err != nil || v != want[p] {
				t.Fatal(p, v, err, want[p])
			}
			if _, ok := want[p]; ok {
				domain = append(domain, p)
			}
		}
	}
	if got := m.Domain(); !reflect.DeepEqual(got, domain) {
		t.Fatal(got, domain)
	}
	if ok, err := m.IsCorrect(io.Discard); err != nil || !ok {
		t.Fatal("source diagnostic", ok, err)
	}
}
func lpStaticMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLPFloatStaticMatrixIndependentMutationOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	m := NewFloatStaticMatrix(5, 7)
	want := make(map[IndexPair]float64)
	r := rand.New(rand.NewPCG(188, 2026))
	for step := 0; step < 12000; step++ {
		i, j := uint32(r.IntN(5)), uint32(r.IntN(7))
		p := IndexPair{i, j}
		switch r.IntN(5) {
		case 0:
			if _, ok := want[p]; !ok {
				v := float64(1+r.IntN(7)) / 4
				if r.IntN(2) == 0 {
					v = -v
				}
				lpStaticMust(t, m.Set(i, j, v))
				want[p] = v
			}
		case 1:
			if _, ok := want[p]; ok {
				var offset uint32
				for k, c := range m.Rows[i] {
					if c.Column == j {
						offset = uint32(k)
						break
					}
				}
				lpStaticMust(t, m.RemoveElement(i, offset))
				delete(want, p)
			}
		case 2:
			a := []float64{0, -1, 2, .5}[r.IntN(4)]
			lpStaticMust(t, m.MultiplyRow(i, a))
			for p, v := range want {
				if p.I == i {
					want[p] = v * a
				}
			}
		case 3:
			a := []float64{0, -1, 2, .5}[r.IntN(4)]
			lpStaticMust(t, m.MultiplyColumn(j, a))
			for p, v := range want {
				if p.J == j {
					want[p] = v * a
				}
			}
		case 4:
			ii := uint32(r.IntN(5))
			lpStaticMust(t, m.TransposeRows(i, ii))
			next := make(map[IndexPair]float64)
			for p, v := range want {
				if p.I == i {
					p.I = ii
				} else if p.I == ii {
					p.I = i
				}
				next[p] = v
			}
			want = next
		}
		lpFloatStaticCheck(t, m, want)
	}
}

func TestLPFloatStaticMatrixIndependentPivotOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	r := rand.New(rand.NewPCG(991, 402))
	multipleCancellations := 0
	for world := 0; world < 2000; world++ {
		m := NewFloatStaticMatrix(4, 7)
		want := make(map[IndexPair]float64)
		pivot, target := uint32(r.IntN(4)), uint32(r.IntN(4))
		for target == pivot {
			target = uint32(r.IntN(4))
		}
		column := uint32(r.IntN(7))
		alpha := float64(1+r.IntN(4)) / 2
		if world%2 == 0 {
			alpha = -alpha
		}
		want[IndexPair{pivot, column}] = 1
		want[IndexPair{target, column}] = alpha
		for j := uint32(0); j < 7; j++ {
			if j == column {
				continue
			}
			if r.IntN(3) != 0 {
				want[IndexPair{pivot, j}] = float64(1+r.IntN(6)) / 2
			}
			if r.IntN(3) != 0 {
				v := float64(r.IntN(13)-6) / 2
				if v != 0 {
					want[IndexPair{target, j}] = v
				}
			}
			if p, ok := want[IndexPair{pivot, j}]; ok && r.IntN(2) == 0 {
				want[IndexPair{target, j}] = alpha * p
			}
		}
		// Insert each row in an independently shuffled order; cancellation
		// offsets then arrive in orders unrelated to sorted coordinates.
		for i := uint32(0); i < 4; i++ {
			for _, j := range r.Perm(7) {
				if v, ok := want[IndexPair{i, uint32(j)}]; ok {
					lpStaticMust(t, m.Set(i, uint32(j), v))
				}
			}
		}
		var cell ColumnCell
		for _, c := range m.Columns[column] {
			if c.Row == target {
				cell = c
				break
			}
		}
		canceled := 0
		delete(want, IndexPair{target, column})
		for j := uint32(0); j < 7; j++ {
			if j == column {
				continue
			}
			p, exists := want[IndexPair{pivot, j}]
			if !exists {
				continue
			}
			key := IndexPair{target, j}
			v, old := want[key]
			v -= float64(alpha * p)
			if old && v == 0 {
				delete(want, key)
				canceled++
			} else {
				want[key] = v
			}
		}
		if canceled > 1 {
			multipleCancellations++
		}
		nonempty, err := m.PivotRowToRow(pivot, cell, column)
		lpStaticMust(t, err)
		expectedNonempty := false
		for p := range want {
			if p.I == target {
				expectedNonempty = true
			}
		}
		if nonempty != expectedNonempty {
			t.Fatal("pivot result", nonempty, expectedNonempty)
		}
		lpFloatStaticCheck(t, m, want)
		for _, offset := range m.RowOffsets {
			if offset != -1 {
				t.Fatal("unclean work offsets", m.RowOffsets)
			}
		}
		for k := 1; k < len(m.becameZeros); k++ {
			if m.becameZeros[k-1] > m.becameZeros[k] {
				t.Fatal("cancellation removal order", m.becameZeros)
			}
		}
	}
	if multipleCancellations < 100 {
		t.Fatal("insufficient multiple cancellation worlds", multipleCancellations)
	}
}

func TestLPFloatStaticMatrixScopesAndInitialization(t *testing.T) {
	m := NewFloatStaticMatrix(2, 3)
	want := map[IndexPair]float64{{0, 1}: 2, {1, 2}: 3}
	for p, v := range want {
		lpStaticMust(t, m.Set(p.I, p.J, v))
	}
	m.Push()
	m.AddColumns(2)
	m.AddRow()
	lpStaticMust(t, m.Set(2, 4, 4))
	want[IndexPair{2, 4}] = 4
	m.Push()
	m.AddColumn()
	m.AddRow()
	lpStaticMust(t, m.Set(3, 1, 5))
	lpStaticMust(t, m.Set(3, 5, 6))
	want[IndexPair{3, 1}] = 5
	want[IndexPair{3, 5}] = 6
	lpFloatStaticCheck(t, m, want)
	lpStaticMust(t, m.Pop(1))
	delete(want, IndexPair{3, 1})
	delete(want, IndexPair{3, 5})
	lpFloatStaticCheck(t, m, want)
	if m.RowCount() != 3 || m.ColumnCount() != 5 || len(m.RowOffsets) != 6 {
		t.Fatal("source workspace extent", m.RowCount(), m.ColumnCount(), len(m.RowOffsets))
	}
	lpStaticMust(t, m.Pop(0))
	lpStaticMust(t, m.Pop(99))
	delete(want, IndexPair{2, 4})
	lpFloatStaticCheck(t, m, want)
	if m.RowCount() != 2 || m.ColumnCount() != 3 || len(m.RowOffsets) != 6 {
		t.Fatal("scope dimensions")
	}
	m.Push()
	m.Clear()
	lpStaticMust(t, m.Pop(1))
	if m.RowCount() != 0 || m.ColumnCount() != 0 || len(m.stack) != 0 {
		t.Fatal("clear preserves then pop consumes scope")
	}
	lpStaticMust(t, m.InitEmptyMatrix(2, 4))
	if len(m.RowOffsets) != 0 || m.ColumnCount() != 4 {
		t.Fatal("source initialization order")
	}
	m.InitRowOffsets()
	if !reflect.DeepEqual(m.RowOffsets, []int32{-1, -1, -1, -1}) {
		t.Fatal(m.RowOffsets)
	}
	m.RowOffsets[0] = 7
	err := m.InitEmptyMatrix(1, 2)
	if sourceDebugTests {
		if err != ErrAssertion || m.RowCount() != 2 || m.ColumnCount() != 4 {
			t.Fatal(err)
		}
	} else if err != nil || m.RowCount() != 3 || m.ColumnCount() != 6 {
		t.Fatal(err)
	}
	if len(m.RowOffsets) != 4 || m.RowOffsets[0] != -1 {
		t.Fatal("offset reset precedes assertion", m.RowOffsets)
	}
}

func TestLPFloatStaticMatrixQueriesAndColumnCopies(t *testing.T) {
	m := NewFloatStaticMatrix(3, 4)
	for _, c := range []struct {
		i, j uint32
		v    float64
	}{{0, 2, -3}, {2, 2, 4}, {1, 0, 2}, {2, 1, -1}} {
		lpStaticMust(t, m.Set(c.i, c.j, c.v))
	}
	if n, err := m.RowSize(2); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if n, err := m.ColumnSize(2); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	for _, c := range []struct {
		i        uint32
		min, max float64
	}{{0, 3, 3}, {1, 2, 2}, {2, 1, 4}} {
		lo, err := m.MinAbsRow(c.i)
		lpStaticMust(t, err)
		hi, err := m.MaxAbsRow(c.i)
		lpStaticMust(t, err)
		if lo != c.min || hi != c.max {
			t.Fatal(lo, hi)
		}
	}
	for _, c := range []struct {
		i        uint32
		min, max float64
	}{{0, 2, 2}, {1, 1, 1}, {2, 3, 4}, {3, 0, 0}} {
		lo, err := m.MinAbsColumn(c.i)
		lpStaticMust(t, err)
		hi, err := m.MaxAbsColumn(c.i)
		lpStaticMust(t, err)
		if lo != c.min || hi != c.max {
			t.Fatal(lo, hi)
		}
	}
	if v, err := m.DotRow(2, []float64{7, 2, 3, 4}); err != nil || v != 10 {
		t.Fatal(v, err)
	}
	if v, err := m.DotColumn([]float64{2, 3, 5}, 2); err != nil || v != 14 {
		t.Fatal(v, err)
	}
	v := []float64{10, 20, 30}
	lpStaticMust(t, m.AddColumnToVector(.5, 2, v))
	lpFloatCheckVector(t, v, []float64{8.5, 20, 32})
	v, err := m.CopyColumnToVector(2, []float64{8, 9, 10, 11})
	lpStaticMust(t, err)
	lpFloatCheckVector(t, v, []float64{-3, 9, 4})
	v, err = m.CopyColumnToVector(0, nil)
	lpStaticMust(t, err)
	lpFloatCheckVector(t, v, []float64{0, 2, 0})
	sparse := NewIndexedFloatVector(3)
	lpStaticMust(t, m.CopyColumnToIndexedVector(2, sparse))
	if !reflect.DeepEqual(sparse.Index, []uint32{0, 2}) {
		t.Fatal(sparse.Index)
	}
	lpStaticMust(t, m.MultiplyColumn(2, 0))
	v, err = m.CopyColumnToVector(2, []float64{8, 9, 10})
	lpStaticMust(t, err)
	lpFloatCheckVector(t, v, []float64{8, 9, 10})
	sparse = NewIndexedFloatVector(3)
	lpStaticMust(t, m.CopyColumnToIndexedVector(2, sparse))
	if sparse.Size() != 0 {
		t.Fatal(sparse.Index)
	}
	if _, err = m.Balance(); err != ErrLPBalanceNotImplemented {
		t.Fatal(err)
	}
	if _, err = m.RowBalance(0); err != nil {
		t.Fatal(err)
	}
	lpStaticMust(t, m.MultiplyRow(1, 0))
	lpStaticMust(t, m.MultiplyRow(2, 0))
	if v, err := m.Balance(); err != nil || v != 0 {
		t.Fatal(v, err)
	}
	m = NewFloatStaticMatrix(1, 3)
	lpStaticMust(t, m.Set(0, 0, math.NaN()))
	lpStaticMust(t, m.Set(0, 1, -2))
	lpStaticMust(t, m.Set(0, 2, math.Inf(-1)))
	lo, err := m.MinAbsRow(0)
	lpStaticMust(t, err)
	hi, err := m.MaxAbsRow(0)
	lpStaticMust(t, err)
	if !math.IsNaN(lo) || !math.IsInf(hi, 1) {
		t.Fatal(lo, hi)
	}
	if _, err = m.RowBalance(0); err != ErrLPBalanceNotImplemented {
		t.Fatal(err)
	}
}

type lpStaticRefusingWriter struct{ err error }

func (w lpStaticRefusingWriter) Write([]byte) (int, error) { return 0, w.err }
func TestLPFloatStaticMatrixLiteralDuplicatesAndDiagnosticGaps(t *testing.T) {
	m := NewFloatStaticMatrix(1, 2)
	lpStaticMust(t, m.Set(99, 99, 0))
	lpStaticMust(t, m.Set(0, 1, 2))
	lpStaticMust(t, m.Set(0, 1, 3))
	if v, err := m.Element(0, 1); err != nil || v != 2 {
		t.Fatal(v, err)
	}
	if !reflect.DeepEqual(m.Domain(), []IndexPair{{0, 1}}) || m.Size() != 2 {
		t.Fatal(m.Domain(), m.Size())
	}
	var out bytes.Buffer
	ok, err := m.IsCorrect(&out)
	lpStaticMust(t, err)
	if ok || out.String() != "found column 1 twice in a row\n" {
		t.Fatal(ok, out.String())
	}
	failure := errors.New("writer stopped")
	if ok, err = m.IsCorrect(lpStaticRefusingWriter{failure}); ok || !errors.Is(err, failure) {
		t.Fatal(ok, err)
	}
	m = NewFloatStaticMatrix(1, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Columns[0] = append(m.Columns[0], m.Columns[0][0])
	out.Reset()
	ok, err = m.IsCorrect(&out)
	lpStaticMust(t, err)
	if ok || out.String() != "found row 0 twice in a column\n" {
		t.Fatal(ok, out.String())
	}
	// Source's final column-value comparison is with itself. An orphan
	// column entry can pass this diagnostic; the independent oracle rejects it.
	m = NewFloatStaticMatrix(1, 2)
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Columns[1] = append(m.Columns[1], ColumnCell{0, 0})
	if ok, err = m.IsCorrect(io.Discard); err != nil || !ok {
		t.Fatal("source gap changed", ok, err)
	}
	m = NewFloatStaticMatrix(1, 1)
	lpStaticMust(t, m.Set(0, 0, math.NaN()))
	if ok, err = m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
}

func TestLPFloatStaticMatrixBoundsAndVectorPrefixes(t *testing.T) {
	m := NewFloatStaticMatrix(3, 4)
	lpStaticMust(t, m.Set(0, 1, 2))
	lpStaticMust(t, m.Set(2, 1, 3))
	assertBounds := error(ErrBounds)
	if sourceDebugTests {
		assertBounds = ErrAssertion
	}
	if err := m.Set(99, 0, 1); err != assertBounds {
		t.Fatal(err)
	}
	for _, op := range []func() error{
		func() error { return m.AddNewElement(99, 0, 1) }, func() error { return m.AddNewElement(0, 99, 1) },
		func() error { _, e := m.ColumnValue(ColumnCell{99, 0}); return e }, func() error { _, e := m.ColumnValue(ColumnCell{0, 99}); return e },
		func() error { _, e := m.Element(99, 0); return e }, func() error { _, e := m.RowSize(99); return e }, func() error { _, e := m.ColumnSize(99); return e },
		func() error { return m.MultiplyRow(99, 1) }, func() error { return m.MultiplyColumn(99, 1) }, func() error { _, e := m.MinAbsRow(99); return e }, func() error { _, e := m.MaxAbsRow(99); return e },
		func() error { _, e := m.MinAbsColumn(99); return e }, func() error { _, e := m.MaxAbsColumn(99); return e }, func() error { _, e := m.RowBalance(99); return e },
		func() error { return m.TransposeRows(99, 0) }, func() error { return m.RemoveElement(99, 0) }, func() error { return m.RemoveElement(0, 99) },
		func() error { return m.AddColumnToVector(1, 99, nil) },
	} {
		if err := op(); err != ErrBounds {
			t.Fatal(err)
		}
	}
	for _, op := range []func() error{
		func() error { _, e := m.DotRow(99, nil); return e }, func() error { _, e := m.DotColumn(nil, 99); return e }, func() error { return m.CopyColumnToIndexedVector(99, NewIndexedFloatVector(3)) },
	} {
		if err := op(); err != assertBounds {
			t.Fatal(err)
		}
	}
	if v, err := m.DotColumn([]float64{4}, 1); err != ErrBounds || v != 8 {
		t.Fatal(v, err)
	}
	if v, err := m.DotRow(0, []float64{4}); err != ErrBounds || v != 0 {
		t.Fatal(v, err)
	}
	v := []float64{5}
	if err := m.AddColumnToVector(2, 1, v); err != ErrBounds || v[0] != 9 {
		t.Fatal(v, err)
	}
	sparse := NewIndexedFloatVector(1)
	if err := m.CopyColumnToIndexedVector(1, sparse); err != ErrBounds || sparse.Data[0] != 2 || !reflect.DeepEqual(sparse.Index, []uint32{0}) {
		t.Fatal(err, sparse)
	}
	v, err := m.CopyColumnToVector(99, []float64{5})
	if err != ErrBounds || !reflect.DeepEqual(v, []float64{5, 0, 0}) {
		t.Fatal(v, err)
	}
	sparse = NewIndexedFloatVector(3)
	lpStaticMust(t, sparse.SetValue(9, 0))
	err = m.CopyColumnToIndexedVector(1, sparse)
	if sourceDebugTests {
		if err != ErrAssertion || sparse.Data[0] != 2 || !reflect.DeepEqual(sparse.Index, []uint32{0}) {
			t.Fatal(err, sparse)
		}
	} else if err != nil || !reflect.DeepEqual(sparse.Index, []uint32{0, 0, 2}) {
		t.Fatal(err, sparse)
	}
	// A structurally invalid source column is reported only after already
	// copied entries. Storage is public for the retained solver algorithms.
	m.Columns[1] = append(m.Columns[1], ColumnCell{1, 99})
	v = []float64{8, 9, 10}
	v, err = m.CopyColumnToVector(1, v)
	if err != ErrBounds || !reflect.DeepEqual(v, []float64{2, 9, 3}) {
		t.Fatal(v, err)
	}
	if err = m.CopyColumnToIndexedVector(1, NewIndexedFloatVector(3)); err != ErrBounds {
		t.Fatal(err)
	}
	if err = m.AddColumnToVector(1, 1, make([]float64, 3)); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err = m.DotColumn(make([]float64, 3), 1); err != ErrBounds {
		t.Fatal(err)
	}
	if err = m.MultiplyColumn(1, 2); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err = m.MinAbsColumn(1); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err = m.MaxAbsColumn(1); err != ErrBounds {
		t.Fatal(err)
	}
	// Empty rows have both extrema zero, independent of column extent.
	if a, err := m.MinAbsRow(1); err != nil || a != 0 {
		t.Fatal(a, err)
	}
	if a, err := m.MaxAbsRow(1); err != nil || a != 0 {
		t.Fatal(a, err)
	}
}

func TestLPFloatStaticMatrixPivotIEEEAndLiteralGuards(t *testing.T) {
	// New fill is stored even when its floating product underflows to -0.
	m := NewFloatStaticMatrix(2, 3)
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(0, 1, math.SmallestNonzeroFloat64))
	lpStaticMust(t, m.Set(1, 0, .5))
	nonempty, err := m.PivotRowToRow(0, m.Columns[0][1], 0)
	lpStaticMust(t, err)
	if !nonempty || len(m.Rows[1]) != 1 || m.Rows[1][0].Value != 0 || !math.Signbit(m.Rows[1][0].Value) {
		t.Fatal(nonempty, m.Rows[1])
	}
	// The source debug precondition mistakenly compares target ROW against
	// COLUMN count; native behavior still admits a valid tall-matrix pivot.
	m = NewFloatStaticMatrix(3, 2)
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(0, 1, 2))
	lpStaticMust(t, m.Set(2, 0, 3))
	cell := m.Columns[0][1]
	nonempty, err = m.PivotRowToRow(0, cell, 0)
	if sourceDebugTests {
		if err != ErrAssertion || len(m.Rows[2]) != 1 {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
		if !nonempty || len(m.Rows[2]) != 1 || m.Rows[2][0].Value != -6 {
			t.Fatal(nonempty, m.Rows[2])
		}
	}
	// InitEmptyMatrix's old-width workspace can stop a real pivot after
	// removing the target pivot entry, before scanning its remaining terms.
	m = NewFloatStaticMatrix(0, 0)
	lpStaticMust(t, m.InitEmptyMatrix(2, 3))
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(1, 0, 2))
	lpStaticMust(t, m.Set(1, 1, 3))
	if _, err = m.PivotRowToRow(0, m.Columns[0][1], 0); err != ErrBounds {
		t.Fatal(err)
	}
	if len(m.Rows[1]) != 1 || m.Rows[1][0].Column != 1 || len(m.Columns[0]) != 1 {
		t.Fatal("removed pivot prefix", m.Rows, m.Columns)
	}
	// A native zero-valued pivot entry is still removed; debug rejects it.
	m = NewFloatStaticMatrix(2, 3)
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(0, 2, 1))
	lpStaticMust(t, m.Set(1, 0, 1))
	lpStaticMust(t, m.MultiplyRow(1, 0))
	_, err = m.PivotRowToRow(0, m.Columns[0][1], 0)
	if sourceDebugTests {
		if err != ErrAssertion || len(m.Columns[0]) != 2 {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
		if len(m.Columns[0]) != 1 || len(m.Rows[1]) != 1 || m.Rows[1][0].Value != 0 {
			t.Fatal(m.Rows)
		}
	}
	// Zero pivot-row coefficients reach their own assertion after the target
	// pivot is removed and the offset scan has completed.
	m = NewFloatStaticMatrix(2, 3)
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(0, 2, 1))
	lpStaticMust(t, m.Set(1, 0, 2))
	lpStaticMust(t, m.MultiplyRow(0, 0))
	_, err = m.PivotRowToRow(0, m.Columns[0][1], 0)
	if sourceDebugTests {
		if err != ErrAssertion || len(m.Rows[1]) != 0 {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
		if len(m.Rows[1]) != 1 || m.Rows[1][0].Value != 0 {
			t.Fatal(m.Rows)
		}
	}
	m = NewFloatStaticMatrix(2, 3)
	lpStaticMust(t, m.Set(0, 0, 1))
	lpStaticMust(t, m.Set(0, 1, 2))
	_, err = m.PivotRowToRow(0, m.Columns[0][0], 0)
	if sourceDebugTests {
		if err != ErrAssertion || len(m.Rows[0]) != 2 {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
		if len(m.Rows[0]) != 0 {
			t.Fatal(m.Rows)
		}
	}
	m = NewFloatStaticMatrix(2, 3)
	lpStaticMust(t, m.Set(1, 0, 2))
	lpStaticMust(t, m.Set(1, 1, 3))
	_, err = m.PivotRowToRow(99, m.Columns[0][0], 0)
	if sourceDebugTests {
		if err != ErrAssertion || len(m.Rows[1]) != 2 {
			t.Fatal(err)
		}
	} else if err != ErrBounds || len(m.Rows[1]) != 1 {
		t.Fatal(err, m.Rows)
	}
	if _, err = m.PivotRowToRow(0, ColumnCell{99, 0}, 0); err != func() error {
		if sourceDebugTests {
			return ErrAssertion
		}
		return ErrBounds
	}() {
		t.Fatal(err)
	}
}

func TestLPFloatStaticMatrixScopeAndCrosslinkStops(t *testing.T) {
	// Adding a column entry to an OLD row after Push violates the source
	// append-only scope contract. Native Pop leaves the dangling row term;
	// debug detects it only after shrinking the columns and consuming scope.
	m := NewFloatStaticMatrix(1, 1)
	m.Push()
	m.AddColumn()
	lpStaticMust(t, m.Set(0, 1, 2))
	err := m.Pop(1)
	if sourceDebugTests {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if m.ColumnCount() != 1 || len(m.stack) != 0 || len(m.Rows[0]) != 1 {
		t.Fatal("pop prefix")
	}
	if ok, err := m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
	// A row added after a scope expects its entries at column tails.
	m = NewFloatStaticMatrix(1, 1)
	m.Push()
	m.AddRow()
	lpStaticMust(t, m.Set(1, 0, 2))
	lpStaticMust(t, m.Set(0, 0, 3))
	err = m.Pop(1)
	if sourceDebugTests {
		if err != ErrAssertion || m.RowCount() != 2 || len(m.Columns[0]) != 2 {
			t.Fatal(err)
		}
	} else if err != nil || m.RowCount() != 1 || len(m.Columns[0]) != 1 {
		t.Fatal(err)
	}
	m = NewFloatStaticMatrix(0, 1)
	m.Push()
	m.AddRow()
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Columns[0] = nil
	if err = m.Pop(1); err != ErrBounds || m.RowCount() != 1 {
		t.Fatal(err)
	}
	// RemoveElement publishes the column-tail replacement before following
	// its row backlink; an invalid backlink retains that replacement prefix.
	m = NewFloatStaticMatrix(2, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	lpStaticMust(t, m.Set(1, 0, 3))
	m.Columns[0][1].Row = 99
	if err = m.RemoveElement(0, 0); err != ErrBounds || m.Columns[0][0].Row != 99 || len(m.Columns[0]) != 2 {
		t.Fatal(err, m.Columns)
	}
	m = NewFloatStaticMatrix(1, 2)
	lpStaticMust(t, m.Set(0, 0, 2))
	lpStaticMust(t, m.Set(0, 1, 3))
	m.Rows[0][1].Offset = 99
	if err = m.RemoveElement(0, 0); err != ErrBounds || m.Rows[0][0].Column != 1 || len(m.Rows[0]) != 2 {
		t.Fatal(err, m.Rows)
	}
	m = NewFloatStaticMatrix(1, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Rows[0][0].Offset = 99
	if err = m.RemoveElement(0, 0); err != ErrBounds {
		t.Fatal(err)
	}
	m = NewFloatStaticMatrix(1, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Columns[0][0].Offset = 99
	if err = m.RemoveElement(0, 0); err != ErrBounds {
		t.Fatal(err)
	}
	m = NewFloatStaticMatrix(2, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	m.Rows[0][0].Offset = 99
	if err = m.TransposeRows(0, 1); err != ErrBounds || len(m.Rows[1]) != 1 || len(m.Rows[0]) != 0 {
		t.Fatal(err, m.Rows)
	}
	// Diagnostic value mismatch and both explicit extent checks precede the
	// deliberately weak final self-comparison documented above.
	m = NewFloatStaticMatrix(2, 1)
	lpStaticMust(t, m.Set(0, 0, 2))
	lpStaticMust(t, m.Set(1, 0, 3))
	m.Rows[0][0].Offset = 1
	if ok, err := m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
	m.Rows[0][0].Offset = 99
	if ok, err := m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
	m = NewFloatStaticMatrix(1, 2)
	m.Columns[1] = []ColumnCell{{99, 0}}
	if ok, err := m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
	m.Columns[1] = []ColumnCell{{0, 99}}
	if ok, err := m.IsCorrect(io.Discard); err != nil || ok {
		t.Fatal(ok, err)
	}
}
