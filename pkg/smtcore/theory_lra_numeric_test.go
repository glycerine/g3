package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSMTLRANumericFixedBoundQueryRefusalCleanup(t *testing.T) {
	refusals, successes := 0, 0
	for _, allowance := range []int64{-1, 0, 1, 2, 5, 12, 32} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			budget := arena.NewBudget(8 << 20)
			m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
				Cells: arena.Config{PageSize: 4, Budget: budget}, Words: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 4, Budget: budget}},
			}, Synchronized: true})
			astTestMust(t, err)
			s, err := newSMTLRANumeric(m, smtLRANumericConfig{strategy: lp.StrategyTableauRows})
			astTestMust(t, err)
			var padding []numeral.Rational
			t.Cleanup(func() {
				budget.FailAfter(-1)
				astTestMust(t, s.Close())
				for i := range padding {
					astTestMust(t, m.Del(&padding[i]))
				}
				lraNumericManagerCensus(t, m)
				astTestMust(t, m.CloseChecked())
				if budget.Used() != 0 {
					t.Fatal("fixed query retained private budget", budget.Used())
				}
			})
			ten := new(big.Int).Exp(big.NewInt(10), big.NewInt(300), nil)
			value := new(big.Rat).SetFrac(new(big.Int).Add(ten, big.NewInt(13)), new(big.Int).Add(new(big.Int).Quo(ten, big.NewInt(10)), big.NewInt(7)))
			ci, err := s.assertBound(0, 101, value, false, true)
			astTestMust(t, err)
			index, _ := s.index(0)
			for i := 0; i < 64; i++ {
				stats := m.Z.Cells.Stats()
				if stats.Slots-stats.Live-stats.Retired <= 1 {
					break
				}
				v := numeral.NewRational()
				astTestMust(t, m.SetInt64(&v, 1<<60))
				padding = append(padding, v)
			}
			stats := m.Z.Cells.Stats()
			if stats.Slots-stats.Live-stats.Retired > 1 {
				t.Fatal("failed to reach cold numeric copy frontier", stats)
			}
			budget.FailAfter(allowance)
			witness, found, err := s.hasRationalBound(index, true, value)
			budget.FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && allowance >= 0 && budget.Used() < budget.Limit()
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatal(err)
				}
				refusals++
				if len(s.pending) == 0 {
					t.Fatal("refused bound output lost its owner")
				}
				if _, _, next := s.hasRationalBound(index, true, value); next != lp.ErrExactVectorPending {
					t.Fatal("stopped bound query was reused", next)
				}
			} else {
				successes++
				if !found || witness != ci {
					t.Fatal("fixed bound query lost exact value or witness", found, witness, ci)
				}
			}
		})
	}
	if refusals == 0 || successes == 0 {
		t.Fatal("fixed query samples require refusal and success", refusals, successes)
	}
}

func lraNumericFixture(t *testing.T, strategy lp.SimplexStrategy) *smtLRANumeric {
	t.Helper()
	m := astTestManager(t)
	manager, err := m.NumeralService().NewPrivateRationalManager()
	astTestMust(t, err)
	s, err := newSMTLRANumeric(manager, smtLRANumericConfig{strategy: strategy, boundPropagation: true, pivotedRows: true})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, manager.CloseChecked())
	})
	return s
}

func lraCoefficient(v SMTTheoryVar, n, d int64) smtLRACoefficient {
	return smtLRACoefficient{v, *big.NewRat(n, d)}
}

func lraBound(t *testing.T, s *smtLRANumeric, v SMTTheoryVar, bv int32, n, d int64, upper, truth bool) uint32 {
	t.Helper()
	ci, err := s.assertBound(v, bv, big.NewRat(n, d), upper, truth)
	astTestMust(t, err)
	return ci
}

func lraFeasible(t *testing.T, s *smtLRANumeric, want lbool.LBool) {
	t.Helper()
	status, err := s.makeFeasible()
	astTestMust(t, err)
	if status != want {
		t.Fatal("LAR feasibility", status, "want", want)
	}
}

