package lp

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

func TestLPRationalPreparationIndependentMatricesAndRHS(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	rng := rand.New(rand.NewSource(180527))
	for world := 0; world < 160; world++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		nr, nc := 1+rng.Intn(5), 1+rng.Intn(7)
		ro, co := rng.Perm(nr), rng.Perm(nc)
		shift := make([]*big.Rat, nc)
		for j := range shift {
			ext := uint32(10 + 7*j)
			shift[j] = new(big.Rat)
			c, err := m.GetOrCreateColumnInfo(ext)
			h.must(err)
			bound := big.NewRat(int64(world-41+j*3), 17)
			switch j % 4 {
			case 0, 2:
				h.must(c.SetLowBound(&h.pair(bound.RatString(), "0").X))
				shift[j].Set(bound)
				if j%4 == 2 {
					upper := new(big.Rat).Add(bound, big.NewRat(7, 13))
					h.must(c.SetUpperBound(&h.pair(upper.RatString(), "0").X))
				}
			case 1:
				h.must(c.SetUpperBound(&h.pair(bound.RatString(), "0").X))
				shift[j].Set(bound)
			}
		}
		a := make([][]*big.Rat, nr)
		rhs, rowShift, transformed := make([]*big.Rat, nr), make([]*big.Rat, nr), make([]*big.Rat, nr)
		for _, i := range ro {
			a[i] = make([]*big.Rat, nc)
			rowShift[i] = new(big.Rat)
			rhs[i] = big.NewRat(int64(rng.Intn(29)-14), 19)
			h.must(m.AddConstraint(LPRelation(i%3), &h.pair(rhs[i].RatString(), "0").X, uint32(100+11*i)))
			for _, j := range co {
				n := new(big.Int).Lsh(big.NewInt(1), uint(81+world))
				n.Add(n, big.NewInt(int64(1+i*nc+j)))
				if rng.Intn(2) == 0 {
					n.Neg(n)
				}
				a[i][j] = new(big.Rat).SetFrac(n, big.NewInt(23))
				h.must(m.SetRowColumnCoefficient(uint32(100+11*i), uint32(10+7*j), &h.pair(a[i][j].RatString(), "0").X))
				rowShift[i].Add(rowShift[i], new(big.Rat).Mul(a[i][j], shift[j]))
			}
			transformed[i] = new(big.Rat).Sub(rhs[i], rowShift[i])
		}
		h.must(m.FillMatrixAAndInitRightSide())
		if m.A.RowCount() != uint32(nr) || m.A.ColumnCount() != uint32(nc) || len(m.B.Data) != nr {
			t.Fatal("prepared extents")
		}
		// Core rows and columns are numbered in m_A_values' libstdc++ order
		// (row keys, then each row's column keys by first appearance).
		ro, co = ro[:0], co[:0]
		seenColumn := map[uint32]bool{}
		for _, ext := range m.AValues.Keys() {
			ro = append(ro, int(ext-100)/11)
			row, _ := m.AValues.Get2(ext)
			for _, col := range row.Keys() {
				if !seenColumn[col] {
					seenColumn[col] = true
					co = append(co, int(col-10)/7)
				}
			}
		}
		for r, i := range ro {
			if m.ExternalRowsToCore[uint32(100+11*i)] != uint32(r) || m.CoreRowsToExternal[uint32(r)] != uint32(100+11*i) {
				t.Fatal("row mapping")
			}
			out := h.pair("91", "0")
			h.must(m.LowBoundShiftForRow(uint32(100+11*i), &out.X))
			lpSparseRationalCheck(t, h, &out.X, rowShift[i])
			for c, j := range co {
				info, _ := m.Columns.Get2(uint32(10 + 7*j))
				k := -1
				for p := range m.A.Rows[r] {
					if m.A.Rows[r][p].Column == uint32(c) {
						k = p
					}
				}
				if k < 0 || info.Index != uint32(c) || m.CoreColumnsToExternal[uint32(c)] != uint32(10+7*j) {
					t.Fatal("column mapping")
				}
				cell := &m.A.Rows[r][k]
				link := m.A.Columns[cell.Column][cell.Offset]
				if link.Row != uint32(r) || link.Offset != uint32(k) {
					t.Fatal("reciprocal matrix links")
				}
				want := new(big.Rat).Set(a[i][j])
				if j%4 == 1 {
					want.Neg(want)
				}
				lpSparseRationalCheck(t, h, &cell.Value, want)
			}
		}
		h.must(m.FillB())
		var slacks, artificials uint32
		for r, i := range ro {
			lpSparseRationalCheck(t, h, &m.B.Data[r], transformed[i])
			if i%3 == int(RelationEqual) {
				artificials++
			} else {
				slacks++
				if i%3 == int(RelationLessOrEqual) && transformed[i].Sign() < 0 || i%3 == int(RelationGreaterOrEqual) && transformed[i].Sign() > 0 {
					artificials++
				}
			}
		}
		h.must(m.CountSlacksAndArtificials())
		if m.Slacks != slacks || m.Artificials != artificials {
			t.Fatal("auxiliary counts")
		}
		// Precise scaling does not read the floating scaler or its settings.
		m.Settings = nil
		h.must(m.Scale())
		for j := range m.ColumnScale.Data {
			lpSparseRationalCheck(t, h, &m.ColumnScale.Data[j], big.NewRat(1, 1))
		}
		old := m.A
		h.must(h.m.SetInt64(&m.A.Rows[0][0].Value, 99))
		h.must(h.m.SetInt64(&m.B.Data[0], 99))
		h.must(h.m.SetInt64(&m.ColumnScale.Data[0], 99))
		h.must(m.Unscale())
		if old == m.A || len(old.Rows) != 0 {
			t.Fatal("old matrix was not destroyed before reconstruction")
		}
		for r, i := range ro {
			lpSparseRationalCheck(t, h, &m.B.Data[r], transformed[i])
			for c, j := range co {
				want := new(big.Rat).Set(a[i][j])
				if j%4 == 1 {
					want.Neg(want)
				}
				lpSparseRationalCheck(t, h, &m.A.Rows[r][c].Value, want)
			}
		}
		for j := range m.ColumnScale.Data {
			lpSparseRationalCheck(t, h, &m.ColumnScale.Data[j], big.NewRat(1, 1))
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPreparationSourceMappingPrefixes(t *testing.T) {
	for _, kind := range []string{"zero", "fixed", "missing", "signed-index", "repeated"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		one, zero := h.pair("7/13", "0"), h.pair("0", "0")
		c, err := m.GetOrCreateColumnInfo(10)
		h.must(err)
		// Inserting 20 before 10 makes libstdc++ visit column 10 first.
		h.must(m.SetRowColumnCoefficient(100, 20, &one.X))
		h.must(m.SetRowColumnCoefficient(100, 10, &one.X))
		wantErr := error(nil)
		switch kind {
		case "zero":
			h.must(m.SetRowColumnCoefficient(100, 20, &zero.X))
			wantErr = ErrLPFixedColumn
		case "fixed":
			h.must(m.SetFixedValue(20, &one.X))
			wantErr = ErrLPFixedColumn
		case "missing":
			wantErr = ErrBounds
		case "signed-index":
			c.Index = 0x80000001
			_, err = m.GetOrCreateColumnInfo(20)
			h.must(err)
		case "repeated":
			_, err = m.GetOrCreateColumnInfo(20)
			h.must(err)
			h.must(m.FillMatrixAAndInitRightSide())
			if sourceDebugEnabled {
				wantErr = ErrAssertion
			}
		}
		err = m.FillMatrixAAndInitRightSide()
		if err != wantErr || c.Index != 0 || m.ExternalRowsToCore[100] != 0 {
			t.Fatal(kind, err, c.Index)
		}
		if kind == "zero" {
			if _, exists := m.Columns.Get2(20); exists {
				t.Fatal("zero short circuit inserted column")
			}
		}
		if kind == "missing" {
			if v, exists := m.Columns.Get2(20); !exists || v != nil {
				t.Fatal("source operator[] publication")
			}
		}
		if kind == "zero" || kind == "fixed" || kind == "missing" {
			if m.A != nil || len(m.B.Data) != 0 {
				t.Fatal("matrix published before column mapping completed")
			}
		}
		if kind == "repeated" && !sourceDebugEnabled && len(m.previousMatrices) != 1 {
			t.Fatal("displaced matrix owner missing")
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPreparationDenseResizeAndExactScale(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
	h.must(m.B.resizeData(3))
	for i := range m.B.Data {
		h.must(m.B.copySlot(&m.B.Data[i], &large.X))
	}
	// Empty coefficient rows still obtain core row IDs.
	m.AValues.Set(50, NewStdUnorderedRationalMap())
	m.AValues.Set(70, NewStdUnorderedRationalMap())
	h.must(m.FillMatrixAAndInitRightSide())
	if len(m.B.Data) != 2 || m.A.ColumnCount() != 0 {
		t.Fatal("dense shrink")
	}
	for i := range m.B.Data {
		want, _ := new(big.Rat).SetString(strings.Repeat("1234567890", 20) + "7/13")
		lpSparseRationalCheck(t, h, &m.B.Data[i], want)
	}
	h.must(m.ColumnScale.resizeData(4))
	for i := range m.ColumnScale.Data {
		h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[i], &large.X))
	}
	h.budget.FailAfter(0)
	err := m.Scale()
	h.budget.FailAfter(-1)
	h.must(err)
	if len(m.ColumnScale.Data) != 0 {
		t.Fatal("zero structural scales")
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)

	h = newLPPairHarness(t)
	m = lpRationalInputModel(t, h)
	h.must(m.ColumnScale.resizeData(1))
	h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[0], &h.pair("7/13", "0").X))
	if m.Scale() != ErrBounds || len(m.ColumnScale.Data) != 0 {
		t.Fatal("source clear precedes missing matrix access")
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPreparationDescendingRHSAndCounters(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	for i := uint32(0); i < 3; i++ {
		m.AValues.Set(10+i, NewStdUnorderedRationalMap())
		if i != 1 {
			h.must(m.AddConstraint(RelationEqual, &h.pair(fmt.Sprint(7+i), "0").X, 10+i))
		}
	}
	h.must(m.FillMatrixAAndInitRightSide())
	for i := range m.B.Data {
		h.must(h.m.SetInt64(&m.B.Data[i], 91))
	}
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err := m.FillB(); err != want {
		t.Fatal(err)
	}
	// libstdc++ numbers the rows 12, 11, 10: fill_m_b writes core row 2
	// (external 10) and stops at the missing constraint of external 11.
	if m.CoreRowsToExternal[2] != 10 || m.CoreRowsToExternal[1] != 11 {
		t.Fatal("core row order", m.CoreRowsToExternal)
	}
	lpSparseRationalCheck(t, h, &m.B.Data[2], big.NewRat(7, 1))
	lpSparseRationalCheck(t, h, &m.B.Data[1], big.NewRat(91, 1))
	lpSparseRationalCheck(t, h, &m.B.Data[0], big.NewRat(91, 1))
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
	for _, relation := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual, LPRelation(99)} {
		for _, sign := range []int64{-1, 0, 1} {
			h := newLPPairHarness(t)
			m := lpRationalInputModel(t, h)
			h.must(m.AddConstraint(relation, &h.pair(fmt.Sprint(sign), "0").X, 0))
			h.must(m.B.resizeData(1))
			h.must(h.m.SetInt64(&m.B.Data[0], sign))
			m.Artificials, m.Slacks = ^uint32(0), ^uint32(0)
			h.must(m.CountSlacksAndArtificialsForRow(0))
			wa, ws := ^uint32(0), ^uint32(0)
			if relation == RelationEqual {
				wa++
			} else if relation == RelationLessOrEqual || relation == RelationGreaterOrEqual {
				ws++
				if relation == RelationLessOrEqual && sign < 0 || relation == RelationGreaterOrEqual && sign > 0 {
					wa++
				}
			}
			if m.Artificials != wa || m.Slacks != ws || m.CoreRowsToExternal[0] != 0 {
				t.Fatal("source unsigned counters")
			}
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPRationalPreparationPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"matrix", "matrix-flipped", "shift", "rhs", "unscale"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 650 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					m := lpRationalInputModel(t, h)
					large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
					out := h.pair("91", "0")
					for i := uint32(0); i < 2; i++ {
						h.must(m.AddConstraint(RelationEqual, &large.X, 100+i))
						for j := uint32(0); j < 2; j++ {
							h.must(m.SetRowColumnCoefficient(100+i, 10+j, &large.X))
						}
					}
					for j := uint32(0); j < 2; j++ {
						if phase == "matrix-flipped" {
							h.must(m.SetUpperBound(10+j, &large.X))
						} else {
							h.must(m.SetLowBound(10+j, &large.X))
						}
					}
					if phase == "rhs" || phase == "unscale" {
						h.must(m.FillMatrixAAndInitRightSide())
					}
					if phase == "unscale" {
						h.must(m.FillB())
						h.must(m.Scale())
					}
					h.budget.FailAfter(ordinal)
					var err error
					var old *RationalStaticMatrix
					switch phase {
					case "matrix", "matrix-flipped":
						err = m.FillMatrixAAndInitRightSide()
					case "shift":
						err = m.LowBoundShiftForRow(100, &out.X)
					case "rhs":
						err = m.FillB()
					case "unscale":
						old = m.A
						err = m.Unscale()
					}
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if m.FillB() != ErrExactVectorPending {
							t.Fatal("stopped preparation accepted retry")
						}
					}
					if old != nil && len(old.Rows) != 0 {
						t.Fatal("unscale failed before destroying old matrix")
					}
					h.must(m.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("physical preparation campaign incomplete", direct, phase, failures)
			}
		}
	}
}

func TestLPRationalPreparationSeparateEntriesAndRepeatedMapping(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	v := h.pair("7/13", "0")
	for _, j := range []uint32{10, 20} {
		_, err := m.GetOrCreateColumnInfo(j)
		h.must(err)
		h.must(m.SetRowColumnCoefficient(100, j, &v.X))
	}
	h.must(m.MapExternalRowsToCoreSolverRows())
	h.must(m.MapExternalColumnsToCoreSolverColumns())
	h.must(m.FillAFromAValues())
	h.must(m.ColumnScale.resizeData(2))
	for i := range m.ColumnScale.Data {
		h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[i], &v.X))
	}
	h.must(m.RestoreColumnScalesToOne())
	for i := range m.ColumnScale.Data {
		lpSparseRationalCheck(t, h, &m.ColumnScale.Data[i], big.NewRat(1, 1))
	}
	_, err := m.GetOrCreateColumnInfo(30)
	h.must(err)
	h.must(m.SetRowColumnCoefficient(100, 30, &v.X))
	h.must(m.MapExternalColumnsToCoreSolverColumns())
	c, _ := m.Columns.Get2(30)
	if c.Index != 0 || m.CoreColumnsToExternal[0] != 30 || len(m.CoreColumnsToExternal) != 2 {
		t.Fatal("source repeated mapping resets newcomer counter")
	}
	// A direct refill has no existing-matrix assertion. Preserve the displaced
	// owner until destruction while the source resets the matrix pointer.
	h.must(m.FillAFromAValues())
	if len(m.previousMatrices) != 1 || m.A.ColumnCount() != 2 || len(m.A.Rows[0]) != 3 {
		t.Fatal("source direct refill")
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)

	h = newLPPairHarness(t)
	m = lpRationalInputModel(t, h)
	h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("7/13", "0").X))
	h.must(m.SetLowBound(10, &h.pair("-1", "0").X))
	h.must(m.SetUpperBound(10, &h.pair("9", "0").X))
	h.must(m.SetFixedValue(10, &h.pair("4", "0").X))
	out := h.pair("91", "0")
	h.must(m.LowBoundShiftForRow(100, &out.X))
	lpSparseRationalCheck(t, h, &out.X, big.NewRat(28, 13))
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)

	h = newLPPairHarness(t)
	m = lpRationalInputModel(t, h)
	h.must(m.Unscale()) // Empty source reconstruction is valid without old A.
	if m.A.RowCount() != 0 || m.A.ColumnCount() != 0 {
		t.Fatal("empty reconstruction")
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPreparationReachedDomains(t *testing.T) {
	for phase := 0; phase < 19; phase++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		out := h.pair("91", "0")
		v := h.pair("7/13", "0")
		var err error
		want := ErrBounds
		switch phase {
		case 0:
			err = m.FillB()
		case 1:
			err = m.CountSlacksAndArtificials()
		case 2:
			err = m.LowBoundShiftForRow(100, nil)
		case 3:
			err = m.LowBoundShiftForRow(100, &out.X)
			want = ErrLPSolverMissingRow
			lpSparseRationalCheck(t, h, &out.X, new(big.Rat))
		case 4:
			m.AValues.Set(100, nil)
			err = m.MapExternalColumnsToCoreSolverColumns()
		case 5:
			m.AValues.Set(100, NewStdUnorderedRationalMap())
			row, _ := m.AValues.Get2(100)
			row.Set(10, nil)
			err = m.MapExternalColumnsToCoreSolverColumns()
		case 6:
			m.AValues.Set(100, nil)
			err = m.LowBoundShiftForRow(100, &out.X)
		case 7:
			h.must(m.SetRowColumnCoefficient(100, 10, &v.X))
			err = m.LowBoundShiftForRow(100, &out.X)
			if c, exists := m.Columns.Get2(10); !exists || c != nil {
				t.Fatal("shift operator[] missing-column insertion")
			}
		case 8, 9, 10, 11, 12:
			h.must(m.SetRowColumnCoefficient(100, 10, &v.X))
			_, e := m.GetOrCreateColumnInfo(10)
			h.must(e)
			if phase != 8 {
				h.must(m.MapExternalRowsToCoreSolverRows())
			}
			if phase == 9 {
				m.Columns = NewStdUnorderedRationalColumns()
			}
			if phase == 10 {
				m.AValues.Set(100, nil)
			}
			if phase == 11 {
				row, _ := m.AValues.Get2(100)
				row.Set(10, nil)
			}
			err = m.FillAFromAValues()
			if (phase == 8 || phase == 9 || phase == 12) && sourceDebugEnabled {
				want = ErrAssertion
			}
			if m.A == nil {
				t.Fatal("source matrix pointer precedes fill guard")
			}
		case 13:
			m.A = NewRationalStaticMatrix(h.m, 1, 0)
			h.must(m.AddConstraint(RelationEqual, &v.X, 0))
			err = m.FillB()
			want = ErrLPSolverMissingRow
			if _, exists := m.CoreRowsToExternal[0]; !exists {
				t.Fatal("RHS map default insertion")
			}
		case 14:
			m.A = NewRationalStaticMatrix(h.m, 1, 0)
			h.must(m.AddConstraint(RelationEqual, &v.X, 0))
			m.AValues.Set(0, NewStdUnorderedRationalMap())
			err = m.FillB()
		case 15:
			h.must(m.AddConstraint(RelationLessOrEqual, &v.X, 0))
			err = m.CountSlacksAndArtificialsForRow(0)
			if m.Slacks != 1 {
				t.Fatal("slack increment must precede B extent guard")
			}
		case 16:
			m.A = NewRationalStaticMatrix(h.m, 1, 0)
			err = m.CountSlacksAndArtificials()
			if sourceDebugEnabled {
				want = ErrAssertion
			}
		case 17:
			row := NewStdUnorderedRationalMap()
			row.Set(10, nil)
			m.AValues.Set(100, row)
			err = m.LowBoundShiftForRow(100, &out.X)
		case 18:
			h.must(m.SetRowColumnCoefficient(100, 10, &v.X))
			h.must(m.SetLowBound(10, &v.X))
			c, _ := m.Columns.Get2(10)
			c.stopped = true
			err = m.LowBoundShiftForRow(100, &out.X)
			want = ErrExactVectorPending
		}
		if err != want {
			t.Fatal(phase, err, want)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for _, op := range []string{"rows", "columns", "fill-a", "prepare", "shift", "fill-b", "restore", "scale", "unscale", "count-row", "count-all"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		m.stopped = true
		var err error
		switch op {
		case "rows":
			err = m.MapExternalRowsToCoreSolverRows()
		case "columns":
			err = m.MapExternalColumnsToCoreSolverColumns()
		case "fill-a":
			err = m.FillAFromAValues()
		case "prepare":
			err = m.FillMatrixAAndInitRightSide()
		case "shift":
			err = m.LowBoundShiftForRow(0, nil)
		case "fill-b":
			err = m.FillB()
		case "restore":
			err = m.RestoreColumnScalesToOne()
		case "scale":
			err = m.Scale()
		case "unscale":
			err = m.Unscale()
		case "count-row":
			err = m.CountSlacksAndArtificialsForRow(0)
		case "count-all":
			err = m.CountSlacksAndArtificials()
		}
		if err != ErrExactVectorPending {
			t.Fatal(op, err)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPreparationActualMPSPipeline(t *testing.T) {
	h := newLPPairHarness(t)
	text := "NAME MODEL\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "R", "7/13") + lpExactMPSPair("y", "R", "-5/17") + "RHS\n" + lpExactMPSPair("RHS1", "R", "11/19") + "BOUNDS\n" + lpMPSBoundLine("LO", "x -3/23") + lpMPSBoundLine("MI", "y") + lpMPSBoundLine("UP", "y 2/29") + "ENDATA\n"
	r := lpReadExactMPS(t, h, text)
	m := lpRationalInputModel(t, h)
	h.must(r.fillRationalSolver(m))
	h.must(m.FillMatrixAAndInitRightSide())
	h.must(m.FillB())
	h.must(m.Scale())
	// MI retains the source lower flag, so y remains boxed and unflipped.
	col, _ := m.Columns.Get2(1)
	if col.IsFlipped() || col.ColumnType() != ColumnBoxed {
		t.Fatal("source MI lower flag")
	}
	want := new(big.Rat).Sub(big.NewRat(11, 19), new(big.Rat).Mul(big.NewRat(7, 13), big.NewRat(-3, 23)))
	lpSparseRationalCheck(t, h, &m.B.Data[0], want)
	lpSparseRationalCheck(t, h, &m.A.Rows[0][1].Value, big.NewRat(-5, 17))
	h.must(m.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPreparationPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
		h.must(m.SetLowBound(10, &large.X))
		h.must(m.SetRowColumnCoefficient(100, 10, &large.X))
		h.must(m.AddConstraint(RelationEqual, &large.X, 100))
		h.must(m.FillMatrixAAndInitRightSide())
		v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
		var caller exactEtaCalls
		b, err := caller.begin(h.m)
		h.must(err)
		h.budget.FailAfter(ordinal)
		_, err = h.m.Compare(&v.X, &v.Y)
		h.budget.FailAfter(-1)
		err = caller.after(h.m, b, err)
		older := h.m.ComparisonInvocationState()
		if older.Active {
			matched = true
			if !lpExactPriorityPhysicalRefusal(err) {
				t.Fatal(err)
			}
			_ = m.FillB()
			if m.calls.comparison != 0 || m.B.calls.comparison != 0 {
				t.Fatal("preparation adopted older comparison")
			}
		}
		h.must(m.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("preparation consumed older comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}

func TestLPRationalPreparationLateFailuresKeepReachedPrefix(t *testing.T) {
	for _, phase := range []string{"shift", "matrix", "zero-shift", "count"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		out := h.pair("91", "0")
		h.must(m.SetLowBound(10, &h.pair("2", "0").X))
		// Inserting 20 before 10 makes libstdc++ visit column 10 first.
		h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("3/17", "0").X))
		h.must(m.SetRowColumnCoefficient(100, 10, &h.pair("7/13", "0").X))
		switch phase {
		case "shift":
			if m.LowBoundShiftForRow(100, &out.X) != ErrBounds {
				t.Fatal("missing later shift column")
			}
			lpSparseRationalCheck(t, h, &out.X, big.NewRat(14, 13))
		case "zero-shift":
			h.must(m.SetRowColumnCoefficient(100, 20, &h.pair("0", "0").X))
			if m.LowBoundShiftForRow(100, &out.X) != ErrBounds {
				t.Fatal("zero coefficient still requires its source shift lookup")
			}
			lpSparseRationalCheck(t, h, &out.X, big.NewRat(14, 13))
		case "matrix":
			h.must(m.MapExternalRowsToCoreSolverRows())
			c, _ := m.Columns.Get2(10)
			c.Index = 0
			m.CoreColumnsToExternal[0] = 10
			want := ErrBounds
			if sourceDebugEnabled {
				want = ErrAssertion
			}
			if m.FillAFromAValues() != want {
				t.Fatal("missing later matrix column")
			}
			if len(m.A.Rows[0]) != 1 || len(m.A.Columns[0]) != 1 || m.work == nil {
				t.Fatal("lost completed first row cell or second loop copy")
			}
			lpSparseRationalCheck(t, h, &m.A.Rows[0][0].Value, big.NewRat(7, 13))
			lpSparseRationalCheck(t, h, &m.work.Data[0], big.NewRat(3, 17))
		case "count":
			m.A = NewRationalStaticMatrix(h.m, 2, 0)
			m.CoreRowsToExternal[1] = 1
			h.must(m.AddConstraint(RelationEqual, &h.pair("0", "0").X, 1))
			want := ErrBounds
			if sourceDebugEnabled {
				want = ErrAssertion
			}
			if m.CountSlacksAndArtificials() != want || m.Artificials != 1 || m.Slacks != 0 {
				t.Fatal("descending count prefix")
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	// Source exact scaling has no allocation for its small one values. Neither
	// fresh vector capacity nor numerical fields incur synthetic logical fees.
	for _, direct := range []bool{false, true} {
		h := newLPPairHarness(t)
		if direct {
			h = lpNewDirectSparseHarness(t)
		}
		m := lpRationalInputModel(t, h)
		m.A = NewRationalStaticMatrix(h.m, 0, 3)
		large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
		h.must(m.ColumnScale.resizeData(4))
		for i := range m.ColumnScale.Data {
			h.must(m.ColumnScale.copySlot(&m.ColumnScale.Data[i], &large.X))
		}
		h.budget.FailAfter(0)
		err := m.Scale()
		h.budget.FailAfter(-1)
		h.must(err)
		if len(m.ColumnScale.Data) != 3 {
			t.Fatal("scale extent")
		}
		for i := range m.ColumnScale.Data {
			lpSparseRationalCheck(t, h, &m.ColumnScale.Data[i], big.NewRat(1, 1))
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
