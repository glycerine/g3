package nlsat

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func searchStateFixture(t *testing.T) (*Evaluator, *searchState) {
	t.Helper()
	e, s, limit := clauseStoreFixtureWithLimit(t)
	r, err := newSearchState(s, e, searchConfig{Limit: limit, MaxMemory: uint64(^uint32(0)), SimplifyCores: true, MaxConflicts: ^uint32(0)})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, r.close()) })
	return e, r
}
func searchClause(t *testing.T, r *searchState, lits ...sat.Literal) *Clause {
	t.Helper()
	c, err := r.store.mkClause(lits, false, nil)
	assignmentMust(t, err)
	return c
}
func searchInequality(t *testing.T, e *Evaluator, r *searchState, k AtomKind, terms ...evaluatorTerm) (sat.Literal, *Clause) {
	t.Helper()
	p := evaluatorPolynomial(t, e, terms...)
	b, err := r.store.atoms.mkInequality(k, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	l := sat.MakeLiteral(b, false)
	return l, searchClause(t, r, l)
}
func searchAtRealStage(t *testing.T, r *searchState) {
	t.Helper()
	assignmentMust(t, r.newStage())
	assignmentMust(t, r.newStage())
	if r.xk != 1 {
		t.Fatal("real stage", r.xk)
	}
}
func TestSearchBooleanPropagationDecisionsAndUndoTargets(t *testing.T) {
	e, r := searchStateFixture(t)
	var b [4]sat.BoolVar
	for i := range b {
		var err error
		b[i], err = r.store.atoms.mkBoolVar()
		assignmentMust(t, err)
	}
	l := func(i int, neg bool) sat.Literal { return sat.MakeLiteral(b[i], neg) }
	c1 := searchClause(t, r, l(0, true))
	c2 := searchClause(t, r, l(0, false), l(1, false))
	c3 := searchClause(t, r, l(1, true), l(2, false), l(3, false))
	c4 := searchClause(t, r, l(3, false))
	for _, c := range []*Clause{r.store.originals[0], c1, c2, c3, c4} {
		ok, err := r.processClause(c, false)
		assignmentMust(t, err)
		if !ok {
			t.Fatal("unexpected Boolean conflict")
		}
	}
	want := []lbool.LBool{lbool.True, lbool.False, lbool.True, lbool.True, lbool.True}
	if !reflect.DeepEqual(r.store.atoms.bools.values, want) || r.level != 1 || r.decisions != 1 || r.propagations != 4 {
		t.Fatal("independent Boolean transitions", r.store.atoms.bools.values, r.level, r.decisions, r.propagations)
	}
	if j := r.store.atoms.bools.justifications[b[2]]; !j.IsDecision() {
		t.Fatal("first undefined literal was not the decision")
	}
	if j := r.store.atoms.bools.justifications[b[1]]; !j.IsClause() || j.clause != c2 {
		t.Fatal("unit propagation did not borrow its clause")
	}
	// A genuinely deleted arithmetic atom leaves a dead slot to skip.
	_, dead := searchInequality(t, e, r, AtomGT, evaluatorTerm{1, 0, 1})
	assignmentMust(t, r.store.deleteClause(dead))
	assignmentMust(t, r.peekNextBooleanVariable())
	if r.bk != sat.NullBoolVar {
		t.Fatal("dead slot was selected as a pure Boolean variable", r.bk)
	}
	conflicting := searchClause(t, r, l(0, false))
	conflict, err := r.processClauses([]*Clause{c1, conflicting, c4})
	assignmentMust(t, err)
	if conflict != conflicting {
		t.Fatal("first Boolean conflict selection")
	}
	inconsistent, err := r.isInconsistent(conflicting.Literals())
	assignmentMust(t, err)
	if !inconsistent {
		t.Fatal("assigned-false unit not inconsistent")
	}
	assignmentMust(t, r.undoUntilUnassigned(b[2]))
	if r.level != 1 || r.bk != b[2] || r.store.atoms.bools.values[b[3]] != lbool.Undef || r.trail[len(r.trail)-1].kind != trailLevel {
		t.Fatal("unassigned predicate consumed the enclosing level")
	}
	assignmentMust(t, r.undoUntilLevel(0))
	if r.level != 0 || r.decisions != 1 || r.propagations != 4 {
		t.Fatal("backtracking changed cumulative statistics")
	}
	assignmentMust(t, r.undoUntilSize(0))
	if r.bk != 0 || len(r.trail) != 0 {
		t.Fatal("Boolean cursor did not rewind to earliest unassigned variable")
	}
	for i, v := range r.store.atoms.bools.values {
		if v != lbool.Undef || r.store.atoms.bools.levels[i] != ^uint32(0) || !r.store.atoms.bools.justifications[i].IsNull() {
			t.Fatal("Boolean undo field order/result", i)
		}
	}
}

func TestSearchInfeasibleAliasOwnershipAndStageUndo(t *testing.T) {
	e, r := searchStateFixture(t)
	searchAtRealStage(t, r)
	evaluatorSetFraction(t, e, 0, 3, 1)
	l, c := searchInequality(t, e, r, AtomLT, evaluatorTerm{1, 0, 1})
	j, err := NewClauseJustification(c)
	assignmentMust(t, err)
	assignmentMust(t, r.assign(l, j))
	base := uint32(len(r.trail))
	set, err := e.InfeasibleIntervals(r.store.atoms.bools.atoms[l.Variable()], false)
	assignmentMust(t, err)
	assignmentMust(t, r.updateInfeasible(set))
	assignmentMust(t, r.ism.DecRef(set))
	if set.refs != 1 || r.store.vars.infeasible[1] != set {
		t.Fatal("first domain transfer")
	}
	assignmentMust(t, r.updateInfeasible(set))
	if set.refs != 2 || r.store.vars.infeasible[1] != set {
		t.Fatal("alias union did not retain separate trail/current references", set.refs)
	}
	assignmentMust(t, r.undoUntilSize(base+1))
	if set.refs != 1 || r.store.vars.infeasible[1] != set {
		t.Fatal("alias undo consumed the restored reference")
	}
	assignmentMust(t, r.undoUntilSize(base))
	if !set.closed || r.store.vars.infeasible[1] != nil {
		t.Fatal("last domain DEC did not retire its actual owner")
	}
	assignmentMust(t, r.undoUntilStage(0))
	assigned, err := r.assignment.Contains(0)
	assignmentMust(t, err)
	if assigned || r.xk != 0 || r.stages != 2 {
		t.Fatal("stage undo did not clear prior-stage assignment flags")
	}
	if len(r.assignment.values) == 0 {
		t.Fatal("stage reset destroyed hidden owning algebraic cells")
	}
	assignmentMust(t, r.undoUntilEmpty())
	if r.xk != NullVariable || len(r.trail) != 0 {
		t.Fatal("root stage undo")
	}
}

func TestSearchEqualityReplacementUsesDegreeAndRestoresBorrowedPointers(t *testing.T) {
	e, r := searchStateFixture(t)
	searchAtRealStage(t, r)
	var literals []sat.Literal
	var atoms []ArithmeticAtom
	for _, terms := range [][]evaluatorTerm{{{1, 0, 3}}, {{1, 0, 2}}, {{1, 0, 2}, {1, 0, 1}}, {{1, 0, 1}}} {
		l, c := searchInequality(t, e, r, AtomEQ, terms...)
		literals = append(literals, l)
		atoms = append(atoms, r.store.atoms.bools.atoms[l.Variable()])
		j, err := NewClauseJustification(c)
		assignmentMust(t, err)
		assignmentMust(t, r.assign(l, j))
		want := len(atoms) - 1
		if want == 2 {
			want = 1
		}
		if r.store.vars.equalities[1] != atoms[want] {
			t.Fatal("strict smaller-degree equality selection", len(atoms))
		}
	}
	for _, a := range atoms {
		if arithmeticAtomHeader(a).refs != 1 {
			t.Fatal("borrowed equality acquired a spurious atom reference")
		}
	}
	assignmentMust(t, r.undoUntilUnassigned(literals[3].Variable()))
	if r.store.vars.equalities[1] != atoms[1] {
		t.Fatal("smallest equality did not restore previous pointer")
	}
	assignmentMust(t, r.undoUntilUnassigned(literals[1].Variable()))
	if r.store.vars.equalities[1] != atoms[0] {
		t.Fatal("equal-degree skipped update changed restoration")
	}
	assignmentMust(t, r.undoUntilUnassigned(literals[0].Variable()))
	if r.store.vars.equalities[1] != nil {
		t.Fatal("initial equality restoration")
	}
}

func TestSearchEqualityConfigurationAndExcludedKinds(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			e, r := searchStateFixture(t)
			r.config.SimplifyCores = enabled
			searchAtRealStage(t, r)
			x := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
			y := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{-1, 0, 0})
			var excluded []sat.Literal
			b, err := r.store.atoms.mkInequality(AtomEQ, []polynomial.Polynomial{x}, []bool{true})
			assignmentMust(t, err)
			excluded = append(excluded, sat.MakeLiteral(b, false))
			b, err = r.store.atoms.mkInequality(AtomEQ, []polynomial.Polynomial{x, y}, []bool{false, false})
			assignmentMust(t, err)
			excluded = append(excluded, sat.MakeLiteral(b, false))
			b, err = r.store.atoms.mkRoot(AtomRootEQ, 1, 1, x)
			assignmentMust(t, err)
			excluded = append(excluded, sat.MakeLiteral(b, false))
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{-2, 0, 0})
			b, err = r.store.atoms.mkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false})
			assignmentMust(t, err)
			excluded = append(excluded, sat.MakeLiteral(b, true))
			for _, l := range excluded {
				c := searchClause(t, r, l)
				j, err := NewClauseJustification(c)
				assignmentMust(t, err)
				assignmentMust(t, r.assign(l, j))
				if r.store.vars.equalities[1] != nil {
					t.Fatal("excluded kind/parity/multiplicity/polarity was selected", l)
				}
			}
			b, err = r.store.atoms.mkInequality(AtomEQ, []polynomial.Polynomial{x}, []bool{false})
			assignmentMust(t, err)
			l := sat.MakeLiteral(b, false)
			c := searchClause(t, r, l)
			j, err := NewClauseJustification(c)
			assignmentMust(t, err)
			assignmentMust(t, r.assign(l, j))
			if (r.store.vars.equalities[1] != nil) != enabled {
				t.Fatal("simplify-cores configuration did not control equality caching")
			}
		})
	}
}

