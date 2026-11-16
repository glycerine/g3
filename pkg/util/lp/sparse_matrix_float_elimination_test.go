package lp

import (
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
)

func lpFloatSparseCheck(t *testing.T, m *FloatSparseMatrix, want map[IndexPair]float64) {
	t.Helper()
	seen := make(map[IndexPair]bool)
	for i, row := range m.Rows {
		for k, v := range row {
			p := IndexPair{uint32(i), v.Index}
			w, ok := want[p]
			if !ok || seen[p] || v.Value != w {
				t.Fatal("row value", p, v.Value, w, ok)
			}
			seen[p] = true
			if int(v.Index) >= len(m.Columns) || int(v.Other) >= len(m.Columns[v.Index].Values) {
				t.Fatal("row link bounds")
			}
			c := m.Columns[v.Index].Values[v.Other]
			if c.Index != uint32(i) || c.Other != uint32(k) || c.Value != v.Value {
				t.Fatal("column backlink/value", p, c, v)
			}
		}
	}
	count := 0
	for j, column := range m.Columns {
		for k, v := range column.Values {
			count++
			if int(v.Index) >= len(m.Rows) || int(v.Other) >= len(m.Rows[v.Index]) {
				t.Fatal("column link bounds")
			}
			r := m.Rows[v.Index][v.Other]
			if r.Index != uint32(j) || r.Other != uint32(k) || r.Value != v.Value {
				t.Fatal("row backlink/value", r, v)
			}
		}
	}
	if count != len(want) || len(seen) != len(want) || int(m.ActiveElements()) != len(want) {
		t.Fatal("counts", count, len(want), m.ActiveElements())
	}
}

func TestLPFloatSparseIndependentMutationOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	m := newFloatSparseMatrix(9)
	rng := rand.New(rand.NewPCG(177, 81))
	want := make(map[IndexPair]float64)
	s := &Settings{DropTolerance: 1e-14, ZeroTolerance: 1e-12}
	for step := 0; step < 6000; step++ {
		i, j := rng.Uint32N(9), rng.Uint32N(9)
		p := IndexPair{i, j}
		switch rng.IntN(5) {
		case 0, 1:
			if _, exists := want[p]; !exists {
				v := float64(rng.IntN(19)-9) / 4
				lpStaticMust(t, m.AddNewElement(i, j, v))
				want[p] = v
			}
		case 2:
			if len(m.Rows[i]) > 0 {
				k := uint32(rng.IntN(len(m.Rows[i])))
				delete(want, IndexPair{i, m.Rows[i][k].Index})
				lpStaticMust(t, m.RemoveElement(i, k))
			}
		case 3:
			lpStaticMust(t, m.SetMaxInRow(i))
			if len(m.Rows[i]) > 0 {
				for _, v := range m.Rows[i] {
					if math.Abs(v.Value) > math.Abs(m.Rows[i][0].Value) {
						t.Fatal("row maximum")
					}
				}
			}
		case 4:
			divisor := math.Ldexp(1, rng.IntN(5)-2)
			lpStaticMust(t, m.DivideRow(i, &divisor, s))
			for p, v := range want {
				if p.I == i {
					v /= divisor
					if math.Abs(v) < s.DropTolerance {
						v = 0
					}
					want[p] = v
				}
			}
		}
		lpFloatSparseCheck(t, m, want)
	}
}

func TestLPFloatSparseIndependentPivotOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(499, 27))
	s := &Settings{DropTolerance: 1e-14, ZeroTolerance: 1e-12}
	zeroResults, cancellations, fills := 0, 0, 0
	for world := 0; world < 800; world++ {
		m := newFloatSparseMatrix(5)
		want := make(map[IndexPair]float64)
		for _, k := range rng.Perm(25) {
			if rng.IntN(3) == 0 {
				continue
			}
			v := float64(rng.IntN(9)-4) / 4
			i, j := uint32(k/5), uint32(k%5)
			lpStaticMust(t, m.AddNewElement(i, j, v))
			want[IndexPair{i, j}] = v
		}
		rows, columns := []uint32{0, 1, 2, 3, 4}, []uint32{0, 1, 2, 3, 4}
		for step := 0; step < 6; step++ {
			a := rng.Uint32N(5)
			b := (a + 1 + rng.Uint32N(4)) % 5
			lpStaticMust(t, m.SwapRows(a, b))
			rows[a], rows[b] = rows[b], rows[a]
			a = rng.Uint32N(5)
			b = (a + 1 + rng.Uint32N(4)) % 5
			lpStaticMust(t, m.SwapColumns(a, b))
			columns[a], columns[b] = columns[b], columns[a]
		}
		pivot := rng.Uint32N(5)
		target := (pivot + 1 + rng.Uint32N(4)) % 5
		alpha := float64(rng.IntN(7)-3) / 2
		delete(want, IndexPair{rows[target], columns[pivot]})
		for j := uint32(0); j < 5; j++ {
			if j == columns[pivot] {
				continue
			}
			v, exists := want[IndexPair{rows[pivot], j}]
			if !exists {
				continue
			}
			p := IndexPair{rows[target], j}
			old, present := want[p]
			result := old + float64(alpha*v)
			if math.Abs(result) < s.DropTolerance {
				if present {
					cancellations++
				}
				delete(want, p)
			} else {
				if !present {
					fills++
				}
				want[p] = result
			}
		}
		ok, err := m.PivotRowToRow(pivot, &alpha, target, s)
		lpStaticMust(t, err)
		n := 0
		for p := range want {
			if p.I == rows[target] {
				n++
			}
		}
		if ok != (n > 0) {
			t.Fatal("empty status", ok, n)
		}
		if !ok {
			zeroResults++
		}
		lpFloatSparseCheck(t, m, want)
		for _, offset := range m.WorkPivot {
			if offset != -1 {
				t.Fatal("uncleared workspace", offset)
			}
		}
		if n > 0 {
			for _, v := range m.Rows[rows[target]] {
				if math.Abs(v.Value) > math.Abs(m.Rows[rows[target]][0].Value) {
					t.Fatal("pivot maximum")
				}
			}
		}
	}
	if zeroResults == 0 || cancellations == 0 || fills == 0 {
		t.Fatal("oracle missed paths", zeroResults, cancellations, fills)
	}
}

