package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// smtArithTestHost records what the engine asks of the context.
type smtArithTestHost struct {
	assignments map[SMTBoolVar]lbool.LBool
	conflicts   [][]SMTLiteral
	lemmas      [][]SMTLiteral
	assigned    []SMTLiteral
	eqs         [][2]SMTTheoryVar
	equal       map[[2]SMTTheoryVar]bool
	ints        map[SMTTheoryVar]bool
}

func (h *smtArithTestHost) assignment(b SMTBoolVar) lbool.LBool {
	if v, ok := h.assignments[b]; ok {
		return v
	}
	return lbool.Undef
}
func (h *smtArithTestHost) setConflict(a *smtArithmeticAntecedents, _ string) error {
	h.conflicts = append(h.conflicts, append([]SMTLiteral(nil), a.literals...))
	return nil
}
func (h *smtArithTestHost) assignBoundLiteral(l SMTLiteral, _ *smtArithmeticAntecedents) error {
	h.assigned = append(h.assigned, l)
	return nil
}
func (h *smtArithTestHost) mkAuxLemma(l []SMTLiteral, _ *smtArithmeticAntecedents) error {
	h.lemmas = append(h.lemmas, append([]SMTLiteral(nil), l...))
	return nil
}
func (h *smtArithTestHost) isIntSource(v SMTTheoryVar) bool { return h.ints[v] }
func (h *smtArithTestHost) isEqual(x, y SMTTheoryVar) bool {
	return h.equal[[2]SMTTheoryVar{x, y}] || h.equal[[2]SMTTheoryVar{y, x}]
}
func (h *smtArithTestHost) propagateEqToCore(x, y SMTTheoryVar, _ *smtArithmeticAntecedents) error {
	h.eqs = append(h.eqs, [2]SMTTheoryVar{x, y})
	return nil
}
func (h *smtArithTestHost) limitInc(uint) bool { return true }
func (h *smtArithTestHost) cancelled() bool    { return false }

// smtArithTestState holds the Z3 defaults of the parameters the engine reads.
func smtArithTestState() SMTArithState {
	return SMTArithState{BlandsRuleThreshold: 1000, PropagateEqs: true, BoundProp: 2, StrongerLemmas: true,
		SkipRowsWithBigCoeffs: true, MaxLemmaSize: 128, SmallLemmaSize: 16, PropagationThreshold: 4294967295,
		RandomLower: -1000, RandomUpper: 1000}
}

func newSMTArithTestEngine(t *testing.T, integer bool) (*smtArithEngine, *smtArithTestHost) {
	t.Helper()
	host := &smtArithTestHost{assignments: map[SMTBoolVar]lbool.LBool{}, equal: map[[2]SMTTheoryVar]bool{}, ints: map[SMTTheoryVar]bool{}}
	params := smtArithTestState()
	e, err := newSMTArithEngine(host, &params, integer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.close() })
	return e, host
}