func TestSMTLRANumericOrderedCancellationAndRegistration(t *testing.T) {
	s := lraNumericFixture(t, lp.StrategyTableauRows)
	coefficients := []smtLRACoefficient{lraCoefficient(17, 1, 1), lraCoefficient(22, 3, 1), lraCoefficient(17, -1, 1), lraCoefficient(30, 2, 1), lraCoefficient(22, -2, 1)}
	index, err := s.addTerm(42, coefficients, big.NewRat(7, 3))
	astTestMust(t, err)
	if index != lp.LARTermStartIndex || len(s.variables) != 2 || s.variables[0] != 22 || s.variables[1] != 30 || s.terms[0] != 42 {
		t.Fatal("first surviving variable order", index, s.variables, s.terms)
	}
	if _, found := s.index(17); found {
		t.Fatal("cancelled variable registered")
	}
	coefficients[1].value.SetInt64(999)
	term := s.solver.Terms[0]
	var columns []uint32
	for j, c := range term.Coefficients().All() {
		columns = append(columns, j)
		value, err := s.rational(c)
		astTestMust(t, err)
		if value.Cmp(big.NewRat(int64(j+1), 1)) != 0 {
			t.Fatal("coefficient not owned", j, value)
		}
	}
	// lar_term's unordered_map iterates column 1 before column 0.
	if fmt.Sprint(columns) != "[1 0]" {
		t.Fatal(columns)
	}
	index2, err := s.getVarIndex(42)
	astTestMust(t, err)
	if index2 != index || len(s.trail) != 3 {
		t.Fatal("term mapping replaced by a fresh column", index2, s.trail)
	}
	if s.solver.Settings.SimplexStrategy != lp.StrategyTableauRows || !s.solver.Settings.BoundPropagation || s.solver.Core.RPrimal.PivotedRows != &s.solver.RowsWithChangedBounds {
		t.Fatal("LRA initialization settings lost")
	}
}

func TestSMTLRANumericStrictOptimizationModelsAndScopeRecovery(t *testing.T) {
	for _, strategy := range []lp.SimplexStrategy{lp.StrategyTableauRows, lp.StrategyTableauCosts} {
		t.Run(fmt.Sprint(strategy), func(t *testing.T) {
			s := lraNumericFixture(t, strategy)
			_, err := s.addTerm(12, []smtLRACoefficient{lraCoefficient(5, 2, 1), lraCoefficient(9, -1, 1)}, big.NewRat(3, 1))
			astTestMust(t, err)
			lraBound(t, s, 5, 101, 0, 1, false, true)
			lraBound(t, s, 5, 102, 3, 1, false, false) // x<3
			lraBound(t, s, 9, 103, 0, 1, false, true)
			lraBound(t, s, 9, 104, 2, 1, true, true)
			lraFeasible(t, s, lbool.True)
			maximum, err := s.maximize(12)
			astTestMust(t, err)
			if maximum.Unbounded {
				t.Fatal("bounded affine objective")
			}
			checkInfiniteArithmetic(t, maximum.Finite, 0, 9, -2)
			current, err := s.value(12)
			astTestMust(t, err)
			if current.Cmp(&maximum.Finite) != 0 {
				t.Fatal("LAR assignment and objective diverged", current.String(), maximum.Finite.String())
			}
			old, err := s.modelValues([]SMTTheoryVar{5, 9, 12, 999})
			astTestMust(t, err)
			want := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), old[5]), old[9]))
			if old[5].Sign() < 0 || old[5].Cmp(big.NewRat(3, 1)) >= 0 || old[9].Sign() != 0 || old[12].Cmp(want) != 0 || old[999].Sign() != 0 {
				t.Fatal("strict finite witness", old)
			}
			oldValue := new(big.Rat).Set(old[12])
			astTestMust(t, s.push())
			lraBound(t, s, 5, 105, 2, 1, true, true)
			_, err = s.addTerm(44, []smtLRACoefficient{lraCoefficient(31, 1, 1)}, big.NewRat(1, 1))
			astTestMust(t, err)
			lraFeasible(t, s, lbool.True)
			maximum, err = s.maximize(12)
			astTestMust(t, err)
			checkInfiniteArithmetic(t, maximum.Finite, 0, 7, 0)
			if old[12].Cmp(oldValue) != 0 {
				t.Fatal("previous model borrowed mutable solver state")
			}
			astTestMust(t, s.pop(1))
			for _, v := range []SMTTheoryVar{31, 44} {
				if _, ok := s.index(v); ok {
					t.Fatal("popped registration survived", v)
				}
			}
			if s.terms[1] != SMTNullTheoryVar || s.variables[2] != SMTNullTheoryVar {
				t.Fatal("reverse registration survived pop")
			}
			lraFeasible(t, s, lbool.True)
			maximum, err = s.maximize(12)
			astTestMust(t, err)
			checkInfiniteArithmetic(t, maximum.Finite, 0, 9, -2)
		})
	}
	checkNativeLRANumeric(t)
}