func TestLPFloatSparseLiteralEliminationAndDivision(t *testing.T) {
	s := &Settings{DropTolerance: 1, ZeroTolerance: 1e-12}
	t.Run("self pivot assertion profile", func(t *testing.T) {
		m := newFloatSparseMatrix(3)
		for j, value := range []float64{1, 2, 3} {
			lpStaticMust(t, m.AddNewElement(0, uint32(j), value))
		}
		alpha := 1.0
		ok, err := m.PivotRowToRow(0, &alpha, 0, s)
		if sourceDebugTests {
			if err != ErrAssertion || ok || m.active != 3 {
				t.Fatal("self assertion prefix", ok, err)
			}
		} else {
			lpStaticMust(t, err)
			if !ok {
				t.Fatal("self release continuation")
			}
			lpFloatSparseCheck(t, m, map[IndexPair]float64{{0, 1}: 4, {0, 2}: 6})
		}
	})
	t.Run("strict drop and zero storage", func(t *testing.T) {
		m := newFloatSparseMatrix(4)
		for _, e := range []struct {
			i, j uint32
			v    float64
		}{{0, 0, 1}, {0, 1, 1}, {0, 2, .5}, {0, 3, 2}, {1, 0, 9}, {1, 1, -1}, {1, 2, .5}} {
			lpStaticMust(t, m.AddNewElement(e.i, e.j, e.v))
		}
		a := 1.0
		ok, err := m.PivotRowToRow(0, &a, 1, s)
		lpStaticMust(t, err)
		if !ok {
			t.Fatal("empty result")
		}
		lpFloatSparseCheck(t, m, map[IndexPair]float64{{0, 0}: 1, {0, 1}: 1, {0, 2}: .5, {0, 3}: 2, {1, 2}: 1, {1, 3}: 2})
		if m.Rows[1][0].Index != 3 {
			t.Fatal("maximum not moved")
		}
		divisor := 2.0
		lpStaticMust(t, m.DivideRow(1, &divisor, s))
		lpFloatSparseCheck(t, m, map[IndexPair]float64{{0, 0}: 1, {0, 1}: 1, {0, 2}: .5, {0, 3}: 2, {1, 2}: 0, {1, 3}: 1})
	})
	t.Run("live divisor", func(t *testing.T) {
		m := newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(1, 0, 2))
		lpStaticMust(t, m.AddNewElement(1, 1, 6))
		lpStaticMust(t, m.AddNewElement(1, 2, 10))
		lpStaticMust(t, m.SwapRows(0, 1))
		lpStaticMust(t, m.DivideRow(0, &m.Rows[1][0].Value, s))
		lpFloatSparseCheck(t, m, map[IndexPair]float64{{1, 0}: 1, {1, 1}: 6, {1, 2}: 10})
	})
	t.Run("empty pivot and reverse cancellation order", func(t *testing.T) {
		m := newFloatSparseMatrix(4)
		for _, j := range []uint32{3, 1, 2} {
			lpStaticMust(t, m.AddNewElement(0, j, 1))
		}
		for _, j := range []uint32{0, 1, 2, 3} {
			lpStaticMust(t, m.AddNewElement(1, j, -1))
		}
		a := 1.0
		ok, err := m.PivotRowToRow(0, &a, 1, s)
		lpStaticMust(t, err)
		if ok || len(m.Rows[1]) != 0 {
			t.Fatal("all cancellations")
		}
		lpFloatSparseCheck(t, m, map[IndexPair]float64{{0, 1}: 1, {0, 2}: 1, {0, 3}: 1})
		ok, err = m.PivotRowToRow(2, &a, 3, s)
		lpStaticMust(t, err)
		if ok {
			t.Fatal("empty untouched row")
		}
	})
	t.Run("duplicate new terms stay separate", func(t *testing.T) {
		m := newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(0, 2, 2))
		lpStaticMust(t, m.AddNewElement(0, 2, 3))
		a := 1.0
		ok, err := m.PivotRowToRow(0, &a, 1, s)
		lpStaticMust(t, err)
		if !ok || len(m.Rows[1]) != 2 || m.Rows[1][0].Value != 3 || m.Rows[1][1].Value != 2 || m.active != 4 {
			t.Fatal("literal duplicate publication")
		}
		for k, v := range m.Rows[1] {
			if c := m.Columns[v.Index].Values[v.Other]; c.Other != uint32(k) || c.Index != 1 || c.Value != v.Value {
				t.Fatal("duplicate backlink")
			}
		}
	})
	t.Run("duplicate cancellation keeps literal removal prefix", func(t *testing.T) {
		m := newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(0, 2, 1))
		lpStaticMust(t, m.AddNewElement(0, 2, 0))
		lpStaticMust(t, m.AddNewElement(1, 1, 7))
		lpStaticMust(t, m.AddNewElement(1, 2, -1))
		alpha := 1.0
		ok, err := m.PivotRowToRow(0, &alpha, 1, s)
		// Both source entries queue offset1. Its first deletion succeeds;
		// the second reaches the source's invalid offset, with no deduplication.
		if ok || err != ErrBounds || m.active != 3 || len(m.Rows[1]) != 1 || m.Rows[1][0].Value != 7 {
			t.Fatal("duplicate removal prefix", ok, err, m.Rows[1])
		}
	})
	t.Run("IEEE and tolerance edges", func(t *testing.T) {
		m := newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(0, 0, math.NaN()))
		lpStaticMust(t, m.AddNewElement(0, 1, math.Inf(1)))
		lpStaticMust(t, m.AddNewElement(0, 2, math.Inf(-1)))
		lpStaticMust(t, m.SetMaxInRow(0))
		if !math.IsNaN(m.Rows[0][0].Value) {
			t.Fatal("NaN head ordering")
		}
		m.Rows[0][0].Value = -2
		m.Columns[0].Values[0].Value = -2
		lpStaticMust(t, m.SetMaxInRow(0))
		if m.Rows[0][0].Index != 1 {
			t.Fatal("first infinity wins")
		}
		zero := 0.0
		err := m.DivideRow(0, &zero, s)
		if sourceDebugTests {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else {
			lpStaticMust(t, err)
			if !math.IsInf(m.Rows[0][2].Value, -1) {
				t.Fatal("release zero division")
			}
		}
		m = newFloatSparseMatrix(2)
		lpStaticMust(t, m.AddNewElement(0, 1, 0))
		s.DropTolerance = 0
		a := 0.0
		ok, err := m.PivotRowToRow(0, &a, 1, s)
		lpStaticMust(t, err)
		if !ok || len(m.Rows[1]) != 1 || m.Rows[1][0].Value != 0 {
			t.Fatal("zero tolerance retains zero")
		}
		s.DropTolerance = 1
	})
}

