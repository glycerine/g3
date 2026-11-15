package lp

// Source: src/util/lp/lp_primal_simplex.hpp; independent exact outer LP models.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/numeral"
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

func lpRationalPrimal(t *testing.T, h *lpPairHarness) *RationalPrimalSimplex {
	t.Helper()
	m, e := NewRationalPrimalSimplex(h.m)
	h.must(e)
	m.Settings.DebugOut = nil
	m.Settings.MessageOut = nil
	m.Settings.MaxTotalIterations = 1000
	return m
}
func TestLPRationalPrimalOuterIndependentVertices(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewSource(169271))
	sourceFlipDiscrepancies := 0
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		m := lpRationalPrimal(t, h)
		m.Settings.SimplexStrategy = StrategyLU
		m.Settings.UseScaling = world%2 == 0
		shift := []*big.Rat{big.NewRat(-2, 3), big.NewRat(-3, 5)}
		sign := []int64{1, 1}
		if world%4 == 0 {
			sign[1] = -1
		}
		cost := []*big.Rat{big.NewRat(int64(1+rng.Intn(9)), 7), big.NewRat(int64(1+rng.Intn(9)), 11)}
		boxed := []bool{world%3 == 0 || sign[1] < 0, world%5 == 0 && sign[1] > 0}
		for j := 0; j < 2; j++ {
			ext := uint32(10 + j)
			if sign[j] > 0 {
				h.must(m.SetLowBound(ext, lpScalarBoundValue(h, shift[j].RatString())))
			} else {
				h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, shift[j].RatString())))
			}
			if boxed[j] {
				h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, new(big.Rat).Add(shift[j], big.NewRat(6, 1)).RatString())))
			}
			h.must(m.SetCostForColumn(ext, lpScalarBoundValue(h, new(big.Rat).Mul(cost[j], big.NewRat(sign[j], 1)).RatString())))
		}
		h.must(m.SetLowBound(99, lpScalarBoundValue(h, "5")))
		h.must(m.SetUpperBound(99, lpScalarBoundValue(h, "5")))
		h.must(m.SetFixedValue(99, lpScalarBoundValue(h, "5")))
		h.must(m.SetCostForColumn(99, lpScalarBoundValue(h, "2")))
		var inequalities [][]*big.Rat
		var rhs []*big.Rat
		originalA := make([][]*big.Rat, 3)
		originalB := make([]*big.Rat, 3)
		for i := 0; i < 3; i++ {
			a, b := big.NewRat(int64(1+rng.Intn(7)), 3), big.NewRat(int64(1+rng.Intn(7)), 5)
			bound := big.NewRat(int64(3+rng.Intn(25)), 2)
			inequalities = append(inequalities, []*big.Rat{a, b})
			rhs = append(rhs, bound)
			originalA[i] = []*big.Rat{new(big.Rat).Mul(a, big.NewRat(sign[0], 1)), new(big.Rat).Mul(b, big.NewRat(sign[1], 1)), big.NewRat(int64(i+1), 1)}
			originalB[i] = new(big.Rat).Set(bound)
			for j, ext := range []uint32{10, 11, 99} {
				h.must(m.SetRowColumnCoefficient(uint32(100+i), ext, lpScalarBoundValue(h, originalA[i][j].RatString())))
				x := big.NewRat(5, 1)
				if j < 2 {
					x = shift[j]
				}
				originalB[i].Add(originalB[i], new(big.Rat).Mul(originalA[i][j], x))
			}
			h.must(m.AddConstraint(RelationLessOrEqual, lpScalarBoundValue(h, originalB[i].RatString()), uint32(100+i)))
		}
		for j := 0; j < 2; j++ {
			if boxed[j] {
				v := []*big.Rat{new(big.Rat), new(big.Rat)}
				v[j].SetInt64(1)
				inequalities = append(inequalities, v)
				rhs = append(rhs, big.NewRat(6, 1))
			}
		}
		cols := 2 + len(rhs)
		a := make([][]*big.Rat, len(rhs))
		c := make([]*big.Rat, cols)
		for j := range c {
			c[j] = new(big.Rat)
			if j < 2 {
				c[j].Neg(cost[j])
			}
		}
		for i, v := range inequalities {
			a[i] = make([]*big.Rat, cols)
			for j := range a[i] {
				a[i][j] = new(big.Rat)
			}
			a[i][0].Set(v[0])
			a[i][1].Set(v[1])
			a[i][2+i].SetInt64(1)
		}
		want := lpIndependentRationalVertexMinimum(a, rhs, c)
		if want == nil {
			t.Fatal("oracle no vertex", world)
		}
		// The retained source flips upper-only coefficients and costs but leaves
		// the core type upper_bound. Its transformed variable is <= 0, not
		// the intended >= 0. Enumerate that actual source domain separately;
		// keep every flipped model and its original mathematical oracle.
		mathematical := new(big.Rat).Neg(new(big.Rat).Set(want))
		if sign[1] < 0 {
			sourceA := make([][]*big.Rat, len(a))
			for i := range a {
				sourceA[i] = make([]*big.Rat, len(a[i]))
				for j := range a[i] {
					sourceA[i][j] = new(big.Rat).Set(a[i][j])
				}
				sourceA[i][1].Neg(sourceA[i][1])
			}
			sourceC := make([]*big.Rat, len(c))
			for j := range c {
				sourceC[j] = new(big.Rat).Set(c[j])
			}
			sourceC[1].Neg(sourceC[1])
			want = lpIndependentRationalVertexMinimum(sourceA, rhs, sourceC)
			if want == nil {
				t.Fatal("source domain oracle", world)
			}
		}
		want.Neg(want)
		want.Add(want, big.NewRat(10, 1))
		mathematical.Add(mathematical, big.NewRat(10, 1))
		for j := 0; j < 2; j++ {
			term := new(big.Rat).Mul(new(big.Rat).Mul(cost[j], big.NewRat(sign[j], 1)), shift[j])
			want.Add(want, term)
			mathematical.Add(mathematical, term)
		}
		if sign[1] < 0 && want.Cmp(mathematical) != 0 {
			sourceFlipDiscrepancies++
		}
		h.must(m.FindMaximalSolution())
		out := lpScalarBoundValue(h, "-97")
		h.must(m.GetCurrentCost(out))
		if m.Status != StatusOptimal || m.CoreSolver == nil || m.TotalIterations != m.CoreSolver.TotalIterations {
			t.Fatal(world, m.Status, m.TotalIterations)
		}
		text, e := h.m.String(out)
		h.must(e)
		if lpRationalTestBig(text).Cmp(want) != 0 {
			xs := []string{}
			for i := range m.X.Data {
				v, e := h.m.String(&m.X.Data[i])
				h.must(e)
				xs = append(xs, v)
			}
			t.Fatal("optimum", world, m.Settings.SimplexStrategy, text, want, "x", xs, "basis", m.Basis, "types", m.ColumnTypes, "input", originalA, originalB, "cost", cost, "sign", sign)
		}
		values := make([]*big.Rat, 3)
		for j, ext := range []uint32{10, 11, 99} {
			h.must(m.GetColumnValue(ext, out))
			text, e := h.m.String(out)
			h.must(e)
			values[j] = lpRationalTestBig(text)
		}
		for i, row := range originalA {
			lhs := new(big.Rat)
			for j, v := range row {
				lhs.Add(lhs, new(big.Rat).Mul(v, values[j]))
			}
			if lhs.Cmp(originalB[i]) > 0 {
				t.Fatal("external constraint", world, i, lhs, originalB[i])
			}
		}
		for j := 0; j < 2; j++ {
			y := new(big.Rat).Mul(new(big.Rat).Sub(values[j], shift[j]), big.NewRat(sign[j], 1))
			invalid := y.Sign() < 0
			if sign[j] < 0 {
				invalid = y.Sign() > 0
			} // retained source transformed upper bound
			if invalid || boxed[j] && y.Cmp(big.NewRat(6, 1)) > 0 {
				t.Fatal("external bound", world, j, y)
			}
		}
		lu := m.CoreSolver.Factorization
		h.must(m.Close())
		h.must(m.Close())
		if lu != nil && !lu.closed {
			t.Fatal("core borrower survived")
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
	if sourceFlipDiscrepancies == 0 {
		t.Fatal("source upper-only inconsistency was not exercised")
	}
}
func TestLPRationalPrimalOuterEmptyAndTerminal(t *testing.T) {
	for _, phase := range []string{"empty", "infeasible", "unbounded", "cancel", "zero-limit"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			m := lpRationalPrimal(t, h)
			m.Settings.SimplexStrategy = StrategyLU
			want := StatusEmpty
			if phase != "empty" {
				h.must(m.SetLowBound(10, lpScalarBoundValue(h, "0")))
				h.must(m.SetLowBound(11, lpScalarBoundValue(h, "0")))
				h.must(m.SetCostForColumn(10, lpScalarBoundValue(h, "1")))
				h.must(m.SetRowColumnCoefficient(100, 10, lpScalarBoundValue(h, "1")))
				h.must(m.SetRowColumnCoefficient(100, 11, lpScalarBoundValue(h, "1")))
				relation, b := RelationLessOrEqual, "-1"
				want = StatusInfeasible
				switch phase {
				case "unbounded":
					relation, b, want = RelationGreaterOrEqual, "1", StatusUnbounded
				case "cancel":
					b, want = "3", StatusTimeExhausted
					m.Settings.SetResourceLimit(&lpTestResourceLimit{cancel: true})
				case "zero-limit":
					b, want = "3", StatusFeasible
					m.Settings.MaxTotalIterations = 0
				}
				h.must(m.AddConstraint(relation, lpScalarBoundValue(h, b), 100))
			}
			h.must(m.FindMaximalSolution())
			if m.Status != want {
				t.Fatal(phase, m.Status, want)
			}
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLPRationalPrimalStageIndependentRows(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	rng := rand.New(rand.NewSource(891774))
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		m := lpRationalPrimal(t, h)
		rows, cols := 1+rng.Intn(9), 1+rng.Intn(6)
		for j := 0; j < cols; j++ {
			ext := uint32(j)
			h.must(m.SetCostForColumn(ext, lpScalarBoundValue(h, fmt.Sprintf("%d/4", j-2))))
			switch j % 4 {
			case 0:
				h.must(m.SetLowBound(ext, lpScalarBoundValue(h, "-2")))
			case 1:
				h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, "3")))
			case 2:
				h.must(m.SetLowBound(ext, lpScalarBoundValue(h, "-2")))
				h.must(m.SetUpperBound(ext, lpScalarBoundValue(h, "3")))
			}
		}
		expected := make([]*big.Rat, rows)
		for i := 0; i < rows; i++ {
			shift := new(big.Rat)
			for j := 0; j < cols; j++ {
				v := big.NewRat(int64(1+rng.Intn(8)), 7)
				if rng.Intn(2) == 0 {
					v.Neg(v)
				}
				h.must(m.SetRowColumnCoefficient(uint32(i), uint32(j), lpScalarBoundValue(h, v.RatString())))
				s := int64(0)
				switch j % 4 {
				case 0, 2:
					s = -2
				case 1:
					s = 3
				}
				shift.Add(shift, new(big.Rat).Mul(v, big.NewRat(s, 1)))
			}
			expected[i] = big.NewRat(int64(rng.Intn(11)-5), 13)
			rhs := new(big.Rat).Add(shift, expected[i])
			h.must(m.AddConstraint(LPRelation(i%3), lpScalarBoundValue(h, rhs.RatString()), uint32(i)))
		}
		h.must(m.FillMatrixAAndInitRightSide())
		h.must(m.FillB())
		h.must(m.Scale())
		h.must(m.X.resizeData(uint32(cols)))
		h.must(m.FillAcceptableValuesForX())
		h.must(m.CountSlacksAndArtificials())
		h.must(m.SetCoreSolverBounds())
		h.must(m.PrepareStageOneTotalInf())
		if m.A.ColumnCount() != uint32(cols+rows) || len(m.X.Data) != cols+rows || len(m.LowBounds.Data) != cols+rows {
			t.Fatal("dimensions", world)
		}
		for i := 0; i < rows; i++ {
			j := uint32(cols + i)
			// Core row i is external row ext (libstdc++ m_A_values order).
			ext := int(m.CoreRowsToExternal[uint32(i)])
			sign := int64(1)
			if LPRelation(ext%3) == RelationGreaterOrEqual {
				sign = -1
			}
			if m.Basis[i] != j {
				t.Fatal("basis", world, i)
			}
			want := new(big.Rat).Mul(expected[ext], big.NewRat(sign, 1))
			lpSparseRationalCheck(t, h, &m.X.Data[j], want)
			v, e := m.A.ColumnValue(m.A.Columns[j][0])
			h.must(e)
			lpSparseRationalCheck(t, h, v, big.NewRat(sign, 1))
			typ := ColumnLowBound
			if LPRelation(ext%3) == RelationEqual {
				typ = ColumnFixed
			}
			if m.ColumnTypes[j] != typ {
				t.Fatal("type", i)
			}
			lpSparseRationalCheck(t, h, &m.LowBounds.Data[j], new(big.Rat))
			lpSparseRationalCheck(t, h, &m.UpperBounds.Data[j], new(big.Rat))
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalPrimalOuterFreshTableauSourcePrefix(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyTableauRows, StrategyTableauCosts} {
		h := newLPPairHarness(t)
		m := lpRationalPrimal(t, h)
		m.Settings.SimplexStrategy = strategy
		for _, j := range []uint32{10, 11} {
			h.must(m.SetLowBound(j, lpScalarBoundValue(h, "0")))
			h.must(m.SetRowColumnCoefficient(100, j, lpScalarBoundValue(h, "1")))
		}
		h.must(m.SetCostForColumn(10, lpScalarBoundValue(h, "1")))
		h.must(m.AddConstraint(RelationLessOrEqual, lpScalarBoundValue(h, "3"), 100))
		e := m.FindMaximalSolution()
		if sourceDebugEnabled && strategy == StrategyTableauCosts {
			if e != ErrAssertion || m.CoreSolver == nil || m.CoreSolver.TotalIterations != 0 || m.Status != StatusUnknown {
				t.Fatal(strategy, e, m.Status)
			}
		} else {
			h.must(e)
			if m.Status != StatusOptimal || m.TotalIterations != 1 {
				t.Fatal(strategy, m.Status, m.TotalIterations)
			}
			out := lpScalarBoundValue(h, "97")
			h.must(m.GetCurrentCost(out))
			lpSparseRationalCheck(t, h, out, new(big.Rat))
		}
		// Both modes retain the fresh all-zero reduced-cost vector; source does
		// not initialize it in init_run_tableau. The mathematical optimum is 3.
		lpSparseRationalCheck(t, h, &m.X.Data[0], new(big.Rat))
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func lpRationalPrimalOuterLarge(t *testing.T, h *lpPairHarness) *RationalPrimalSimplex {
	m := lpRationalPrimal(t, h)
	m.Settings.SimplexStrategy = StrategyLU
	huge := strings.Repeat("1234567890", 10)
	for _, j := range []uint32{10, 11} {
		h.must(m.SetLowBound(j, lpScalarBoundValue(h, "0")))
		h.must(m.SetRowColumnCoefficient(100, j, lpScalarBoundValue(h, "1")))
	}
	h.must(m.SetCostForColumn(10, lpScalarBoundValue(h, huge+"7/17")))
	h.must(m.SetCostForColumn(11, lpScalarBoundValue(h, huge+"3/19")))
	h.must(m.SetUpperBound(10, lpScalarBoundValue(h, huge+"9/23")))
	h.must(m.AddConstraint(RelationLessOrEqual, lpScalarBoundValue(h, huge+"5/29"), 100))
	return m
}
func lpRationalPrimalOuterPrepare(t *testing.T, h *lpPairHarness, m *RationalPrimalSimplex) {
	t.Helper()
	h.must(m.FillMatrixAAndInitRightSide())
	h.must(m.FillB())
	h.must(m.Scale())
	h.must(m.X.resizeData(m.A.ColumnCount()))
	h.must(m.FillAcceptableValuesForX())
	h.must(m.CountSlacksAndArtificials())
	h.must(m.SetCoreSolverBounds())
}
func lpRationalPrimalOuterOperationFixture(t *testing.T, h *lpPairHarness, op string) *RationalPrimalSimplex {
	m := lpRationalPrimalOuterLarge(t, h)
	switch op {
	case "prepare", "row", "solve":
		lpRationalPrimalOuterPrepare(t, h, m)
	case "bounds":
		h.must(m.FillMatrixAAndInitRightSide())
		h.must(m.FillB())
		h.must(m.Scale())
		h.must(m.CountSlacksAndArtificials())
	case "cost", "value":
		h.must(m.FindMaximalSolution())
	}
	if op == "row" {
		total := m.A.ColumnCount() + m.A.RowCount()
		h.must(m.X.resizeData(total))
		h.must(m.LowBounds.resizeData(total))
		h.must(m.Costs.resizeData(total))
		m.Basis = make([]uint32, m.A.RowCount())
	}
	return m
}
func lpRationalPrimalOuterOperation(m *RationalPrimalSimplex, op string, out *numeral.Rational) error {
	switch op {
	case "find":
		return m.FindMaximalSolution()
	case "prepare":
		return m.PrepareStageOneTotalInf()
	case "row":
		return m.FillAXAndBasisForStageOneTotalInfForRow(0)
	case "bounds":
		return m.SetCoreSolverBounds()
	case "solve":
		return m.SolveWithTotalInf()
	case "cost":
		return m.GetCurrentCost(out)
	case "value":
		return m.GetColumnValue(10, out)
	}
	panic(op)
}
func TestLPRationalPrimalOuterPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, op := range []string{"find", "prepare", "row", "bounds", "solve", "cost", "value"} {
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
					m := lpRationalPrimalOuterOperationFixture(t, h, op)
					out := lpScalarBoundValue(h, "97")
					h.budget.FailAfter(ordinal)
					e := lpRationalPrimalOuterOperation(m, op, out)
					h.budget.FailAfter(-1)
					if e == nil {
						success, done = true, true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(e) {
							t.Fatal(e)
						}
						if retry := lpRationalPrimalOuterOperation(m, op, out); retry != ErrExactVectorPending {
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
func TestLPRationalPrimalOuterPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, op := range []string{"find", "prepare", "row", "bounds", "solve", "cost", "value"} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			m := lpRationalPrimalOuterOperationFixture(t, h, op)
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
				_ = lpRationalPrimalOuterOperation(m, op, out)
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

func TestLPRationalPrimalStageMetadataPublication(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, phase := range []string{"matrix", "row-map", "constraint-map", "basis", "rhs", "x", "types", "low", "upper", "relation", "bound-index", "bound-scale", "nil-column", "acceptable-x", "nil-cost-output"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			m := lpRationalPrimalOuterLarge(t, h)
			lpRationalPrimalOuterPrepare(t, h, m)
			total := m.A.ColumnCount() + m.A.RowCount()
			h.must(m.X.resizeData(total))
			h.must(m.LowBounds.resizeData(total))
			h.must(m.Costs.resizeData(total))
			m.Basis = make([]uint32, m.A.RowCount())
			a, b, x, low, up, scale, types, basis := m.A, m.B.Data, m.X.Data, m.LowBounds.Data, m.UpperBounds.Data, m.ColumnScale.Data, m.ColumnTypes, m.Basis
			c, _ := m.Columns.Get2(10)
			idx := c.Index
			constraint, _ := m.Constraints.Get2(100)
			oldRelation := constraint.Relation
			want := error(ErrBounds)
			var e error
			switch phase {
			case "matrix":
				m.A = nil
				e = m.PrepareStageOneTotalInf()
			case "row-map":
				delete(m.CoreRowsToExternal, 0)
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "constraint-map":
				m.Constraints.Del(100)
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "basis":
				m.Basis = nil
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "rhs":
				m.B.Data = nil
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "x":
				m.X.Data = m.X.Data[:2]
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "types":
				m.ColumnTypes = m.ColumnTypes[:2]
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "low":
				m.LowBounds.Data = m.LowBounds.Data[:2]
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "upper":
				m.UpperBounds.Data = m.UpperBounds.Data[:2]
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "relation":
				constraint.Relation = LPRelation(99)
				want = nil
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				e = m.FillAXAndBasisForStageOneTotalInfForRow(0)
			case "bound-index":
				c.Index = uint32(len(m.ColumnTypes) + 9)
				e = m.SetCoreSolverBounds()
			case "bound-scale":
				m.ColumnScale.Data = nil
				e = m.SetCoreSolverBounds()
			case "nil-column":
				m.Columns.Set(99, nil)
				e = m.SetCoreSolverBounds()
			case "acceptable-x":
				m.X.Data = nil
				e = m.FillAcceptableValuesForX()
			case "nil-cost-output":
				e = m.GetCurrentCost(nil)
			}
			if e != want {
				t.Fatal(e, want)
			}
			if phase == "types" || phase == "low" || phase == "upper" {
				if m.Basis[0] != 2 || m.A.ColumnCount() != 3 {
					t.Fatal("late publication", m.Basis, m.A.ColumnCount())
				}
				lpSparseRationalCheck(t, h, &m.X.Data[2], lpRationalTestBig(strings.Repeat("1234567890", 10)+"5/29"))
			}
			if e != nil {
				if retry := m.FindMaximalSolution(); retry != ErrExactVectorPending {
					t.Fatal("retry", retry)
				}
			}
			m.A = a
			m.B.Data = b
			m.X.Data = x
			m.LowBounds.Data = low
			m.UpperBounds.Data = up
			m.ColumnScale.Data = scale
			m.ColumnTypes = types
			m.Basis = basis
			c.Index = idx
			constraint.Relation = oldRelation
			// New fields inserted only for the guard own no numerical values.
			h.must(m.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLPRationalPrimalStageReporterAndFlush(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	const want = "extended A[1,3]\n"
	for cut := 0; cut <= len(want); cut++ {
		h := newLPPairHarness(t)
		m := lpRationalPrimalOuterLarge(t, h)
		lpRationalPrimalOuterPrepare(t, h, m)
		w := &lpResidualFailWriter{remaining: cut}
		m.Settings.MessageOut = lpResidualWriterOnly{writer: w}
		e := m.PrepareStageOneTotalInf()
		if w.String() != want[:cut] {
			t.Fatal(cut, w.String())
		}
		if cut < len(want) {
			if e != lpErrResidualWrite || m.A.ColumnCount() != 3 {
				t.Fatal(cut, e)
			}
		} else {
			h.must(e)
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	m := lpRationalPrimalOuterLarge(t, h)
	lpRationalPrimalOuterPrepare(t, h, m)
	w := &lpRationalDualDiagnosticWriter{flushErr: lpErrResidualWrite}
	m.Settings.MessageOut = w
	e := m.PrepareStageOneTotalInf()
	if e != lpErrResidualWrite || w.flushes != 1 || w.String() != want {
		t.Fatal(e, w.flushes, w.String())
	}
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalPrimalOuterCostAliasesAndOwnerEdges(t *testing.T) {
	for _, alias := range []bool{false, true} {
		h := newLPPairHarness(t)
		m := lpRationalPrimal(t, h)
		want := new(big.Rat)
		for i := uint32(0); i < 5; i++ {
			c := big.NewRat(int64(i+3), 7)
			v := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(int64(i+1)), uint(85+i)), big.NewInt(19))
			h.must(m.SetCostForColumn(i, lpScalarBoundValue(h, c.RatString())))
			h.must(m.SetFixedValue(i, lpScalarBoundValue(h, v.RatString())))
			want.Add(want, new(big.Rat).Mul(c, v))
		}
		out := lpScalarBoundValue(h, "97")
		if alias {
			c, _ := m.Columns.Get2(4)
			out = &c.Cost
		}
		h.must(m.GetCurrentCost(out))
		lpSparseRationalCheck(t, h, out, want)
		h.must(m.Close())
		if e := m.GetCurrentCost(out); e != ErrExactVectorPending {
			t.Fatal(e)
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	m := lpRationalPrimal(t, h)
	m.A = NewRationalStaticMatrix(h.m, 0, 0)
	h.must(m.SolveWithTotalInf())
	if m.Status != StatusOptimal || m.CoreSolver != nil {
		t.Fatal(m.Status)
	}
	// Invalid indices are unmapped and skipped; mapped fixed upper bounds are zero.
	c, e := m.GetOrCreateColumnInfo(10)
	h.must(e)
	c.Index = InvalidColumnIndex
	h.must(m.SetCoreSolverBounds())
	h.must(m.A.AddColumn())
	c.Index = 0
	h.must(c.SetFixedValue(lpScalarBoundValue(h, "3")))
	h.must(m.SetCoreSolverBounds())
	lpSparseRationalCheck(t, h, &m.UpperBounds.Data[0], new(big.Rat))
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
	if _, e := NewRationalPrimalSimplex(nil); e != ErrExactVectorOwner {
		t.Fatal(e)
	}
}