func TestSearchArithmeticClauseBranchesAndLazyModes(t *testing.T) {
	for _, name := range []string{"tautology", "impossible", "unit", "decision", "subset", "union-full", "lazy-one", "lazy-two", "force-learned"} {
		t.Run(name, func(t *testing.T) {
			e, r := searchStateFixture(t)
			searchAtRealStage(t, r)
			if name == "tautology" || name == "impossible" {
				k := AtomGT
				if name == "impossible" {
					k = AtomLT
				}
				l, c := searchInequality(t, e, r, k, evaluatorTerm{1, 0, 2}, evaluatorTerm{1, 0, 0})
				ok, err := r.processClause(c, false)
				assignmentMust(t, err)
				if ok != (name == "tautology") || !r.store.atoms.bools.justifications[l.Variable()].IsLazy() || r.propagations != 1 || r.level != 0 || r.store.vars.infeasible[1] != nil {
					t.Fatal("constant-truth propagation branch")
				}
				value, err := r.value(l)
				assignmentMust(t, err)
				if (value == lbool.True) != (name == "tautology") {
					t.Fatal("propagated constant truth")
				}
				return
			}
			base, unit := searchInequality(t, e, r, AtomLT, evaluatorTerm{1, 0, 1})
			if name == "unit" || name == "subset" || name == "union-full" {
				ok, err := r.processClause(unit, false)
				assignmentMust(t, err)
				if !ok {
					t.Fatal("feasible arithmetic unit rejected")
				}
				if !r.store.atoms.bools.justifications[base.Variable()].IsClause() || r.level != 0 {
					t.Fatal("arithmetic unit justification")
				}
				q, err := e.am.RationalManager()
				assignmentMust(t, err)
				point := unumeral.NewRational()
				for _, x := range []int32{-2, 0, 2} {
					assignmentMust(t, q.SetInt32(&point, x))
					count, _ := intervalActualAt(t, r.ism, r.store.vars.infeasible[1], &point)
					if (count != 0) != (x >= 0) {
						t.Fatal("independent infeasible domain", x)
					}
				}
				assignmentMust(t, q.Del(&point))
				if name == "unit" {
					return
				}
				k := AtomLT
				bound := int32(-1)
				if name == "union-full" {
					k = AtomGT
					bound = 0
				}
				l, c := searchInequality(t, e, r, k, evaluatorTerm{1, 0, 1}, evaluatorTerm{bound, 0, 0})
				ok, err = r.processClause(c, false)
				assignmentMust(t, err)
				if ok != (name == "subset") {
					t.Fatal("domain implication/conflict branch", ok)
				}
				j := r.store.atoms.bools.justifications[l.Variable()]
				if !j.IsLazy() {
					t.Fatal("domain inference did not retain lazy reason")
				}
				why, err := j.lazy.Literals()
				assignmentMust(t, err)
				if len(why) != 2 {
					t.Fatal("domain inference duplicated or dropped justification literals", why)
				}
				return
			}
			other, _ := searchInequality(t, e, r, AtomGT, evaluatorTerm{1, 0, 1}, evaluatorTerm{-1, 0, 0})
			learned := name != "decision"
			c, err := r.store.mkClause([]sat.Literal{other, base}, learned, nil)
			assignmentMust(t, err)
			if name == "lazy-one" {
				r.config.Lazy = 1
			} else if name == "lazy-two" || name == "force-learned" {
				r.config.Lazy = 2
			}
			ok, err := r.processClause(c, name == "force-learned")
			assignmentMust(t, err)
			if !ok {
				t.Fatal("feasible non-unit rejected")
			}
			decision := name == "decision" || name == "force-learned"
			if (r.level == 1) != decision || (r.store.vars.infeasible[1] != nil) != decision || (r.decisions == 1) != decision {
				t.Fatal("lazy/forced non-unit decision", r.level, r.decisions)
			}
		})
	}
}