func TestLPFloatSparseMutationBoundsAndPrefixes(t *testing.T) {
	s := &Settings{DropTolerance: 1e-14, ZeroTolerance: 1e-12}
	alpha := 1.0
	m := newFloatSparseMatrix(3)
	for _, f := range []func() error{
		func() error { return m.AddNewElement(3, 0, 1) }, func() error { return m.AddNewElement(0, 3, 1) }, func() error { return m.RemoveElement(0, 0) },
		func() error { return m.SetMaxInRow(3) }, func() error { return m.putMaxFirst(0, 1) }, func() error { return m.scanRowAndRemovePivot(3, 0) },
		func() error { return m.DivideRow(3, &alpha, s) }, func() error { return m.removeAt(0, 0, 0, 0) },
	} {
		if !errors.Is(f(), ErrBounds) {
			t.Fatal("missing bounds")
		}
	}
	for _, pair := range [][2]uint32{{3, 1}, {0, 3}} {
		_, err := m.PivotRowToRow(pair[0], &alpha, pair[1], s)
		if sourceDebugTests {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != ErrBounds {
			t.Fatal(err)
		}
	}
	// Tail assignment happens before validating the copied tail's backlink.
	for _, rowTail := range []bool{false, true} {
		m = newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(0, 0, 1))
		lpStaticMust(t, m.AddNewElement(0, 1, 2))
		lpStaticMust(t, m.AddNewElement(1, 0, 3))
		if rowTail {
			m.Rows[0][1].Other = 99
		} else {
			m.Columns[0].Values[1].Other = 99
		}
		if err := m.RemoveElement(0, 0); err != ErrBounds {
			t.Fatal(err)
		}
		if m.Columns[0].Values[0].Value != 3 || len(m.Columns[0].Values) != 2 || m.active != 3 {
			t.Fatal("column assignment prefix")
		}
		if rowTail && m.Rows[0][0].Value != 2 {
			t.Fatal("row assignment prefix")
		}
	}
	// Max-column repair precedes the malformed old-head lookup and row swap.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 0, 1))
	lpStaticMust(t, m.AddNewElement(0, 1, 3))
	m.Rows[0][0].Other = 99
	if err := m.SetMaxInRow(0); err != ErrBounds {
		t.Fatal(err)
	}
	if m.Columns[1].Values[0].Other != 0 || m.Rows[0][0].Index != 0 {
		t.Fatal("max repair prefix")
	}
	// Division mutates the row before looking up its paired column value.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 0, 4))
	m.Rows[0][0].Other = 99
	d := 2.0
	if err := m.DivideRow(0, &d, s); err != ErrBounds || m.Rows[0][0].Value != 2 || m.Columns[0].Values[0].Value != 4 {
		t.Fatal("division prefix", err)
	}
	// No implicit workspace repair at entry: source assumes prior cleanup.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 2, 2))
	m.WorkPivot[2] = -2
	_, err := m.PivotRowToRow(0, &alpha, 1, s)
	if err != ErrBounds {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.WorkPivot, []int32{-1, -1, -2}) {
		t.Fatal("workspace repaired")
	}
	// A stale current reciprocal link stops before any tail assignments.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 0, 1))
	m.Rows[0][0].Other = 9
	if err := m.RemoveElement(0, 0); err != ErrBounds || m.active != 1 {
		t.Fatal(err)
	}
	// This second-tail repair follows the branch with no column-tail copy.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 0, 1))
	lpStaticMust(t, m.AddNewElement(0, 1, 2))
	m.Rows[0][1].Other = 9
	if err := m.RemoveElement(0, 0); err != ErrBounds || m.Rows[0][0].Value != 2 {
		t.Fatal(err)
	}
	// The selected maximum has an invalid link, so no backlink is repaired.
	m = newFloatSparseMatrix(3)
	lpStaticMust(t, m.AddNewElement(0, 0, 1))
	lpStaticMust(t, m.AddNewElement(0, 1, 2))
	m.Rows[0][1].Other = 9
	if err := m.SetMaxInRow(0); err != ErrBounds || m.Columns[0].Values[0].Other != 0 {
		t.Fatal(err)
	}
	for _, phase := range []string{"scan", "pivot removal", "product index", "new column", "paired update", "max after fill"} {
		m = newFloatSparseMatrix(3)
		lpStaticMust(t, m.AddNewElement(0, 2, 2))
		switch phase {
		case "scan":
			lpStaticMust(t, m.AddNewElement(1, 2, 3))
			m.WorkPivot = m.WorkPivot[:2]
		case "pivot removal":
			lpStaticMust(t, m.AddNewElement(1, 0, 3))
			m.Rows[1][0].Other = 99
		case "product index":
			m.WorkPivot = m.WorkPivot[:2]
		case "new column":
			m.Columns = m.Columns[:2]
		case "paired update":
			lpStaticMust(t, m.AddNewElement(1, 2, 3))
			m.Rows[1][0].Other = 99
		case "max after fill":
			lpStaticMust(t, m.AddNewElement(1, 1, 1))
			m.Rows[1][0].Other = 99
		}
		if _, err := m.PivotRowToRow(0, &alpha, 1, s); err != ErrBounds {
			t.Fatal(phase, err)
		}
		if phase == "paired update" && m.Rows[1][0].Value != 5 {
			t.Fatal("addition before paired update")
		}
		if phase == "max after fill" && m.Columns[2].Values[1].Other != 0 {
			t.Fatal("new maximum backlink prefix")
		}
	}
}
