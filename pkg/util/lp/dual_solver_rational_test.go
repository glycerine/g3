package lp

// Source: src/util/lp/lp_dual_simplex.hpp. Concrete exact stage consumers.

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func lpRationalDual(t *testing.T, h *lpPairHarness) *RationalDualSimplex {
	t.Helper()
	m, e := NewRationalDualSimplex(h.m)
	h.must(e)
	m.Settings.DebugOut = nil
	m.Settings.MessageOut = nil
	m.Settings.SimplexStrategy = StrategyLU
	m.Settings.MaxTotalIterations = 200
	return m
}

// A prepared source boxed stage can be consumed directly, bypassing the
// earlier unconditional rational(double) structural-constructor boundary.
func lpRationalDualOuterBoxedStage(t *testing.T, h *lpPairHarness, m *RationalDualSimplex) {
	t.Helper()
	h.must(m.FlipCosts())
	h.must(m.FillMatrixAAndInitRightSide())
	h.must(m.FillB())
	h.must(m.Scale())
	h.must(m.AugmentMatrixAndAllocateFields())
	slack, artificial := m.numberOfCoreStructurals(), m.numberOfCoreStructurals()+m.Slacks
	for row := uint32(0); row < m.A.RowCount(); row++ {
		h.must(m.FillFirstStageRow(row, &slack, &artificial))
	}
	for j := m.A.ColumnCount(); j > m.numberOfCoreStructurals(); {
		j--
		h.must(m.FillFirstStageLogicalColumn(j))
	}
	for j := uint32(0); j < m.numberOfCoreStructurals(); j++ {
		m.CanEnterBasis[j] = false
		m.CoreColumnTypes[j] = ColumnBoxed
		h.must(m.setConstant(m.Costs, j, 0))
		h.must(m.setConstant(m.LowBounds, j, 0))
		h.must(m.setConstant(m.UpperBounds, j, 0))
	}
	h.must(m.CopyBAsideAndSetToZeros())
}
func TestLPRationalDualOuterIndependentPreparedVertices(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewSource(366352))
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		cost := []*big.Rat{big.NewRat(int64(1+rng.Intn(8)), 7), big.NewRat(int64(1+rng.Intn(8)), 11)}
		shift := []*big.Rat{big.NewRat(-2, 3), big.NewRat(-3, 5)}
		for j := 0; j < 2; j++ {
			ext := uint32(10 + j)
			h.must(m.SetLowBound(ext, lpScalarBoundValue(h, shift[j].RatString())))
			h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, new(big.Rat).Add(shift[j], big.NewRat(6, 1)).RatString())))
			h.must(m.SetCostForColumn(ext, lpScalarBoundValue(h, cost[j].RatString())))
		}
		a := make([][]*big.Rat, 5)
		b := make([]*big.Rat, 5)
		c := make([]*big.Rat, 7)
		for j := range c {
			c[j] = new(big.Rat)
			if j < 2 {
				c[j].Neg(cost[j])
			}
		}
		for i := range a {
			a[i] = make([]*big.Rat, 7)
			for j := range a[i] {
				a[i][j] = new(big.Rat)
			}
			a[i][2+i].SetInt64(1)
			if i < 3 {
				a[i][0].SetFrac64(int64(1+rng.Intn(7)), 3)
				a[i][1].SetFrac64(int64(1+rng.Intn(7)), 5)
				b[i] = big.NewRat(int64(3+rng.Intn(25)), 2)
				rhs := new(big.Rat).Set(b[i])
				for j := 0; j < 2; j++ {
					h.must(m.SetRowColumnCoefficient(uint32(100+i), uint32(10+j), lpScalarBoundValue(h, a[i][j].RatString())))
					rhs.Add(rhs, new(big.Rat).Mul(a[i][j], shift[j]))
				}
				h.must(m.AddConstraint(RelationLessOrEqual, lpScalarBoundValue(h, rhs.RatString()), uint32(100+i)))
			} else {
				a[i][i-3].SetInt64(1)
				b[i] = big.NewRat(6, 1)
			}
		}
		want := lpIndependentRationalVertexMinimum(a, b, c)
		if want == nil {
			t.Fatal("oracle", world)
		}
		want.Neg(want)
		for j := 0; j < 2; j++ {
			want.Add(want, new(big.Rat).Mul(cost[j], shift[j]))
		}
		lpRationalDualOuterBoxedStage(t, h, m)
		h.must(m.Stage1())
		if m.Status != StatusFeasible || m.FirstStageIterations != 0 || m.CoreSolver == nil {
			t.Fatal(world, m.Status, m.FirstStageIterations)
		}
		// The prepared stage-two fixture supplies extents required by core pivots;
		// the real outer constructor omits them, tested by FullEntrySourceBounds.
		m.CoreSolver.RowsNZ = make([]uint32, m.A.RowCount())
		m.CoreSolver.ColumnsNZ = make([]uint32, m.A.ColumnCount())
		e := m.Stage2()
		if sourceDebugEnabled && e == ErrAssertion {
			if m.Status != StatusFeasible || m.SecondStageIterations != 0 || m.CoreSolver.priceWork == nil || len(m.CoreSolver.priceWork.Data) != 14 || m.CoreSolver.loopValues == nil {
				t.Fatal(world, m.Status, m.SecondStageIterations)
			}
		} else {
			h.must(e)
			if m.Status != StatusOptimal || m.TotalIterations != m.FirstStageIterations+m.SecondStageIterations {
				t.Fatal(world, m.Status, m.TotalIterations)
			}
			out := lpScalarBoundValue(h, "97")
			h.must(m.GetCurrentCost(out))
			lpSparseRationalCheck(t, h, out, want)
			values := make([]*big.Rat, 2)
			for j := 0; j < 2; j++ {
				h.must(m.GetColumnValue(uint32(10+j), out))
				s, e := h.m.String(out)
				h.must(e)
				values[j] = new(big.Rat).Sub(lpRationalTestBig(s), shift[j])
				if values[j].Sign() < 0 || values[j].Cmp(big.NewRat(6, 1)) > 0 {
					t.Fatal("bound", world, j, values[j])
				}
			}
			for i := 0; i < 3; i++ {
				lhs := new(big.Rat)
				for j := 0; j < 2; j++ {
					lhs.Add(lhs, new(big.Rat).Mul(a[i][j], values[j]))
				}
				if lhs.Cmp(b[i]) > 0 {
					t.Fatal("constraint", world, i, lhs, b[i])
				}
			}
		}
		h.must(m.Close())
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func lpRationalDualOuterSimple(t *testing.T, h *lpPairHarness, boxed bool) *RationalDualSimplex {
	m := lpRationalDual(t, h)
	for _, j := range []uint32{10, 11} {
		h.must(m.SetLowBound(j, lpScalarBoundValue(h, "0")))
		if boxed {
			h.must(m.SetUpperBound(j, lpScalarBoundValue(h, "6")))
		}
		h.must(m.SetRowColumnCoefficient(100, j, lpScalarBoundValue(h, "1")))
	}
	h.must(m.SetCostForColumn(10, lpScalarBoundValue(h, "1")))
	h.must(m.SetCostForColumn(11, lpScalarBoundValue(h, "2")))
	h.must(m.AddConstraint(RelationLessOrEqual, lpScalarBoundValue(h, "3"), 100))
	return m
}
func TestLPRationalDualOuterFullEntrySourceBounds(t *testing.T) {
	for _, boxed := range []bool{false, true} {
		h := newLPPairHarness(t)
		m := lpRationalDualOuterSimple(t, h, boxed)
		e := m.FindMaximalSolution()
		if sourceDebugEnabled {
			if e != ErrAssertion || m.CoreSolver != nil || m.stageWork == nil || len(m.BCopy.Data) != 0 {
				t.Fatal(boxed, e, m.CoreSolver)
			}
			// All logical fields publish before the first structural constructor.
			if m.CoreColumnTypes[2] != ColumnBoxed || !m.CanEnterBasis[2] {
				t.Fatal(m.CoreColumnTypes, m.CanEnterBasis)
			}
		} else {
			if e != ErrBounds || m.CoreSolver == nil || len(m.CoreSolver.RowsNZ) != 0 || m.CoreSolver.pivotWork == nil {
				t.Fatal(boxed, e, m.CoreSolver)
			}
			if boxed && m.Status != StatusFeasible || !boxed && m.Status != StatusUnknown {
				t.Fatal(boxed, m.Status)
			}
		}
		if retry := m.FindMaximalSolution(); retry != ErrExactVectorPending {
			t.Fatal(retry)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	m := lpRationalDual(t, h)
	h.must(m.FindMaximalSolution())
	if m.Status != StatusEmpty {
		t.Fatal(m.Status)
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
func TestLPRationalDualOuterPreparationIndependentRows(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewSource(186336))
	for world := 0; world < 160; world++ {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		rows, cols := 1+rng.Intn(8), 3+rng.Intn(5)
		for j := 0; j < cols; j++ {
			ext := uint32(j)
			h.must(m.SetCostForColumn(ext, lpScalarBoundValue(h, fmt.Sprintf("%d/7", j-2))))
			switch j % 3 {
			case 0:
				h.must(m.SetLowBound(ext, lpScalarBoundValue(h, "0")))
			case 1:
				h.must(m.SetLowBound(ext, lpScalarBoundValue(h, "0")))
				h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, "5")))
			}
		}
		rhs := make([]*big.Rat, rows)
		slacks, artificials := uint32(0), uint32(0)
		for i := 0; i < rows; i++ {
			for j := 0; j < cols; j++ {
				v := big.NewRat(int64(1+rng.Intn(7)), 5)
				if rng.Intn(2) == 0 {
					v.Neg(v)
				}
				h.must(m.SetRowColumnCoefficient(uint32(i), uint32(j), lpScalarBoundValue(h, v.RatString())))
			}
			rhs[i] = big.NewRat(int64(rng.Intn(11)-5), 13)
			rel := LPRelation(i % 3)
			h.must(m.AddConstraint(rel, lpScalarBoundValue(h, rhs[i].RatString()), uint32(i)))
			if rel == RelationEqual {
				artificials++
			} else {
				slacks++
				if rel == RelationGreaterOrEqual && rhs[i].Sign() > 0 || rel == RelationLessOrEqual && rhs[i].Sign() < 0 {
					artificials++
				}
			}
		}
		h.must(m.FlipCosts())
		h.must(m.FillMatrixAAndInitRightSide())
		h.must(m.FillB())
		h.must(m.Scale())
		h.must(m.X.resizeData(1))
		h.must(h.m.SetInt32(&m.X.Data[0], 91))
		h.must(m.AugmentMatrixAndAllocateFields())
		if m.Slacks != slacks || m.Artificials != artificials || len(m.X.Data) != 1 || m.A.ColumnCount() != uint32(cols)+slacks+artificials {
			t.Fatal(world, "extent")
		}
		e := m.FillFirstStageSolverFields()
		if sourceDebugEnabled {
			if e != ErrAssertion {
				t.Fatal(world, e)
			}
		} else {
			h.must(e)
		}
		slack, artificial := uint32(cols), uint32(cols)+slacks
		for i := 0; i < rows; i++ {
			// Core rows follow m_A_values' libstdc++ order.
			ext := int(m.CoreRowsToExternal[uint32(i)])
			rel := LPRelation(ext % 3)
			wantBasis := slack
			want := map[uint32]int64{}
			switch rel {
			case RelationEqual:
				wantBasis = artificial
				want[artificial] = 1
				artificial++
			case RelationGreaterOrEqual:
				want[slack] = -1
				if rhs[ext].Sign() > 0 {
					wantBasis = artificial
					want[artificial] = 1
					artificial++
				}
				slack++
			case RelationLessOrEqual:
				want[slack] = 1
				if rhs[ext].Sign() < 0 {
					wantBasis = artificial
					want[artificial] = -1
					artificial++
				}
				slack++
			}
			if m.Basis[i] != wantBasis {
				t.Fatal(world, i, m.Basis[i], wantBasis)
			}
			for j, sign := range want {
				out := lpScalarBoundValue(h, "97")
				h.must(m.A.Element(uint32(i), j, out))
				lpSparseRationalCheck(t, h, out, big.NewRat(sign, 1))
			}
		}
		if !sourceDebugEnabled {
			// Core columns and rows follow m_A_values' libstdc++ order.
			for j := uint32(0); j < uint32(cols); j++ {
				ext := m.CoreColumnsToExternal[j]
				if m.CoreColumnTypes[j] != ColumnBoxed || m.CanEnterBasis[j] != (ext%3 != 1) {
					t.Fatal(world, j, m.CoreColumnTypes, m.CanEnterBasis)
				}
				upper := int64(0)
				if ext%3 == 0 {
					upper = 1
				}
				lpSparseRationalCheck(t, h, &m.UpperBounds.Data[j], big.NewRat(upper, 1))
			}
			h.must(m.CopyBAsideAndSetToZeros())
			for i := range rhs {
				lpSparseRationalCheck(t, h, &m.B.Data[i], new(big.Rat))
				lpSparseRationalCheck(t, h, &m.BCopy.Data[i], rhs[m.CoreRowsToExternal[uint32(i)]])
			}
			h.must(m.RestoreRightSides())
			for i := range rhs {
				lpSparseRationalCheck(t, h, &m.B.Data[i], rhs[m.CoreRowsToExternal[uint32(i)]])
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func lpRationalDualOuterReadyStage(t *testing.T, h *lpPairHarness) *RationalDualSimplex {
	m := lpRationalDualOuterSimple(t, h, true)
	lpRationalDualOuterBoxedStage(t, h, m)
	h.must(m.Stage1())
	m.CoreSolver.RowsNZ = make([]uint32, m.A.RowCount())
	m.CoreSolver.ColumnsNZ = make([]uint32, m.A.ColumnCount())
	return m
}
func TestLPRationalDualOuterStageStatusExactZero(t *testing.T) {
	for _, text := range []string{"0", "1/1000000000000000000000000000000000000000000000000000", "-1/1000000000000000000000000000000000000000000000000000", "9"} {
		h := newLPPairHarness(t)
		m := lpRationalDualOuterReadyStage(t, h)
		m.Settings.ToleranceForArtificials = 1e99
		h.must(h.m.SetInt32(&m.Costs.Data[0], 1))
		h.must(h.m.Copy(&m.X.Data[0], lpScalarBoundValue(h, text)))
		m.CoreSolver.Status = StatusOptimal
		h.must(m.DecideOnStatusAfterStage1())
		want := StatusUnbounded
		if text == "0" {
			want = StatusFeasible
		}
		if m.Status != want {
			t.Fatal(text, m.Status)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for _, status := range []Status{StatusDualUnbounded, StatusIterationsExhausted, StatusTimeExhausted, StatusFloatingPointError, StatusUnknown} {
		h := newLPPairHarness(t)
		m := lpRationalDualOuterReadyStage(t, h)
		m.Status = StatusEmpty
		m.CoreSolver.Status = status
		e := m.DecideOnStatusAfterStage1()
		want := status
		if status == StatusDualUnbounded {
			want = StatusIterationsExhausted
		}
		if status == StatusUnknown {
			want = StatusEmpty
		}
		if sourceDebugEnabled && (status == StatusDualUnbounded || status == StatusUnknown) {
			if e != ErrAssertion || m.Status != StatusEmpty {
				t.Fatal(status, e, m.Status)
			}
		} else {
			h.must(e)
			if m.Status != want {
				t.Fatal(status, m.Status, want)
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func TestLPRationalDualOuterStageTwoPublication(t *testing.T) {
	for _, phase := range []string{"normal", "head-cancel", "head-error", "zero-limit", "infeasible"} {
		h := newLPPairHarness(t)
		m := lpRationalDualOuterReadyStage(t, h)
		m.FirstStageIterations = 17
		m.SecondStageIterations = 91
		m.TotalIterations = 99
		limit := &lpRationalSolveCancelSequence{}
		m.Settings.SetResourceLimit(limit)
		want, eWant := StatusOptimal, error(nil)
		switch phase {
		case "head-cancel":
			limit.values = []bool{true}
			want = StatusTimeExhausted
		case "head-error":
			limit.errorAt = 1
			eWant = lpErrRationalSolveCancel
			want = StatusFeasible
		case "zero-limit":
			m.Settings.MaxTotalIterations = 0
			want = StatusFeasible // source switch default does not map exhausted
		case "infeasible":
			h.must(h.m.SetInt32(&m.BCopy.Data[0], -1))
			h.must(h.m.SetInt32(&m.Costs.Data[0], 0))
			h.must(h.m.SetInt32(&m.Costs.Data[1], 0))
			want = StatusInfeasible
		}
		if sourceDebugEnabled && (phase == "normal" || phase == "zero-limit" || phase == "infeasible") {
			eWant = ErrAssertion
			want = StatusFeasible
		}
		e := m.Stage2()
		if e != eWant || m.Status != want {
			t.Fatal(phase, e, m.Status, "want", eWant, want)
		}
		if e != nil {
			if m.SecondStageIterations != 91 || m.TotalIterations != 99 {
				t.Fatal("premature counts", phase, m.SecondStageIterations, m.TotalIterations)
			}
		} else {
			if m.TotalIterations != m.FirstStageIterations+m.SecondStageIterations || m.SecondStageIterations != m.CoreSolver.TotalIterations {
				t.Fatal(phase, m.TotalIterations)
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func TestLPRationalDualOuterFirstStructuralSourceBoundary(t *testing.T) {
	for _, typ := range []ColumnType{ColumnLowBound, ColumnBoxed, ColumnFree, ColumnUpperBound, ColumnFixed} {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		m.A = NewRationalStaticMatrix(h.m, 0, 1)
		m.CoreColumnsToExternal[0] = 10
		c, e := m.GetOrCreateColumnInfo(10)
		h.must(e)
		c.Index = 0
		switch typ {
		case ColumnLowBound:
			h.must(c.SetLowBound(lpScalarBoundValue(h, "-2")))
		case ColumnBoxed:
			h.must(c.SetLowBound(lpScalarBoundValue(h, "-2")))
			h.must(c.SetUpperBound(lpScalarBoundValue(h, "5")))
		case ColumnUpperBound:
			h.must(c.SetUpperBound(lpScalarBoundValue(h, "5")))
		case ColumnFixed:
			h.must(c.SetFixedValue(lpScalarBoundValue(h, "3")))
		}
		h.must(m.ColumnScale.resizeData(1))
		h.must(h.m.SetInt32(&m.ColumnScale.Data[0], 1))
		h.must(m.Costs.resizeData(1))
		h.must(m.LowBounds.resizeData(1))
		h.must(m.UpperBounds.resizeData(1))
		m.CanEnterBasis = []bool{false}
		m.CoreColumnTypes = []ColumnType{ColumnFree}
		h.must(h.m.SetInt32(&c.Cost, 7))
		e = m.FillFirstStageStructuralColumn(0)
		if sourceDebugEnabled {
			if e != ErrAssertion || m.CanEnterBasis[0] || m.CoreColumnTypes[0] != ColumnFree {
				t.Fatal(typ, e)
			}
		} else if typ == ColumnUpperBound {
			if e == nil || !strings.Contains(e.Error(), "unexpected bound type 0 upper_bound") {
				t.Fatal(e)
			}
		} else {
			h.must(e)
			if m.CoreColumnTypes[0] != ColumnBoxed {
				t.Fatal(typ, m.CoreColumnTypes)
			}
			can := typ == ColumnLowBound || typ == ColumnFree
			if m.CanEnterBasis[0] != can {
				t.Fatal(typ, m.CanEnterBasis)
			}
			upper := int64(0)
			if typ == ColumnLowBound {
				upper = 1
			}
			lpSparseRationalCheck(t, h, &m.UpperBounds.Data[0], big.NewRat(upper, 1))
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func TestLPRationalDualOuterFixTypesAndBoxWidth(t *testing.T) {
	for _, typ := range []ColumnType{ColumnLowBound, ColumnBoxed, ColumnFree, ColumnUpperBound, ColumnFixed} {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		m.A = NewRationalStaticMatrix(h.m, 0, 1)
		m.CoreColumnsToExternal[0] = 10
		c, e := m.GetOrCreateColumnInfo(10)
		h.must(e)
		c.Index = 0
		h.must(c.SetCost(lpScalarBoundValue(h, "-7/11")))
		switch typ {
		case ColumnLowBound:
			h.must(c.SetLowBound(lpScalarBoundValue(h, "-2")))
		case ColumnBoxed, ColumnFixed:
			h.must(c.SetLowBound(lpScalarBoundValue(h, "-2")))
			h.must(c.SetUpperBound(lpScalarBoundValue(h, "5")))
			if typ == ColumnFixed {
				h.must(c.SetFixedValue(lpScalarBoundValue(h, "3")))
			}
		case ColumnUpperBound:
			h.must(c.SetUpperBound(lpScalarBoundValue(h, "5")))
		}
		h.must(m.ColumnScale.resizeData(1))
		h.must(h.m.SetString(&m.ColumnScale.Data[0], "3/7"))
		h.must(m.Costs.resizeData(1))
		h.must(m.LowBounds.resizeData(1))
		h.must(m.UpperBounds.resizeData(1))
		m.CanEnterBasis = []bool{false}
		m.CoreColumnTypes = []ColumnType{ColumnFree}
		e = m.FixStructuralForStage2(0)
		if sourceDebugEnabled && (typ == ColumnFixed || typ == ColumnUpperBound) {
			if e != ErrAssertion || m.CanEnterBasis[0] {
				t.Fatal(typ, e)
			}
		} else {
			h.must(e)
			if !m.CanEnterBasis[0] {
				t.Fatal(typ)
			}
			want := typ
			if typ == ColumnFixed || typ == ColumnUpperBound {
				want = ColumnBoxed
			}
			if m.CoreColumnTypes[0] != want {
				t.Fatal(typ, m.CoreColumnTypes)
			}
			if want == ColumnBoxed {
				width := big.NewRat(49, 3)
				if typ == ColumnUpperBound {
					width = big.NewRat(35, 3)
				}
				lpSparseRationalCheck(t, h, &m.UpperBounds.Data[0], width)
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func lpRationalDualOuterLarge(t *testing.T, h *lpPairHarness, op string) *RationalDualSimplex {
	m := lpRationalDualOuterSimple(t, h, true)
	huge := strings.Repeat("1234567890", 10)
	h.must(m.SetUpperBound(10, lpScalarBoundValue(h, huge+"9/23")))
	h.must(m.SetCostForColumn(10, lpScalarBoundValue(h, huge+"7/17")))
	h.must(m.SetCostForColumn(11, lpScalarBoundValue(h, huge+"3/19")))
	if op == "find" {
		return m
	}
	lpRationalDualOuterBoxedStage(t, h, m)
	if op == "stage1" {
		// Positive cost selects the low bound and keeps exact stage-one cost zero.
		h.must(h.m.SetString(&m.Costs.Data[0], huge+"7/17"))
		m.CanEnterBasis[0] = true
		h.must(h.m.SetInt32(&m.UpperBounds.Data[0], 1))
		return m
	}
	if op == "copy" || op == "restore" {
		rhs := huge + "5/29"
		if op == "restore" {
			rhs = strings.Repeat("1234567890", 100) + "5/29"
		}
		h.must(h.m.SetString(&m.B.Data[0], rhs))
		if op == "restore" {
			h.must(m.BCopy.resizeData(0)) // this campaign starts its own first RHS backup
			h.must(m.CopyBAsideAndSetToZeros())
			h.must(m.B.resizeData(0))
			h.must(m.B.resizeData(1)) // cold zero destination for physical copy refusal
		}
		return m
	}
	if op == "fix" {
		return m
	}
	h.must(m.Stage1())
	m.CoreSolver.RowsNZ = make([]uint32, m.A.RowCount())
	m.CoreSolver.ColumnsNZ = make([]uint32, m.A.ColumnCount())
	if op == "cost" || op == "value" {
		h.must(m.UnmarkColumnsAndFixStructuralCosts())
		h.must(h.m.SetString(&m.X.Data[0], huge+"5/29"))
	}
	return m
}
func lpRationalDualOuterOperation(m *RationalDualSimplex, op string, out *numeral.Rational) error {
	switch op {
	case "find":
		return m.FindMaximalSolution()
	case "copy":
		return m.CopyBAsideAndSetToZeros()
	case "restore":
		return m.RestoreRightSides()
	case "fix":
		return m.FixStructuralForStage2(0)
	case "stage1":
		return m.Stage1()
	case "stage2":
		return m.Stage2()
	case "cost":
		return m.GetCurrentCost(out)
	case "value":
		// Core column 0 holds the huge X; its external index follows the
		// libstdc++ m_A_values order.
		return m.GetColumnValue(m.CoreColumnsToExternal[0], out)
	}
	panic(op)
}
func TestLPRationalDualOuterPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, op := range []string{"find", "copy", "restore", "fix", "stage1", "stage2", "cost", "value"} {
		for _, direct := range []bool{false, true} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 6000; ordinal++ {
				done := false
				t.Run(fmt.Sprintf("%s/%t/%d", op, direct, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					m := lpRationalDualOuterLarge(t, h, op)
					out := lpScalarBoundValue(h, "97")
					h.budget.FailAfter(ordinal)
					e := lpRationalDualOuterOperation(m, op, out)
					h.budget.FailAfter(-1)
					terminal := e == nil || sourceDebugEnabled && (op == "find" || op == "stage2") && e == ErrAssertion || !sourceDebugEnabled && op == "find" && e == ErrBounds && m.CoreSolver != nil && m.CoreSolver.pivotWork != nil
					if terminal {
						success, done = true, true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(e) {
							t.Fatal(e)
						}
						if retry := lpRationalDualOuterOperation(m, op, out); retry != ErrExactVectorPending {
							t.Fatal("retry", retry)
						}
					}
					h.must(m.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
				if done {
					break
				}
			}
			if !success || direct && failures == 0 {
				t.Fatal(op, direct, success, failures)
			}
		}
	}
}
func TestLPRationalDualOuterPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, op := range []string{"find", "copy", "restore", "fix", "stage1", "stage2", "cost", "value"} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			m := lpRationalDualOuterLarge(t, h, op)
			out := lpScalarBoundValue(h, "97")
			v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			var caller exactEtaCalls
			before, e := caller.begin(h.m)
			h.must(e)
			h.budget.FailAfter(ordinal)
			_, e = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			e = caller.after(h.m, before, e)
			older := h.m.ComparisonInvocationState()
			if older.Active {
				matched = true
				if !lpExactPriorityPhysicalRefusal(e) {
					t.Fatal(e)
				}
				_ = lpRationalDualOuterOperation(m, op, out)
				if m.outerCalls.comparison != 0 || m.RationalLPSolver.calls.comparison != 0 {
					t.Fatal("adopted caller comparison")
				}
			} else if e != nil && !lpExactPriorityPhysicalRefusal(e) {
				t.Fatal(e)
			}
			h.must(m.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no older comparison", op)
		}
	}
}

func TestLPRationalDualOuterPreparedInfeasible(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalDualOuterReadyStage(t, h)
	for j := 0; j < 2; j++ {
		m.CoreColumnTypes[j] = ColumnFixed
		m.CanEnterBasis[j] = false
	}
	h.must(h.m.SetInt32(&m.B.Data[0], -1))
	m.FirstStageIterations = 17
	h.must(m.SolveForStage2())
	if m.Status != StatusInfeasible || m.SecondStageIterations != 2 || m.TotalIterations != 19 {
		t.Fatal(m.Status, m.SecondStageIterations, m.TotalIterations)
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
func TestLPRationalDualOuterMetadataPublication(t *testing.T) {
	for _, phase := range []string{"missing-map", "missing-column", "type-range", "logical-range", "logical-upper", "row-map", "row-rhs", "row-basis", "row-cost", "row-logical", "copy-prefix", "restore-prefix", "fix-scale", "fix-upper", "stage1-repeat", "stage2-core", "cost-nil"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			m := lpRationalDualOuterSimple(t, h, true)
			h.must(m.FlipCosts())
			h.must(m.FillMatrixAAndInitRightSide())
			h.must(m.FillB())
			h.must(m.Scale())
			h.must(m.AugmentMatrixAndAllocateFields())
			b, up, scale, costs, logical, basis := m.B.Data, m.UpperBounds.Data, m.ColumnScale.Data, m.Costs.Data, m.LogicalColumnTypes, m.Basis
			// The external column of core column 1 (libstdc++ m_A_values order).
			removedExt := m.CoreColumnsToExternal[1]
			removedColumn, _ := m.Columns.Get2(removedExt)
			slack, artificial := uint32(2), uint32(2)+m.Slacks
			want := error(ErrBounds)
			var e error
			switch phase {
			case "missing-map":
				delete(m.CoreColumnsToExternal, 1)
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillFirstStageStructuralColumn(1)
			case "missing-column":
				m.Columns.Del(removedExt)
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillFirstStageStructuralColumn(1)
			case "type-range":
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				_, e = m.GetColumnType(99)
			case "logical-range":
				e = m.SetTypeForLogical(0, ColumnFixed)
			case "logical-upper":
				m.LogicalColumnTypes[0] = ColumnUpperBound
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillFirstStageLogicalColumn(2)
			case "row-map":
				delete(m.CoreRowsToExternal, 0)
				e = m.FillFirstStageRow(0, &slack, &artificial)
				if m.Constraints.Get(0) == nil || m.stageWork == nil {
					t.Fatal("default constraint and copied RHS prefix")
				}
			case "row-rhs":
				m.B.Data = nil
				e = m.FillFirstStageRow(0, &slack, &artificial)
			case "row-basis":
				m.Basis = nil
				e = m.FillFirstStageRow(0, &slack, &artificial)
			case "row-cost":
				m.Costs.Data = nil
				e = m.FillFirstStageRow(0, &slack, &artificial)
			case "row-logical":
				m.LogicalColumnTypes = nil
				e = m.FillFirstStageRow(0, &slack, &artificial)
			case "copy-prefix":
				h.must(m.B.resizeData(2))
				b = m.B.Data
				h.must(h.m.SetInt32(&m.B.Data[1], 7))
				h.must(m.CopyBAsideAndSetToZeros())
				h.must(h.m.SetInt32(&m.B.Data[0], 9))
				h.must(m.CopyBAsideAndSetToZeros())
				want = nil
				if len(m.BCopy.Data) != 4 {
					t.Fatal(len(m.BCopy.Data))
				}
				lpSparseRationalCheck(t, h, &m.BCopy.Data[0], big.NewRat(3, 1))
				lpSparseRationalCheck(t, h, &m.BCopy.Data[2], big.NewRat(9, 1))
			case "restore-prefix":
				e = m.RestoreRightSides()
			case "fix-scale":
				m.ColumnScale.Data = nil
				e = m.FixStructuralForStage2(0)
			case "fix-upper":
				m.UpperBounds.Data = nil
				e = m.FixStructuralForStage2(0)
			case "stage1-repeat":
				h.must(m.Close())
				m = lpRationalDualOuterReadyStage(t, h)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.Stage1()
			case "stage2-core":
				e = m.SolveForStage2()
			case "cost-nil":
				e = m.GetCurrentCost(nil)
			}
			if e != want {
				t.Fatal(e, want)
			}
			if e != nil {
				if retry := m.FindMaximalSolution(); retry != ErrExactVectorPending {
					t.Fatal("retry", retry)
				}
			}
			if phase != "stage1-repeat" {
				if phase == "missing-column" {
					m.Columns.Set(removedExt, removedColumn)
				}
				m.B.Data = b
				m.UpperBounds.Data = up
				m.ColumnScale.Data = scale
				m.Costs.Data = costs
				m.LogicalColumnTypes = logical
				m.Basis = basis
				// The append-only BCopy owns its new slots; never shorten it for cleanup.
			}
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLPRationalDualOuterStageReporterAndFlush(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	const want = "extended A[1,3]\n"
	for cut := 0; cut <= len(want); cut++ {
		h := newLPPairHarness(t)
		m := lpRationalDualOuterLarge(t, h, "stage1")
		w := &lpResidualFailWriter{remaining: cut}
		m.Settings.MessageOut = lpResidualWriterOnly{writer: w}
		e := m.Stage1()
		if w.String() != want[:cut] {
			t.Fatal(cut, w.String())
		}
		if cut < len(want) {
			if e != lpErrResidualWrite || m.CoreSolver != nil || len(m.X.Data) != 3 {
				t.Fatal(cut, e)
			}
		} else {
			h.must(e)
			if m.Status != StatusFeasible {
				t.Fatal(m.Status)
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	m := lpRationalDualOuterLarge(t, h, "stage1")
	w := &lpRationalDualDiagnosticWriter{flushErr: lpErrResidualWrite}
	m.Settings.MessageOut = w
	e := m.Stage1()
	if e != lpErrResidualWrite || w.flushes != 1 || m.CoreSolver != nil {
		t.Fatal(e, w.flushes)
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
func TestLPRationalDualOuterCostAliasesAndLifetime(t *testing.T) {
	for _, alias := range []bool{false, true} {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		want := new(big.Rat)
		for j := uint32(0); j < 5; j++ {
			c := big.NewRat(-int64(j+3), 7)
			v := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(int64(j+1)), uint(85+j)), big.NewInt(19))
			h.must(m.SetCostForColumn(j, lpScalarBoundValue(h, c.RatString())))
			h.must(m.SetFixedValue(j, lpScalarBoundValue(h, v.RatString())))
			want.Sub(want, new(big.Rat).Mul(c, v))
		}
		out := lpScalarBoundValue(h, "97")
		if alias {
			c, _ := m.Columns.Get2(4)
			out = &c.Cost
		}
		h.must(m.GetCurrentCost(out))
		lpSparseRationalCheck(t, h, out, want)
		h.must(m.Close())
		h.must(m.Close())
		if e := m.GetColumnValue(0, out); e != ErrExactVectorPending {
			t.Fatal(e)
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
	if _, e := NewRationalDualSimplex(nil); e != ErrExactVectorOwner {
		t.Fatal(e)
	}
}

func TestLPRationalDualOuterCopyCompletedPrefix(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		success, witness := false, false
		for ordinal := int64(0); ordinal < 1000; ordinal++ {
			var h *lpPairHarness
			if direct {
				h = lpNewDirectSparseHarness(t)
			} else {
				h = newLPPairHarness(t)
			}
			m := lpRationalDual(t, h)
			h.must(m.B.resizeData(2))
			h.must(h.m.SetInt32(&m.B.Data[0], 7))
			h.must(h.m.SetString(&m.B.Data[1], strings.Repeat("1234567890", 100)+"5/29"))
			h.budget.FailAfter(ordinal)
			e := m.CopyBAsideAndSetToZeros()
			h.budget.FailAfter(-1)
			if e == nil {
				success = true
				if len(m.BCopy.Data) != 2 {
					t.Fatal(len(m.BCopy.Data))
				}
			} else {
				if !lpExactPriorityPhysicalRefusal(e) {
					t.Fatal(e)
				}
				if len(m.BCopy.Data) == 1 {
					witness = true
					lpSparseRationalCheck(t, h, &m.BCopy.Data[0], big.NewRat(7, 1))
					lpSparseRationalCheck(t, h, &m.B.Data[0], new(big.Rat))
					if m.stageWork == nil {
						t.Fatal("lost incomplete append owner")
					}
				}
			}
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
			if success {
				break
			}
		}
		if !success || !witness {
			t.Fatal(direct, success, witness)
		}
	}
}

func TestLPRationalDualOuterLogicalStageTransitions(t *testing.T) {
	for _, typ := range []ColumnType{ColumnLowBound, ColumnFixed, ColumnUpperBound, ColumnFree, ColumnType(99)} {
		h := newLPPairHarness(t)
		m := lpRationalDual(t, h)
		m.A = NewRationalStaticMatrix(h.m, 0, 1)
		m.LogicalColumnTypes = []ColumnType{typ}
		m.CoreColumnTypes = []ColumnType{ColumnBoxed}
		m.CanEnterBasis = []bool{true}
		h.must(m.LowBounds.resizeData(1))
		h.must(m.UpperBounds.resizeData(1))
		h.must(h.m.SetInt32(&m.LowBounds.Data[0], 7))
		h.must(h.m.SetInt32(&m.UpperBounds.Data[0], 9))
		got, e := m.GetColumnType(0)
		h.must(e)
		if got != typ {
			t.Fatal(got, typ)
		}
		e = m.FixLogicalForStage2(0)
		valid := typ == ColumnLowBound || typ == ColumnFixed
		if !valid && sourceDebugEnabled {
			if e != ErrAssertion {
				t.Fatal(typ, e)
			}
		} else {
			h.must(e)
		}
		if valid {
			lpSparseRationalCheck(t, h, &m.LowBounds.Data[0], new(big.Rat))
			upper := int64(9)
			if typ == ColumnFixed {
				upper = 0
			}
			lpSparseRationalCheck(t, h, &m.UpperBounds.Data[0], big.NewRat(upper, 1))
			if m.CoreColumnTypes[0] != typ || m.CanEnterBasis[0] != (typ == ColumnLowBound) {
				t.Fatal(typ, m.CoreColumnTypes, m.CanEnterBasis)
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func TestLPRationalDualOuterPresolveRejectsBeforeStages(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalDualOuterSimple(t, h, false)
	c, _ := m.Constraints.Get2(100)
	h.must(h.m.SetInt32(c.RightSide, -1))
	h.must(m.FindMaximalSolution())
	if m.Status != StatusInfeasible || m.CoreSolver != nil || len(m.BCopy.Data) != 0 {
		t.Fatal(m.Status, m.CoreSolver)
	}
	// Costs flip before presolve, even though no stage is constructed.
	col, _ := m.Columns.Get2(10)
	lpSparseRationalCheck(t, h, &col.Cost, big.NewRat(-1, 1))
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
