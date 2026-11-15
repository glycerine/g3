package lp

// Source: mps_reader.h:796-800; retained exact consumers in src/test/lp.cpp.

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Test-only concrete dispatch exercises both factories without introducing a
// production solver adapter or replacing either solver's actual value getter.
type lpMPSExactOuter struct {
	p *RationalPrimalSimplex
	d *RationalDualSimplex
}

func lpCreateMPSExactOuter(r *RationalMPSReader, dual bool) (lpMPSExactOuter, error) {
	if dual {
		d, err := r.CreateDualSolver()
		return lpMPSExactOuter{d: d}, err
	}
	p, err := r.CreatePrimalSolver()
	return lpMPSExactOuter{p: p}, err
}
func (s lpMPSExactOuter) base() *RationalLPSolver {
	if s.p != nil {
		return s.p.RationalLPSolver
	}
	if s.d != nil {
		return s.d.RationalLPSolver
	}
	return nil
}
func (s lpMPSExactOuter) close() error {
	if s.p != nil {
		return s.p.Close()
	}
	if s.d != nil {
		return s.d.Close()
	}
	return nil
}
func (s lpMPSExactOuter) named(name string, out *numeral.Rational) error {
	if s.p != nil {
		return s.p.GetColumnValueByName(name, out)
	}
	return s.d.GetColumnValueByName(name, out)
}

