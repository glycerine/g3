package smtcore

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtIncrementalRowFixture struct {
	m      *ast.ASTManager
	s      *SMTInternalizerState
	th     *SMTLinearArithmeticTheory
	family ast.FamilyID
	x, y   ast.ASTHandle
	sort   ast.ASTHandle
}

func newSMTIncrementalRowFixture(t *testing.T, prefix string) *smtIncrementalRowFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	sort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, prefix+"-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, prefix+"-y"), sort)
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	th, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(th))
	f := &smtIncrementalRowFixture{m: m, s: s, th: th, family: family, x: x, y: y, sort: sort}
	// Both variables exist before any scope, so later atoms add no variables.
	sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(sum, false))
	f.feasible(t)
	return f
}

func (f *smtIncrementalRowFixture) feasible(t *testing.T) {
	t.Helper()
	feasible, err := f.th.rebuild()
	if err != nil || !feasible {
		t.Fatal("rebuild", feasible, err)
	}
}

// atom builds and internalizes left+k*y >= bound, a two-term row atom.
func (f *smtIncrementalRowFixture) atom(t *testing.T, k, bound string) ast.ASTHandle {
	t.Helper()
	coefficient, err := f.m.ArithmeticRealNumeralText(k)
	astTestMust(t, err)
	limit, err := f.m.ArithmeticRealNumeralText(bound)
	astTestMust(t, err)
	product, err := f.m.MkFamilyApp(f.family, smtArithOpMul, nil, []ast.ASTHandle{coefficient, f.y})
	astTestMust(t, err)
	sum, err := f.m.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{f.x, product})
	astTestMust(t, err)
	atom, err := f.m.MkFamilyApp(f.family, smtArithOpGE, nil, []ast.ASTHandle{sum, limit})
	astTestMust(t, err)
	astTestMust(t, f.s.Internalize(atom, true))
	return atom
}

func (f *smtIncrementalRowFixture) assign(t *testing.T, atom ast.ASTHandle, truth bool) {
	t.Helper()
	variable, found := f.s.BoolVar(atom)
	if !found {
		t.Fatal("atom has no variable")
	}
	astTestMust(t, f.th.Assign(variable, truth))
	astTestMust(t, f.th.Propagate())
}

// A new atom must extend the live tableau rather than replace it.
func TestArithmeticNewAtomKeepsTheLiveTableau(t *testing.T) {
	f := newSMTIncrementalRowFixture(t, "keep-new-atom")
	solver := f.th.solver
	if f.th.incrementalSolver != solver {
		t.Fatal("fixture is not on the incremental path")
	}
	epoch := f.th.incrementalRowEpoch
	atom := f.atom(t, "3", "2")
	f.feasible(t)
	if f.th.solver != solver {
		t.Fatal("a new atom rebuilt the tableau")
	}
	if f.th.incrementalAtoms != len(f.th.atoms) || len(f.th.incrementalBases) != len(f.th.atoms) || len(f.th.incrementalAtomKeys) != len(f.th.atoms) {
		t.Fatal("incremental atom bookkeeping", f.th.incrementalAtoms, len(f.th.atoms))
	}
	if f.th.incrementalRowEpoch == epoch {
		t.Fatal("a new row did not invalidate the propagation index")
	}
	f.assign(t, atom, true)
	if f.th.solver != solver {
		t.Fatal("asserting the atom rebuilt the tableau")
	}
}

// Atoms with one coefficient vector share a row, and the bases must agree with
// the replay computation.
func TestArithmeticSharedRowAtomsAgreeWithReplayBases(t *testing.T) {
	f := newSMTIncrementalRowFixture(t, "shared-row")
	f.atom(t, "2", "1")
	f.atom(t, "2", "5")
	f.atom(t, "7", "5")
	f.feasible(t)
	want := f.th.arithmeticAtomBases()
	if fmt.Sprint(want) != fmt.Sprint(f.th.incrementalBases) {
		t.Fatalf("incremental bases %v, replay bases %v", f.th.incrementalBases, want)
	}
}