func TestSMTLRANumericConflictEvidenceAndReusedConstraintIndex(t *testing.T) {
	s := lraNumericFixture(t, lp.StrategyTableauRows)
	lraBound(t, s, 5, 101, 0, 1, false, true)
	for _, bv := range []int32{102, 103} {
		astTestMust(t, s.push())
		ci := lraBound(t, s, 5, bv, 0, 1, false, false)
		if ci != 1 {
			t.Fatal("constraint index not reused", ci)
		}
		lraFeasible(t, s, lbool.False)
		evidence, err := s.infeasibilityEvidence()
		astTestMust(t, err)
		got := make(map[SMTLiteral]bool)
		for _, e := range evidence {
			if e.coefficient.Sign() == 0 {
				t.Fatal("zero explanation coefficient")
			}
			got[e.literal] = true
		}
		if len(got) != 2 || !got[NewSMTLiteral(101, false)] || !got[NewSMTLiteral(bv, true)] {
			t.Fatal("stale or unsigned explanation", got)
		}
		before := s.assertLower + s.assertUpper
		if ci := lraBound(t, s, 6, 999, 0, 1, true, true); ci != smtLRANoIndex || s.assertLower+s.assertUpper != before {
			t.Fatal("infeasible LAR accepted another bound")
		}
		astTestMust(t, s.pop(1))
		lraFeasible(t, s, lbool.True)
	}
}

func TestSMTLRANumericUnboundedAndConstantTerm(t *testing.T) {
	s := lraNumericFixture(t, lp.StrategyTableauRows)
	value, err := s.maximize(5)
	astTestMust(t, err)
	if !value.Unbounded {
		t.Fatal("unregistered objective")
	}
	_, err = s.getVarIndex(5)
	astTestMust(t, err)
	lraFeasible(t, s, lbool.True)
	value, err = s.maximize(5)
	astTestMust(t, err)
	if !value.Unbounded {
		t.Fatal("free objective")
	}
	_, err = s.addTerm(6, nil, big.NewRat(7, 3))
	astTestMust(t, err)
	lraFeasible(t, s, lbool.True)
	value, err = s.maximize(6)
	astTestMust(t, err)
	if value.Unbounded || value.Finite.Main.Cmp(big.NewRat(7, 3)) != 0 || !value.Finite.IsRational() {
		t.Fatal(value)
	}
	astTestMust(t, s.Close())
	astTestMust(t, s.Close())
	if _, err = s.makeFeasible(); err == nil {
		t.Fatal("closed owner reused")
	}
}

func TestSMTLRANumericInterruptedRegistrationCleanup(t *testing.T) {
	for _, allowance := range []int64{-1, 0, 1, 2, 5, 12, 32} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			budget := arena.NewBudget(8 << 20)
			manager, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
				Cells: arena.Config{PageSize: 4, Budget: budget},
				Words: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 4, Budget: budget}},
			}, Synchronized: true})
			astTestMust(t, err)
			s, err := newSMTLRANumeric(manager, smtLRANumericConfig{strategy: lp.StrategyTableauRows})
			astTestMust(t, err)
			t.Cleanup(func() {
				budget.FailAfter(-1)
				astTestMust(t, s.Close())
				astTestMust(t, manager.CloseChecked())
				if budget.Used() != 0 {
					t.Fatal("retained allocator budget", budget.Used())
				}
			})
			large := new(big.Int).Exp(big.NewInt(10), big.NewInt(300), nil)
			c := new(big.Rat).SetFrac(large, big.NewInt(7))
			budget.FailAfter(allowance)
			_, err = s.addTerm(9, []smtLRACoefficient{{5, *c}}, c)
			budget.FailAfter(-1)
			if allowance == 0 && err == nil {
				t.Fatal("allocation refusal was not exercised")
			}
			if allowance == -1 {
				astTestMust(t, err)
			}
			if err != nil {
				if _, next := s.makeFeasible(); next != lp.ErrExactVectorPending {
					t.Fatal("stopped registration reused", err, next)
				}
			}
			astTestMust(t, s.Close())
			if allowance == -1 {
				lraNumericManagerCensus(t, manager)
			}
		})
	}
}