func smtArithTestVar(t *testing.T, e *smtArithEngine, isInt bool) SMTTheoryVar {
	t.Helper()
	v, err := e.addVariable(isInt, false)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// smtArithTestDefine makes a row  s = sum(coeffs[i]*vars[i])  and returns s.
// The owner has coefficient 1 and is the last entry, as init_row requires.
func smtArithTestDefine(t *testing.T, e *smtArithEngine, isInt bool, coeffs []int64, vars []SMTTheoryVar) SMTTheoryVar {
	t.Helper()
	s := smtArithTestVar(t, e, isInt)
	rowID := e.mkRow()
	for i, v := range vars {
		idx, entry := e.rows[rowID].addEntry()
		_ = idx
		entry.variable = v
		entry.coefficient.SetInt64(-coeffs[i])
		ci, ce := e.columns[v].addEntry()
		entry.columnIndex = ci
		ce.rowID, ce.rowIndex = rowID, idx
	}
	idx, entry := e.rows[rowID].addEntry()
	entry.variable = s
	entry.coefficient.SetInt64(1)
	ci, ce := e.columns[s].addEntry()
	entry.columnIndex = ci
	ce.rowID, ce.rowIndex = rowID, idx
	if err := e.initRow(rowID); err != nil {
		t.Fatal(err)
	}
	return s
}

// smtArithTestBound asserts v >= k or v <= k with a literal-backed atom.
func smtArithTestBound(t *testing.T, e *smtArithEngine, v SMTTheoryVar, bv SMTBoolVar, upper bool, k int64) (bool, error) {
	t.Helper()
	kind := smtArithmeticLowerAtom
	if upper {
		kind = smtArithmeticUpperAtom
	}
	var th smtArithmeticValue
	th.rational.SetInt64(k)
	a, err := newSMTArithmeticAtom(bv, v, &th, kind)
	if err != nil {
		t.Fatal(err)
	}
	e.atoms = append(e.atoms, a)
	e.varOccs[v] = append(e.varOccs[v], a)
	e.boolVarToAtom[bv] = a
	e.unassignedAtoms[v]++
	if err = a.assign(true, e.epsilon(v)); err != nil {
		t.Fatal(err)
	}
	return e.assertBound(a)
}

func smtArithTestValue(e *smtArithEngine, v SMTTheoryVar) string {
	return e.getValue(v).rational.RatString()
}

func TestSMTArithEngineFeasibleAssignment(t *testing.T) {
	e, host := newSMTArithTestEngine(t, false)
	x := smtArithTestVar(t, e, false)
	y := smtArithTestVar(t, e, false)
	s := smtArithTestDefine(t, e, false, []int64{1, 1}, []SMTTheoryVar{x, y}) // s = x + y
	for i, c := range []struct {
		v     SMTTheoryVar
		upper bool
		k     int64
	}{{x, false, 3}, {y, false, 4}, {s, true, 10}} {
		ok, err := smtArithTestBound(t, e, c.v, SMTBoolVar(1+i), c.upper, c.k)
		if err != nil || !ok {
			t.Fatal("assert", i, ok, err)
		}
	}
	ok, err := e.makeFeasible()
	if err != nil || !ok || len(host.conflicts) != 0 {
		t.Fatal("feasible expected", ok, err, host.conflicts)
	}
	if got := smtArithTestValue(e, s); got != "7" {
		t.Fatalf("s = x + y with x>=3,y>=4: got %s", got)
	}
}

func TestSMTArithEngineRowConflictNamesBothBounds(t *testing.T) {
	e, host := newSMTArithTestEngine(t, false)
	x := smtArithTestVar(t, e, false)
	y := smtArithTestVar(t, e, false)
	s := smtArithTestDefine(t, e, false, []int64{1, 1}, []SMTTheoryVar{x, y})
	for i, c := range []struct {
		v     SMTTheoryVar
		upper bool
		k     int64
	}{{x, false, 3}, {y, false, 4}, {s, true, 6}} {
		if ok, err := smtArithTestBound(t, e, c.v, SMTBoolVar(1+i), c.upper, c.k); err != nil || !ok {
			t.Fatal("assert", i, ok, err)
		}
	}
	ok, err := e.makeFeasible()
	if err != nil || ok {
		t.Fatal("conflict expected", ok, err)
	}
	if len(host.conflicts) != 1 || len(host.conflicts[0]) != 3 {
		t.Fatalf("Farkas conflict must use all three bounds, got %v", host.conflicts)
	}
}

func TestSMTArithEngineBoundConflictAndRedundancy(t *testing.T) {
	e, host := newSMTArithTestEngine(t, false)
	x := smtArithTestVar(t, e, false)
	if ok, err := smtArithTestBound(t, e, x, 1, false, 5); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := smtArithTestBound(t, e, x, 2, false, 3); err != nil || !ok {
		t.Fatal("weaker lower bound is redundant, not a conflict", ok, err)
	}
	if e.lower(x).core().value.rational.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("redundant bound replaced the stronger one")
	}
	ok, err := smtArithTestBound(t, e, x, 3, true, 4)
	if err != nil || ok || len(host.conflicts) != 1 || len(host.conflicts[0]) != 2 {
		t.Fatal("x>=5, x<=4 must conflict on both atoms", ok, err, host.conflicts)
	}
}

func TestSMTArithEnginePopRestoresBoundsAndAssignment(t *testing.T) {
	e, _ := newSMTArithTestEngine(t, false)
	x := smtArithTestVar(t, e, false)
	y := smtArithTestVar(t, e, false)
	s := smtArithTestDefine(t, e, false, []int64{1, 1}, []SMTTheoryVar{x, y})
	e.pushScope()
	if ok, err := smtArithTestBound(t, e, x, 1, false, 3); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := e.makeFeasible(); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if e.lower(x) == nil {
		t.Fatal("bound missing in scope")
	}
	e.discardUpdateTrail()
	if err := e.popScopes(1); err != nil {
		t.Fatal(err)
	}
	if e.lower(x) != nil || e.unassignedAtoms[x] != 1 || len(e.atoms) != 0 || len(e.varOccs[x]) != 0 {
		t.Fatal("pop must remove the bound and its atom and restore the unassigned count")
	}
	if len(e.boundTrail) != 0 || len(e.unassignedTrail) != 0 {
		t.Fatal("trails must be unwound")
	}
	_ = s
}

func TestSMTArithEngineFixedVarEqualityPropagation(t *testing.T) {
	e, host := newSMTArithTestEngine(t, false)
	x := smtArithTestVar(t, e, false)
	y := smtArithTestVar(t, e, false)
	for i, v := range []SMTTheoryVar{x, y} {
		if ok, err := smtArithTestBound(t, e, v, SMTBoolVar(1+2*i), false, 7); err != nil || !ok {
			t.Fatal(ok, err)
		}
		if ok, err := smtArithTestBound(t, e, v, SMTBoolVar(2+2*i), true, 7); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	// fixed_var_eh ran from set_bound as each variable became fixed; the
	// second variable fixed at the same value is equated with the first.
	if len(host.eqs) != 1 || host.eqs[0] != [2]SMTTheoryVar{y, x} || e.stats.fixedEqs != 1 {
		t.Fatalf("two variables fixed at 7 must be equated, got %v", host.eqs)
	}
	// Different sort: no equality.
	host.eqs = nil
	z := smtArithTestVar(t, e, true)
	host.ints[z] = true
	for i, up := range []bool{false, true} {
		if ok, err := smtArithTestBound(t, e, z, SMTBoolVar(10+i), up, 7); err != nil || !ok {
			t.Fatal(ok, err)
		}
	}
	if len(host.eqs) != 0 {
		t.Fatal("Int and Real variables must not be equated")
	}
}