// A pop must drop the scope's rows and bounds on the same solver, with values
// still within the surviving bounds.
func TestArithmeticPopRestoresBoundsAndRowsInPlace(t *testing.T) {
	f := newSMTIncrementalRowFixture(t, "pop-in-place")
	base := f.atom(t, "1", "0")
	f.feasible(t)
	f.assign(t, base, true)
	solver := f.th.solver
	if f.th.incrementalSolver != solver {
		t.Fatal("fixture is not on the incremental path")
	}
	rowsBefore := len(solver.BasicRows())
	atomsBefore, keysBefore := len(f.th.atoms), len(f.th.incrementalKeys)

	astTestMust(t, f.s.PushScope())
	scoped := f.atom(t, "5", "100")
	f.feasible(t)
	f.assign(t, scoped, true)
	if len(solver.BasicRows()) != rowsBefore+1 || f.th.solver != solver {
		t.Fatal("scoped atom should add one row to the same solver", len(solver.BasicRows()), rowsBefore)
	}
	astTestMust(t, f.s.PopScopes(1))

	if f.th.solver != solver || f.th.incrementalSolver != solver {
		t.Fatal("pop replaced the tableau")
	}
	if len(solver.BasicRows()) != rowsBefore || len(f.th.atoms) != atomsBefore || len(f.th.incrementalKeys) != keysBefore {
		t.Fatal("pop left scoped rows or keys", len(solver.BasicRows()), rowsBefore, len(f.th.incrementalKeys), keysBefore)
	}
	if len(f.th.incrementalBases) != atomsBefore || len(f.th.incrementalAtomKeys) != atomsBefore || f.th.incrementalAtoms != atomsBefore {
		t.Fatal("pop left scoped atom bookkeeping")
	}
	// Only the surviving bound remains on its column.
	column := f.th.incrementalBases[atomsBefore-1]
	if _, found := solver.Lower(column); !found {
		t.Fatal("surviving lower bound lost")
	}
	value := solver.Value(column)
	lower, _ := solver.Lower(column)
	if value.Cmp(&lower) < 0 {
		t.Fatal("value below the surviving bound")
	}
	one := simplex.RationalInfinitesimal(big.NewRat(1, 1))
	zero := solver.Value(0)
	if zero.Cmp(&one) != 0 {
		t.Fatal("constant column lost its fixed value")
	}

	// The popped column can host a different atom afterward.
	again := f.atom(t, "9", "1")
	f.feasible(t)
	f.assign(t, again, true)
	if f.th.solver != solver {
		t.Fatal("re-adding an atom after a pop rebuilt the tableau")
	}
}

// When restoring is impossible, the pop must fall back to a replay.
func TestArithmeticPopFallsBackWhenVariablesWerePopped(t *testing.T) {
	f := newSMTIncrementalRowFixture(t, "pop-fallback")
	astTestMust(t, f.s.PushScope())
	z, err := f.m.MkConst(astTestSymbol(t, f.m, "pop-fallback-z"), f.sort)
	astTestMust(t, err)
	sum, err := f.m.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{z, f.x})
	astTestMust(t, err)
	astTestMust(t, f.s.Internalize(sum, false))
	f.feasible(t)
	astTestMust(t, f.s.PopScopes(1))
	if f.th.solver == nil {
		t.Fatal("no tableau after pop")
	}
	if f.th.replayedNextSimplex != f.th.nextSimplex {
		t.Fatal("replay does not cover the popped variables", f.th.replayedNextSimplex, f.th.nextSimplex)
	}
	f.feasible(t)
}

// Random scopes must agree with a from-scratch replay on feasibility, on every
// bound, and on the surviving rows' values satisfying their definitions.
func TestArithmeticIncrementalStateMatchesFullReplay(t *testing.T) {
	for seed := int64(0); seed < 25; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			f := newSMTIncrementalRowFixture(t, fmt.Sprintf("replay-%d", seed))
			depth := 0
			for step := 0; step < 40; step++ {
				switch action := r.Intn(6); {
				case action == 0:
					astTestMust(t, f.s.PushScope())
					depth++
				case action == 1 && depth > 0:
					astTestMust(t, f.s.PopScopes(1))
					depth--
				default:
					atom := f.atom(t, fmt.Sprint(1+r.Intn(4)), fmt.Sprint(-20+r.Intn(10)))
					variable, _ := f.s.BoolVar(atom)
					asserted := false
					for _, assertion := range f.th.assertions {
						asserted = asserted || f.th.atoms[assertion.atom].variable == variable
					}
					if !asserted {
						f.assign(t, atom, true)
					}
				}
				feasible, err := f.th.rebuild()
				astTestMust(t, err)
				replay, status, err := f.th.buildSimplex()
				astTestMust(t, err)
				if feasible != (status == lbool.True) {
					t.Fatalf("step %d: incremental feasible %v, replay %v", step, feasible, status)
				}
				if !feasible {
					return
				}
				bases := f.th.arithmeticAtomBases()
				for index := range f.th.atoms {
					column := bases[index]
					liveLower, liveHas := f.th.solver.Lower(column)
					wantLower, wantHas := replay.Lower(column)
					if liveHas != wantHas || liveHas && liveLower.Cmp(&wantLower) != 0 {
						t.Fatalf("step %d atom %d: lower bound differs", step, index)
					}
					liveUpper, liveHasUpper := f.th.solver.Upper(column)
					wantUpper, wantHasUpper := replay.Upper(column)
					if liveHasUpper != wantHasUpper || liveHasUpper && liveUpper.Cmp(&wantUpper) != 0 {
						t.Fatalf("step %d atom %d: upper bound differs", step, index)
					}
				}
			}
		})
	}
}
