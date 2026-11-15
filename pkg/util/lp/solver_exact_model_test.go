package lp

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func lpRationalInputModel(t *testing.T, h *lpPairHarness) *RationalLPSolver {
	t.Helper()
	m, err := NewRationalLPSolver(h.m)
	h.must(err)
	return m
}

func TestLPRationalInputModelIndependentAssembly(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		if !m.ProblemIsEmpty() || m.Status != StatusUnknown || m.TotalIterations != 0 || m.Settings == nil {
			t.Fatal("initial model")
		}
		nc, nr := 1+world%7, 1+world%5
		for j := nc - 1; j >= 0; j-- {
			ext := uint32(10 + 7*j)
			c, err := m.GetOrCreateColumnInfo(ext)
			h.must(err)
			if c.Index != InvalidColumnIndex {
				t.Fatal("input column assigned a core index")
			}
			h.must(m.GiveSymbolicNameToColumn(fmt.Sprint("c", j), ext))
			cost := big.NewRat(int64(world*13-j*17), 19)
			h.must(m.SetCostForColumn(ext, &h.pair(cost.RatString(), "0").X))
			lpSparseRationalCheck(t, h, &c.Cost, cost)
			switch j % 4 {
			case 0:
				h.must(m.SetLowBound(ext, &h.pair("-7/13", "0").X))
				lpSparseRationalCheck(t, h, &c.Low, big.NewRat(-7, 13))
			case 1:
				h.must(m.SetUpperBound(ext, &h.pair("11/17", "0").X))
				lpSparseRationalCheck(t, h, &c.Upper, big.NewRat(11, 17))
			case 2:
				h.must(m.SetFixedValue(ext, &h.pair("3/23", "0").X))
				lpSparseRationalCheck(t, h, &c.FixedValue, big.NewRat(3, 23))
			}
			if c.Name != fmt.Sprint("c", j) || lpTestNamedColumn(t, m.NamesToColumns, c.Name) != ext {
				t.Fatal("column name")
			}
			m.CoreColumnsToExternal[uint32(j)] = ext
			if m.ColumnName(uint32(j)) != c.Name {
				t.Fatal("core/external name mapping")
			}
		}
		for i := nr - 1; i >= 0; i-- {
			row := uint32(100 + 11*i)
			rhs := big.NewRat(int64(world-3*i), 29)
			h.must(m.AddConstraint(LPRelation(i%3), &h.pair(rhs.RatString(), "0").X, row))
			got, ok := m.Constraints.Get2(row)
			if !ok || got.Relation != LPRelation(i%3) {
				t.Fatal("constraint")
			}
			lpSparseRationalCheck(t, h, got.RightSide, rhs)
			for j := nc - 1; j >= 0; j-- {
				// Distinct high bits ensure that no binary64 intermediary can pass.
				n := new(big.Int).Lsh(big.NewInt(1), uint(81+world))
				n.Add(n, big.NewInt(int64(1+i*nc+j)))
				if (i+j)%2 != 0 {
					n.Neg(n)
				}
				want := new(big.Rat).SetFrac(n, big.NewInt(31))
				v := h.pair(want.RatString(), "0")
				h.must(m.SetRowColumnCoefficient(row, uint32(10+7*j), &v.X))
				// Mutating the borrowed input must not mutate the owning model.
				h.must(h.m.SetInt64(&v.X, 17))
				r, _ := m.AValues.Get2(row)
				stored, _ := r.Get2(uint32(10 + 7*j))
				lpSparseRationalCheck(t, h, stored, want)
			}
		}
		if m.ProblemIsEmpty() || m.Constraints.Len() != nr || m.Columns.Len() != nc {
			t.Fatal("model extent")
		}
		h.must(m.Close())
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalInputModelSourceReplacementAndNames(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	zero, one, two := h.pair("0", "0"), h.pair("1/3", "0"), h.pair("2/7", "0")
	m.AValues.Set(9, NewStdUnorderedRationalMap())
	if !m.ProblemIsEmpty() {
		t.Fatal("empty stored row")
	}
	h.must(m.SetRowColumnCoefficient(9, 3, &zero.X))
	if m.ProblemIsEmpty() {
		t.Fatal("explicit zero must count as a stored coefficient")
	}
	h.must(m.SetRowColumnCoefficient(9, 3, &one.X))
	r, _ := m.AValues.Get2(9)
	first, _ := r.Get2(3)
	h.must(m.SetRowColumnCoefficient(9, 3, &two.X))
	second, _ := r.Get2(3)
	if first != second || r.Len() != 1 {
		t.Fatal("coefficient assignment must replace in place")
	}
	lpSparseRationalCheck(t, h, second, big.NewRat(2, 7))
	h.must(m.GiveSymbolicNameToColumn("old", 3))
	h.must(m.GiveSymbolicNameToColumn("new", 3))
	if lpTestNamedColumn(t, m.NamesToColumns, "old") != 3 || lpTestNamedColumn(t, m.NamesToColumns, "new") != 3 || m.ColumnName(3) != "x3" {
		t.Fatal("source aliases or unmapped core name")
	}
	m.CoreColumnsToExternal[2] = 3
	m.CoreColumnsToExternal[8] = 999
	if m.ColumnName(2) != "new" || m.ColumnName(8) != "x999" {
		t.Fatal("name fallback")
	}
	h.must(m.GiveSymbolicNameToColumn("wrap", 0x80000000))
	if lpTestNamedColumn(t, m.NamesToColumns, "wrap") != 0x80000000 {
		t.Fatal("full unsigned native name index")
	}
	h.must(m.AddConstraint(RelationEqual, &one.X, 9))
	err := m.AddConstraint(RelationGreaterOrEqual, &two.X, 9)
	c, _ := m.Constraints.Get2(9)
	if sourceDebugEnabled {
		if err != ErrAssertion || c.Relation != RelationEqual || m.work == nil {
			t.Fatal("duplicate source assertion prefix", err)
		}
		lpSparseRationalCheck(t, h, c.RightSide, big.NewRat(1, 3))
	} else {
		h.must(err)
		if c.Relation != RelationGreaterOrEqual {
			t.Fatal("native replacement relation")
		}
		lpSparseRationalCheck(t, h, c.RightSide, big.NewRat(2, 7))
	}
	h.must(m.Close())
	if m.SetLowBound(1, &one.X) != ErrExactVectorPending {
		t.Fatal("closed model")
	}
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalInputModelPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		for _, op := range []string{"coefficient", "replacement", "constraint", "constraint-replacement", "cost", "low", "upper", "fixed"} {
			done, failures := false, 0
			beforeColumn, afterColumn, beforeConstraint, afterConstraint := false, false, false, false
			for ordinal := int64(0); ordinal < 350 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, op, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					m := lpRationalInputModel(t, h)
					large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
					old := h.pair("17/19", "0")
					if op == "replacement" {
						h.must(m.SetRowColumnCoefficient(3, 7, &old.X))
					}
					if op == "constraint-replacement" {
						h.must(m.AddConstraint(RelationEqual, &old.X, 3))
					}
					h.budget.FailAfter(ordinal)
					var err error
					switch op {
					case "coefficient", "replacement":
						err = m.SetRowColumnCoefficient(3, 7, &large.X)
					case "constraint", "constraint-replacement":
						err = m.AddConstraint(RelationGreaterOrEqual, &large.X, 3)
					case "cost":
						err = m.SetCostForColumn(7, &large.X)
					case "low":
						err = m.SetLowBound(7, &large.X)
					case "upper":
						err = m.SetUpperBound(7, &large.X)
					case "fixed":
						err = m.SetFixedValue(7, &large.X)
					}
					h.budget.FailAfter(-1)
					if err == nil || sourceDebugEnabled && op == "constraint-replacement" && err == ErrAssertion {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if m.SetCostForColumn(7, &old.X) != ErrExactVectorPending {
							t.Fatal("failed model accepted retry")
						}
						if op == "coefficient" {
							r, ok := m.AValues.Get2(3)
							if !ok || r.Len() != 1 {
								t.Fatal("map entry must precede coefficient copy")
							}
						}
						if op == "constraint" {
							c, exists := m.Constraints.Get2(3)
							beforeConstraint = beforeConstraint || !exists
							afterConstraint = afterConstraint || exists
							if exists && c.Relation != RelationLessOrEqual {
								t.Fatal("relation published before failed RHS assignment")
							}
						}
						if op == "cost" || op == "low" || op == "upper" || op == "fixed" {
							_, exists := m.Columns.Get2(7)
							beforeColumn = beforeColumn || !exists
							afterColumn = afterColumn || exists
						}
						if op == "constraint-replacement" {
							c, _ := m.Constraints.Get2(3)
							if c.Relation != RelationEqual {
								t.Fatal("failed replacement changed relation")
							}
						}
					}
					h.must(m.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("physical model campaign incomplete", direct, op, failures)
			}
			if direct && op == "constraint" && (!beforeConstraint || !afterConstraint) {
				t.Fatal("constraint parameter/map publication prefixes missing")
			}
			if direct && (op == "cost" || op == "low" || op == "upper" || op == "fixed") && (!beforeColumn || !afterColumn) {
				t.Fatal("column parameter/map publication prefixes missing")
			}
		}
	}
}