func TestLPRationalMPSOuterIndependentOptima(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	rng := rand.New(rand.NewSource(796800))
	for world := 0; world < 60; world++ {
		h := newLPPairHarness(t)
		shift := []*big.Rat{big.NewRat(-2, 3), big.NewRat(3, 7)}
		cost := []*big.Rat{big.NewRat(int64(1+rng.Intn(9)), 11), big.NewRat(int64(1+rng.Intn(9)), 13)}
		fixed, fixedCost := big.NewRat(5, 29), new(big.Rat).SetFrac(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(7)), big.NewInt(17))
		a := make([][]*big.Rat, 5)
		b := make([]*big.Rat, 5)
		originalB := make([]*big.Rat, 3)
		var input strings.Builder
		input.WriteString("NAME EXACT\nROWS\n N COST\n L R0\n L R1\n L R2\nCOLUMNS\n")
		for i := range a {
			a[i] = make([]*big.Rat, 7)
			for j := range a[i] {
				a[i][j] = new(big.Rat)
			}
			a[i][2+i].SetInt64(1)
			if i < 3 {
				a[i][0].SetFrac64(int64(1+rng.Intn(8)), 3)
				a[i][1].SetFrac64(int64(1+rng.Intn(8)), 5)
				b[i] = big.NewRat(int64(3+rng.Intn(20)), 2)
				originalB[i] = new(big.Rat).Set(b[i])
				for j := 0; j < 2; j++ {
					originalB[i].Add(originalB[i], new(big.Rat).Mul(a[i][j], shift[j]))
				}
				originalB[i].Add(originalB[i], new(big.Rat).Mul(big.NewRat(int64(i+1), 1), fixed))
			} else {
				a[i][i-3].SetInt64(1)
				b[i] = big.NewRat(int64(6+i-3), 1)
			}
		}
		for j, name := range []string{"x", "y"} {
			input.WriteString(lpExactMPSPair(name, "COST", cost[j].RatString()))
			for i := 0; i < 3; i++ {
				input.WriteString(lpExactMPSPair(name, fmt.Sprint("R", i), a[i][j].RatString()))
			}
		}
		input.WriteString(lpExactMPSPair("f", "COST", fixedCost.RatString()))
		for i := 0; i < 3; i++ {
			input.WriteString(lpExactMPSPair("f", fmt.Sprint("R", i), fmt.Sprint(i+1)))
		}
		input.WriteString("RHS\n")
		for i := 0; i < 3; i++ {
			input.WriteString(lpExactMPSPair("RHS1", fmt.Sprint("R", i), originalB[i].RatString()))
		}
		input.WriteString("BOUNDS\n")
		for j, name := range []string{"x", "y"} {
			input.WriteString(lpMPSBoundLine("LO", name+" "+shift[j].RatString()))
			input.WriteString(lpMPSBoundLine("UP", name+" "+new(big.Rat).Add(shift[j], b[3+j]).RatString()))
		}
		input.WriteString(lpMPSBoundLine("FX", "f "+fixed.RatString()) + "ENDATA\n")
		r := lpReadExactMPS(t, h, input.String())
		for _, minimum := range []bool{false, true} {
			objective := make([]*big.Rat, 7)
			for j := range objective {
				objective[j] = new(big.Rat)
				if j < 2 {
					objective[j].Set(cost[j])
					if !minimum {
						objective[j].Neg(objective[j])
					}
				}
			}
			want := lpIndependentRationalVertexMinimum(a, b, objective)
			if want == nil {
				t.Fatal("oracle has no vertex", world)
			}
			if !minimum {
				want.Neg(want)
			}
			want.Add(want, new(big.Rat).Mul(fixed, fixedCost))
			for j := 0; j < 2; j++ {
				want.Add(want, new(big.Rat).Mul(cost[j], shift[j]))
			}
			s, err := r.CreatePrimalSolver()
			h.must(err)
			s.Settings.SimplexStrategy = StrategyLU
			s.Settings.UseScaling = world%2 == 0
			s.Settings.MessageOut, s.Settings.DebugOut = nil, nil
			s.Settings.MaxTotalIterations = 1000
			if minimum {
				h.must(s.FlipCosts())
			}
			h.must(s.FindMaximalSolution())
			out := lpScalarBoundValue(h, "-97")
			h.must(s.GetCurrentCost(out))
			if minimum {
				h.must(h.m.Neg(out))
			}
			if s.Status != StatusOptimal || s.CoreSolver == nil || s.TotalIterations != s.CoreSolver.TotalIterations {
				t.Fatal(world, minimum, s.Status)
			}
			lpSparseRationalCheck(t, h, out, want)
			values := make([]*big.Rat, 3)
			for j, name := range []string{"x", "y", "f"} {
				h.must(s.GetColumnValueByName(name, out))
				text, err := h.m.String(out)
				h.must(err)
				values[j], _ = new(big.Rat).SetString(text)
			}
			for j := 0; j < 2; j++ {
				if values[j].Cmp(shift[j]) < 0 || values[j].Cmp(new(big.Rat).Add(shift[j], b[3+j])) > 0 {
					t.Fatal("external bounds", world, minimum, values)
				}
			}
			if values[2].Cmp(fixed) != 0 {
				t.Fatal("fixed external value")
			}
			for i := 0; i < 3; i++ {
				lhs := new(big.Rat).Mul(values[2], big.NewRat(int64(i+1), 1))
				for j := 0; j < 2; j++ {
					lhs.Add(lhs, new(big.Rat).Mul(a[i][j], values[j]))
				}
				if lhs.Cmp(originalB[i]) > 0 {
					t.Fatal("external constraint", world, minimum, i)
				}
			}
			h.must(s.Close())
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalMPSOuterIndependentClones(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 40; world++ {
		h := newLPPairHarness(t)
		value := new(big.Rat).SetFrac(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(80+world)), big.NewInt(3)), big.NewInt(19))
		input := "NAME CLONES\nROWS\n N COST\n L L\n G G\n E E\nCOLUMNS\n"
		for _, name := range []string{"default", "free", "boxed", "fixed"} {
			for _, row := range []string{"COST", "L", "G", "E"} {
				input += lpExactMPSPair(name, row, value.RatString())
			}
		}
		input += "RHS\n"
		for _, row := range []string{"L", "G", "E"} {
			input += lpExactMPSPair("RHS1", row, "7/13")
		}
		input += "RANGES\n" + lpExactMPSPair("RNG", "L", "3/17") + "BOUNDS\n" + lpMPSBoundLine("FR", "free") + lpMPSBoundLine("LO", "boxed -5/11") + lpMPSBoundLine("UP", "boxed 9/23") + lpMPSBoundLine("FX", "fixed "+value.RatString()) + "ENDATA\n"
		r := lpReadExactMPS(t, h, input)
		// Source create_solver does not require the parse success flag.
		r.ok = false
		var owners []lpMPSExactOuter
		for _, dual := range []bool{false, true, false, true} {
			s, err := lpCreateMPSExactOuter(r, dual)
			h.must(err)
			owners = append(owners, s)
			m := s.base()
			if m.Columns.Len() != 4 || m.Constraints.Len() != 4 {
				t.Fatal("extent")
			}
			for j, name := range []string{"default", "free", "boxed", "fixed"} {
				c, _ := m.Columns.Get2(uint32(j))
				if c == nil || c.Name != name || c.ColumnType() != []ColumnType{ColumnLowBound, ColumnFree, ColumnBoxed, ColumnFixed}[j] {
					t.Fatal(name, c)
				}
				lpSparseRationalCheck(t, h, &c.Cost, value)
			}
			for i, relation := range []LPRelation{RelationLessOrEqual, RelationGreaterOrEqual, RelationEqual, RelationGreaterOrEqual} {
				c, _ := m.Constraints.Get2(uint32(i))
				if c == nil || c.Relation != relation {
					t.Fatal("relation", i)
				}
				want := big.NewRat(7, 13)
				if i == 3 {
					want.Sub(want, big.NewRat(3, 17))
				}
				lpSparseRationalCheck(t, h, c.RightSide, want)
			}
		}
		first, _ := owners[0].base().Columns.Get2(0)
		h.must(h.m.SetInt32(&first.Cost, 97))
		for _, s := range owners[1:] {
			c, _ := s.base().Columns.Get2(0)
			if c == first {
				t.Fatal("aliased column owner")
			}
			lpSparseRationalCheck(t, h, &c.Cost, value)
		}
		row, _ := r.rows.Get2("COST")
		v, _ := row.coefficients.Get2("default")
		lpSparseRationalCheck(t, h, v, value)
		for i := len(owners) - 1; i >= 0; i-- {
			h.must(owners[i].close())
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalMPSOuterFullDualSourceBoundary(t *testing.T) {
	for _, boxed := range []bool{false, true} {
		h := newLPPairHarness(t)
		input := "NAME DUAL\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("y", "COST", "2") + lpExactMPSPair("x", "R", "1") + lpExactMPSPair("y", "R", "1") + "RHS\n" + lpExactMPSPair("RHS1", "R", "3")
		if boxed {
			input += "BOUNDS\n" + lpMPSBoundLine("UP", "x 6") + lpMPSBoundLine("UP", "y 6")
		}
		r := lpReadExactMPS(t, h, input+"ENDATA\n")
		s, err := r.CreateDualSolver()
		h.must(err)
		s.Settings.MessageOut, s.Settings.DebugOut = nil, nil
		s.Settings.SimplexStrategy = StrategyLU
		err = s.FindMaximalSolution()
		if sourceDebugEnabled {
			if err != ErrAssertion || s.CoreSolver != nil || s.stageWork == nil || len(s.BCopy.Data) != 0 {
				t.Fatal("structural conversion boundary", boxed, err)
			}
		} else if err != ErrBounds || s.CoreSolver == nil || s.CoreSolver.pivotWork == nil || len(s.CoreSolver.RowsNZ) != 0 {
			t.Fatal("source missing sparsity extent", boxed, err)
		}
		if r.stopped {
			t.Fatal("solver failure stopped independent reader")
		}
		h.must(s.Close())
		p, err := r.CreatePrimalSolver()
		h.must(err)
		p.Settings.MessageOut, p.Settings.DebugOut = nil, nil
		p.Settings.SimplexStrategy = StrategyLU
		h.must(p.FindMaximalSolution())
		out := lpScalarBoundValue(h, "-1")
		h.must(p.GetCurrentCost(out))
		lpSparseRationalCheck(t, h, out, big.NewRat(6, 1))
		h.must(p.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalMPSOuterRetainedFile(t *testing.T) {
	h := newLPPairHarness(t)
	r := OpenRationalMPSReader(h.m, filepath.Join("test_vectors", "mps_small.mps"))
	r.Messages = &bytes.Buffer{}
	h.must(r.Read())
	if !r.IsOK() {
		t.Fatal(r.Messages)
	}
	s, err := r.CreatePrimalSolver()
	h.must(err)
	s.Settings.MessageOut, s.Settings.DebugOut = nil, nil
	s.Settings.SimplexStrategy = StrategyLU
	h.must(s.FindMaximalSolution())
	if s.Status != StatusOptimal {
		t.Fatal(s.Status)
	}
	out := lpScalarBoundValue(h, "-1")
	h.must(s.GetCurrentCost(out))
	// Independent vertices: x+y<=6, 2*x+y<=8, 0<=x<=4,
	// 0<=y<=5; fixed f=2 contributes 10. The range also requires x+y>=1.
	lpSparseRationalCheck(t, h, out, big.NewRat(24, 1))
	for i, name := range []string{"x", "y", "f"} {
		h.must(s.GetColumnValueByName(name, out))
		lpSparseRationalCheck(t, h, out, []*big.Rat{big.NewRat(2, 1), big.NewRat(4, 1), big.NewRat(2, 1)}[i])
	}
	h.must(s.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalMPSOuterFactoryGuards(t *testing.T) {
	for _, dual := range []bool{false, true} {
		for _, profile := range []string{"closed", "stopped", "manager", "work", "relation", "column", "unread"} {
			h := newLPPairHarness(t)
			r := lpReadExactMPS(t, h, "NAME GUARDS\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "R", "7/13")+"RHS\nENDATA\n")
			want := ErrExactVectorPending
			switch profile {
			case "closed":
				h.must(r.Close())
			case "stopped":
				r.stopped = true
			case "manager":
				r.manager = nil
				want = ErrExactVectorOwner
			case "work":
				r.work = NewIndexedRationalVector(h.m, 1)
			case "relation":
				row, _ := r.rows.Get2("R")
				row.typ = mpsCost
				want = ErrMPSUnhandled
			case "column":
				r.columns.Set("x", nil)
				want = ErrBounds
			case "unread":
				h.must(r.Close())
				r = NewRationalMPSReader(h.m, nil)
				want = nil
			}
			s, err := lpCreateMPSExactOuter(r, dual)
			if err != want {
				t.Fatal(dual, profile, err, want)
			}
			if profile == "manager" {
				r.manager = h.m
			}
			partial := profile == "relation" || profile == "column"
			if (partial || profile == "unread") != (s.base() != nil) {
				t.Fatal("partial owner publication", dual, profile)
			}
			if partial {
				retry, err := lpCreateMPSExactOuter(r, dual)
				if err != ErrExactVectorPending || retry.base() != nil {
					t.Fatal("failed reader retry", err)
				}
			}
			if profile == "relation" && (r.IsOK() || !strings.Contains(r.Messages.(*bytes.Buffer).String(), "Unexpected rt")) {
				t.Fatal("relation failure diagnostic")
			}
			if profile == "unread" && !s.base().ProblemIsEmpty() {
				t.Fatal("creation attempted implicit parsing")
			}
			h.must(s.close())
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPRationalMPSOuterFactoryPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, dual := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			for _, phase := range []string{"cost", "row", "bounds"} {
				success, failures, prefix := false, 0, false
				for ordinal := int64(0); ordinal < 2000 && !success; ordinal++ {
					if !t.Run(fmt.Sprintf("dual%t/direct%t/%s/%d", dual, direct, phase, ordinal), func(t *testing.T) {
						var h *lpPairHarness
						if direct {
							h = lpNewDirectSparseHarness(t)
						} else {
							h = newLPPairHarness(t)
						}
						large := strings.Repeat("1234567890", 100) + "7/13"
						// m_row_columns walks y before x (libstdc++ order), so
						// the cost phase puts its large value on x.
						input := "NAME REFUSE\nROWS\n N COST\n L R\nCOLUMNS\n"
						if phase == "cost" {
							input += lpExactMPSPair("x", "COST", large) + lpExactMPSPair("y", "COST", "7")
						} else {
							input += lpExactMPSPair("x", "COST", "7")
						}
						input += lpExactMPSPair("x", "R", "1")
						if phase == "row" {
							input += lpExactMPSPair("y", "R", large)
						}
						input += "RHS\n" + lpExactMPSPair("RHS1", "R", "3")
						if phase == "bounds" {
							input += "BOUNDS\n" + lpMPSBoundLine("LO", "x -"+large) + lpMPSBoundLine("UP", "x "+large) + lpMPSBoundLine("FX", "x "+large)
						}
						r := lpReadExactMPS(t, h, input+"ENDATA\n")
						h.budget.FailAfter(ordinal)
						s, err := lpCreateMPSExactOuter(r, dual)
						h.budget.FailAfter(-1)
						if s.base() == nil {
							t.Fatal("lost partial factory owner", err)
						}
						if err == nil {
							success = true
						} else {
							failures++
							if !lpExactPriorityPhysicalRefusal(err) {
								t.Fatal(err)
							}
							// A completed small cost survives a later refusal while
							// source symbolic names have not yet been published.
							for _, j := range []uint32{0, 1} {
								if c, ok := s.base().Columns.Get2(j); ok && c != nil && s.base().NamesToColumns.Len() == 0 {
									value, inspectErr := h.m.Int64(&c.Cost)
									if inspectErr == nil && value == 7 {
										prefix = true
									}
								}
							}
							retry, retryErr := lpCreateMPSExactOuter(r, dual)
							if retryErr != ErrExactVectorPending || retry.base() != nil {
								t.Fatal("factory retry", retryErr)
							}
						}
						// A partial model call can still borrow the reader's loop value.
						h.must(s.close())
						h.must(r.Close())
						lpExactVectorNamedCensus(t, h, nil)
					}) {
						t.FailNow()
					}
				}
				// fill_solver_on_rows visits row R before the cost row
				// (libstdc++ m_rows order), so the row phase fails before
				// any cost is published.
				if !success || direct && (failures == 0 || !prefix && phase != "row") {
					t.Fatal("factory refusal campaign", dual, direct, phase, success, failures, prefix)
				}
			}
		}
	}
}

func TestLPRationalMPSOuterNamedLookup(t *testing.T) {
	for _, dual := range []bool{false, true} {
		for _, profile := range []string{"fixed", "unprepared", "unknown", "orphan", "unsigned", "alias", "nil", "closed"} {
			h := newLPPairHarness(t)
			input := "NAME NAMED\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("f", "COST", "3") + lpExactMPSPair("x", "R", "1") + "RHS\nBOUNDS\n" + lpMPSBoundLine("FX", "f 5/29") + "ENDATA\n"
			r := lpReadExactMPS(t, h, input)
			s, err := lpCreateMPSExactOuter(r, dual)
			h.must(err)
			name, want := "f", big.NewRat(5, 29)
			out := lpScalarBoundValue(h, "-97")
			switch profile {
			case "unprepared":
				name, want = "x", new(big.Rat)
			case "unknown":
				name = " f\tmissing "
			case "orphan":
				s.base().NamesToColumns.Put("orphan", 77)
				name, want = "orphan", new(big.Rat)
			case "unsigned":
				h.must(s.base().SetFixedValue(^uint32(0), lpScalarBoundValue(h, "11/31")))
				h.must(s.base().GiveSymbolicNameToColumn("wide", ^uint32(0)))
				name, want = "wide", big.NewRat(11, 31)
				if lpTestNamedColumn(t, s.base().NamesToColumns, name) != ^uint32(0) {
					t.Fatal("full unsigned native name index")
				}
			case "alias":
				c, _ := s.base().Columns.Get2(0)
				out = &c.Cost
			case "nil":
				out = nil
			case "closed":
				h.must(s.close())
			}
			err = s.named(name, out)
			switch profile {
			case "unknown":
				if err == nil || err.Error() != "get_column_value_by_name "+name || s.named("f", out) != ErrExactVectorPending {
					t.Fatal("missing symbolic name exception", err)
				}
			case "nil":
				if err != ErrBounds {
					t.Fatal(err)
				}
			case "closed":
				if err != ErrExactVectorPending {
					t.Fatal(err)
				}
			default:
				h.must(err)
				lpSparseRationalCheck(t, h, out, want)
			}
			h.must(s.close())
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPRationalMPSOuterNamedPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, dual := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 1000 && !success; ordinal++ {
				if !t.Run(fmt.Sprintf("dual%t/direct%t/%d", dual, direct, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					large := strings.Repeat("1234567890", 100) + "7/13"
					r := lpReadExactMPS(t, h, "NAME NAMED\nROWS\n N COST\nCOLUMNS\n"+lpExactMPSPair("f", "COST", "3")+"RHS\nBOUNDS\n"+lpMPSBoundLine("FX", "f "+large)+"ENDATA\n")
					s, err := lpCreateMPSExactOuter(r, dual)
					h.must(err)
					out := lpScalarBoundValue(h, "97")
					h.budget.FailAfter(ordinal)
					err = s.named("f", out)
					h.budget.FailAfter(-1)
					if err == nil {
						success = true
						want, _ := new(big.Rat).SetString(large)
						lpSparseRationalCheck(t, h, out, want)
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) || s.named("f", out) != ErrExactVectorPending {
							t.Fatal("named refusal/retry", err)
						}
					}
					h.must(s.close())
					h.must(r.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !success || direct && failures == 0 {
				t.Fatal("named physical campaign", dual, direct, success, failures)
			}
		}
	}
}

func TestLPRationalMPSOuterPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, dual := range []bool{false, true} {
		for _, operation := range []string{"create", "named"} {
			matched := false
			for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
				h := newLPPairHarness(t)
				large := strings.Repeat("1234567890", 100) + "7/13"
				r := lpReadExactMPS(t, h, "NAME OLDER\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("f", "COST", large)+lpExactMPSPair("f", "R", "1")+"RHS\n"+lpExactMPSPair("RHS1", "R", large)+"BOUNDS\n"+lpMPSBoundLine("FX", "f "+large)+"ENDATA\n")
				var s lpMPSExactOuter
				if operation == "named" {
					var err error
					s, err = lpCreateMPSExactOuter(r, dual)
					h.must(err)
				}
				out := lpScalarBoundValue(h, "97")
				v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
				var caller exactEtaCalls
				before, err := caller.begin(h.m)
				h.must(err)
				h.budget.FailAfter(ordinal)
				_, err = h.m.Compare(&v.X, &v.Y)
				h.budget.FailAfter(-1)
				err = caller.after(h.m, before, err)
				older := h.m.ComparisonInvocationState()
				if older.Active {
					matched = true
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					if operation == "create" {
						s, err = lpCreateMPSExactOuter(r, dual)
					} else {
						err = s.named("f", out)
					}
					h.must(err)
					if r.calls.comparison != 0 || s.base().calls.comparison != 0 {
						t.Fatal("factory adopted caller comparison")
					}
					for _, c := range s.base().Columns.All() {
						if c.calls.comparison != 0 {
							t.Fatal("column adopted caller comparison")
						}
					}
					if s.p != nil && s.p.outerCalls.comparison != 0 || s.d != nil && s.d.outerCalls.comparison != 0 {
						t.Fatal("outer solver adopted caller comparison")
					}
				}
				h.must(s.close())
				h.must(r.Close())
				if older.Active && h.m.ComparisonInvocationState() != older {
					t.Fatal("factory/named cleanup consumed caller comparison")
				}
				h.must(caller.cleanupRaw(h.m))
				h.must(caller.cleanupRoots(h.m))
				lpExactVectorNamedCensus(t, h, nil)
			}
			if !matched {
				t.Fatal("no actual older comparison", dual, operation)
			}
		}
	}
}

func TestLPRationalMPSOuterFactoryDiagnosticPrefixes(t *testing.T) {
	for _, dual := range []bool{false, true} {
		done := false
		for accepted := 0; accepted < 100 && !done; accepted++ {
			h := newLPPairHarness(t)
			r := lpReadExactMPS(t, h, "NAME OUTPUT\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "R", "7/13")+"RHS\nENDATA\n")
			row, _ := r.rows.Get2("R")
			row.typ = mpsCost
			r.Messages = &lpValidationWriter{allowed: accepted, err: lpErrResidualWrite}
			s, err := lpCreateMPSExactOuter(r, dual)
			if err != lpErrResidualWrite {
				if err != ErrMPSUnhandled || r.IsOK() {
					t.Fatal("relation diagnostic terminal", dual, err)
				}
				done = true
			}
			if s.base() == nil || s.base().Constraints.Len() != 0 || s.base().Columns.Len() != 0 || s.base().AValues.Len() != 0 {
				t.Fatal("diagnostics precede model population", dual, err)
			}
			h.must(s.close())
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !done {
			t.Fatal("no diagnostic success", dual)
		}
	}
}