func lraNumericManagerCensus(t *testing.T, m *numeral.RationalManager) {
	t.Helper()
	owned := make(map[numeral.NumericCellRef]bool)
	add := func(v *numeral.Int) {
		if v.Cell == 0 || !numeral.ValidNumericRef(m.Z.Cells, v.Cell) {
			return
		}
		if owned[v.Cell] {
			t.Fatal("duplicated remaining numeral owner", v.Cell)
		}
		owned[v.Cell] = true
	}
	for _, ref := range m.Z.Tmp {
		v := numeral.Int{Cell: ref}
		add(&v)
	}
	for _, ref := range m.Z.Arg {
		v := numeral.Int{Cell: ref}
		add(&v)
	}
	for _, v := range []*numeral.Int{&m.Z.IntMin, &m.Z.Two64, &m.NTmp, &m.AddTmp1, &m.AddTmp2} {
		add(v)
	}
	for _, v := range []*numeral.Rational{&m.AddMulTmp, &m.LtTmp1, &m.LtTmp2} {
		add(&v.Num)
		add(&v.Den)
	}
	if uint32(len(owned)) != m.Z.Cells.Stats().Live {
		t.Fatal("orphan LRA numeric cells", len(owned), m.Z.Cells.Stats().Live)
	}
}

func TestSMTLRANumericCoupledRowsStrictOptimumAndConflict(t *testing.T) {
	s := lraNumericFixture(t, lp.StrategyTableauRows)
	_, err := s.addTerm(12, []smtLRACoefficient{lraCoefficient(5, 1, 1), lraCoefficient(9, 1, 1)}, new(big.Rat))
	astTestMust(t, err)
	_, err = s.addTerm(13, []smtLRACoefficient{lraCoefficient(5, 5, 1), lraCoefficient(9, 3, 1)}, big.NewRat(2, 1))
	astTestMust(t, err)
	lraBound(t, s, 5, 101, 0, 1, false, true)
	lraBound(t, s, 5, 102, 3, 1, true, true)
	lraBound(t, s, 9, 103, 0, 1, false, true)
	lraBound(t, s, 9, 104, 2, 1, true, true)
	lraBound(t, s, 12, 105, 4, 1, true, true)
	lraFeasible(t, s, lbool.True)
	value, err := s.maximize(13)
	astTestMust(t, err)
	if value.Unbounded {
		t.Fatal("bounded coupled objective")
	}
	// Native LAR leaves the preceding OPTIMAL status in the core. A raw
	// maximize callback can therefore stop after one improving iteration.
	// optsmt's retained caller repeats search/maximize with a blocker instead
	// of treating this first point as a certified global optimum.
	checkInfiniteArithmetic(t, value.Finite, 0, 17, 0)
	value, err = s.maximize(13)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 0, 20, 0)
	astTestMust(t, s.push())
	lraBound(t, s, 12, 106, 4, 1, false, false)
	lraFeasible(t, s, lbool.True)
	value, err = s.maximize(13)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value.Finite, 0, 20, -3)
	astTestMust(t, s.push())
	lraBound(t, s, 12, 107, 0, 1, false, false)
	lraFeasible(t, s, lbool.False)
	evidence, err := s.infeasibilityEvidence()
	astTestMust(t, err)
	want := map[SMTLiteral]bool{NewSMTLiteral(101, false): true, NewSMTLiteral(103, false): true, NewSMTLiteral(107, true): true}
	for _, e := range evidence {
		if !want[e.literal] {
			t.Fatal("extraneous bound in row explanation", e)
		}
		delete(want, e.literal)
	}
	if len(want) != 0 {
		t.Fatal("missing coupled conflict premises", want)
	}
	astTestMust(t, s.pop(2))
	lraFeasible(t, s, lbool.True)
	// Recovered basis and pricing order retain the native progression. The
	// prebuilt C++ optsmt caller reports the same 8,18,20 sequence before it
	// certifies the final objective. Do not reset status or add a new inner
	// optimization loop to hide this source callback behavior.
	for _, bound := range []int64{8, 18, 20} {
		value, err = s.maximize(13)
		astTestMust(t, err)
		checkInfiniteArithmetic(t, value.Finite, 0, bound, 0)
	}
	checkNativeLRACoupledProgression(t)
}