func TestLPRationalInputModelEntryGuards(t *testing.T) {
	if _, err := NewRationalLPSolver(nil); err != ErrExactVectorOwner {
		t.Fatal(err)
	}
	for phase := 0; phase < 10; phase++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		v := h.pair("1", "0")
		var err error
		switch phase {
		case 0:
			err = m.SetRowColumnCoefficient(1, 2, nil)
		case 1:
			err = m.AddConstraint(RelationEqual, nil, 1)
		case 2:
			err = m.SetCostForColumn(2, nil)
		case 3:
			m.AValues.Set(1, nil)
			err = m.SetRowColumnCoefficient(1, 2, &v.X)
		case 4:
			m.Columns.Set(2, nil)
			err = m.SetLowBound(2, &v.X)
		case 5:
			m.Columns.Set(2, nil)
			_, err = m.GetOrCreateColumnInfo(2)
		case 6:
			m.Columns.Set(2, nil)
			err = m.GiveSymbolicNameToColumn("bad", 2)
		case 7:
			m.stopped = true
			_, err = m.GetOrCreateColumnInfo(2)
		case 8:
			m.stopped = true
			err = m.GiveSymbolicNameToColumn("bad", 2)
		case 9:
			m.stopped = true
			err = m.AddConstraint(RelationEqual, &v.X, 1)
		}
		want := ErrBounds
		if phase >= 7 {
			want = ErrExactVectorPending
		}
		if err != want {
			t.Fatal(phase, err)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

// Literal input half of src/test/lp.cpp:1725-1763 solve_rational. The source
// find_maximal_solution/model assertions still require the unfinished outer
// exact primal solver. An independent dual bound checks this fixture's answer;
// this test does not credit an optimization performed by RationalLPSolver.
func TestLPRationalInputModelUpstreamSolveRationalInput(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	cost := []int64{-3, -1, -1, 2, -1, 1, 1, -4}
	a := [][]int64{{1, 0, 3, 1, -5, -2, 4, -6}, {0, 1, -2, -1, 4, 1, -3, 5}}
	b := []int64{7, -3}
	upper := []int64{8, 6, 4, 15, 2, 10, 10, 3}
	x := []int64{0, 6, 0, 15, 2, 1, 1, 0}
	for i, rhs := range b {
		h.must(m.AddConstraint(RelationEqual, &h.pair(fmt.Sprint(rhs), "0").X, uint32(i)))
	}
	for j, c := range cost {
		h.must(m.SetCostForColumn(uint32(j), &h.pair(fmt.Sprint(c), "0").X))
		h.must(m.GiveSymbolicNameToColumn(fmt.Sprint("x", j+1), uint32(j)))
	}
	for i, row := range a {
		for j, v := range row {
			h.must(m.SetRowColumnCoefficient(uint32(i), uint32(j), &h.pair(fmt.Sprint(v), "0").X))
		}
	}
	for j, u := range upper {
		h.must(m.SetLowBound(uint32(j), &h.pair("0", "0").X))
		h.must(m.SetUpperBound(uint32(j), &h.pair(fmt.Sprint(u), "0").X))
	}
	y := []int64{-2, -3}
	primal, dual := new(big.Rat), new(big.Rat)
	for i, row := range a {
		sum := new(big.Rat)
		stored, _ := m.AValues.Get2(uint32(i))
		for j, coefficient := range row {
			v, _ := stored.Get2(uint32(j))
			lpSparseRationalCheck(t, h, v, big.NewRat(coefficient, 1))
			sum.Add(sum, new(big.Rat).Mul(big.NewRat(coefficient, 1), big.NewRat(x[j], 1)))
		}
		constraint, _ := m.Constraints.Get2(uint32(i))
		lpSparseRationalCheck(t, h, constraint.RightSide, sum)
		if constraint.Relation != RelationEqual || sum.Cmp(big.NewRat(b[i], 1)) != 0 {
			t.Fatal("upstream expected point violates row")
		}
		dual.Add(dual, new(big.Rat).Mul(big.NewRat(y[i], 1), big.NewRat(b[i], 1)))
	}
	for j, c := range cost {
		col, _ := m.Columns.Get2(uint32(j))
		lpSparseRationalCheck(t, h, &col.Cost, big.NewRat(c, 1))
		lpSparseRationalCheck(t, h, &col.Low, new(big.Rat))
		lpSparseRationalCheck(t, h, &col.Upper, big.NewRat(upper[j], 1))
		if col.ColumnType() != ColumnBoxed || x[j] < 0 || x[j] > upper[j] || lpTestNamedColumn(t, m.NamesToColumns, fmt.Sprint("x", j+1)) != uint32(j) {
			t.Fatal("upstream expected bound/name")
		}
		primal.Add(primal, new(big.Rat).Mul(big.NewRat(c, 1), big.NewRat(x[j], 1)))
		reduced := new(big.Rat).SetInt64(c)
		for i := range a {
			reduced.Sub(reduced, new(big.Rat).Mul(big.NewRat(y[i], 1), big.NewRat(a[i][j], 1)))
		}
		if reduced.Sign() > 0 {
			dual.Add(dual, new(big.Rat).Mul(reduced, big.NewRat(upper[j], 1)))
		}
	}
	if primal.Cmp(big.NewRat(24, 1)) != 0 || dual.Cmp(primal) != 0 {
		t.Fatal("independent primal/dual bound", primal, dual)
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
