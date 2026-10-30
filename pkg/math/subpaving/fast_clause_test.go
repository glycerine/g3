package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSubpavingFastClauseStableWatchUnitAndConflict(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false, false})
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	atom := func(variable uint32, value int32, lower, open bool) *FastInequality {
		t.Helper()
		must(m.SetInt32(v, value))
		a, e := c.NewInequality(variable, v, lower, open)
		must(e)
		return a
	}
	yUpper := atom(1, 2, false, false)
	xLower := atom(0, 5, true, false)
	xUpper := atom(0, 9, false, true)
	clause, err := c.AddClause([]*FastInequality{yUpper, xLower, xUpper}, false, true)
	must(err)
	if clause == nil || len(clause.atoms) != 3 || clause.atoms[0] != xLower || clause.atoms[1] != xUpper || clause.atoms[2] != yUpper ||
		len(c.watchOrder[0]) != 1 || len(c.watchOrder[1]) != 1 || xLower.refs != 1 || xUpper.refs != 1 || yUpper.refs != 1 {
		t.Fatal("stable clause order or unique watchers", clause, c.watchOrder)
	}
	// The source atom owns a copy: changing the caller's numeral cannot change
	// the clause's propagation value.
	fastMonomialBound(t, m, c, root, v, 0, 4, false)
	if err := c.PropagateQueued(root); err != nil {
		t.Fatal(err)
	}
	// x <= 4 makes x >= 5 false, but x < 9 is already true, so no y bound.
	if root.upper[1] != nil {
		t.Fatal("satisfied clause propagated")
	}
	if clause.visited == 0 || c.numVisited == 0 {
		t.Fatal("clause watcher not visited")
	}

	m2, c2, v2, root2 := fastMonomialFixture(t, []bool{false, false})
	must(m2.SetInt32(v2, 5))
	a0, err := c2.NewInequality(0, v2, true, false)
	must(err)
	must(m2.SetInt32(v2, 2))
	a1, err := c2.NewInequality(1, v2, false, false)
	must(err)
	unit, err := c2.AddClause([]*FastInequality{a0, a1}, false, true)
	must(err)
	fastMonomialBound(t, m2, c2, root2, v2, 0, 4, false)
	if err := c2.PropagateQueued(root2); err != nil {
		t.Fatal(err)
	}
	fastMonomialCheckBound(t, m2, root2, v2, 1, 2, false)
	if root2.inconsistent || unit.visited == 0 {
		t.Fatal("unit propagation", root2.inconsistent, unit.visited)
	}
	if reason := root2.upper[1].reason; reason.kind != fastReasonClause || reason.clause != unit {
		t.Fatal("borrowed clause justification", reason)
	}
	// With both atoms false, source chooses the first atom and publishes a
	// contradictory bound to mark the node inconsistent.
	m3, c3, v3, root3 := fastMonomialFixture(t, []bool{false, false})
	fastMonomialBound(t, m3, c3, root3, v3, 0, 4, false)
	fastMonomialBound(t, m3, c3, root3, v3, 1, 3, true)
	must(m3.SetInt32(v3, 5))
	b0, err := c3.NewInequality(0, v3, true, false)
	must(err)
	must(m3.SetInt32(v3, 2))
	b1, err := c3.NewInequality(1, v3, false, false)
	must(err)
	conflictClause, err := c3.AddClause([]*FastInequality{b1, b0}, false, true)
	must(err)
	if err := c3.PropagateClause(conflictClause, root3); err != nil {
		t.Fatal(err)
	}
	if !root3.inconsistent || root3.conflictVar != 0 {
		t.Fatal("source first-atom conflict", root3.conflictVar)
	}
	if reason := root3.lower[0].reason; reason.kind != fastReasonClause || reason.clause != conflictClause {
		t.Fatal("conflicting clause justification", reason)
	}
}

func TestSubpavingFastInequalityConstructionAfterChildCreation(t *testing.T) {
	m, c, value, root := fastMonomialFixture(t, []bool{false})
	child, err := c.NewChild(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := m.SetInt32(value, 7); err != nil {
		t.Fatal(err)
	}
	atom, err := c.NewInequality(0, value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if atom.variable != 0 || atom.refs != 0 || len(c.inequalities) != 1 || c.root.children != 1 || child.parent != root {
		t.Fatal("source mk_ineq must allocate independently of the existing child", atom, c.root.children)
	}
	if err := c.AddUnitClause(atom, false); err != nil {
		t.Fatal("source add_unit_clause after child creation", err)
	}
	clause, err := c.AddClause([]*FastInequality{atom, atom}, true, true)
	if err != nil || clause == nil || atom.refs != 3 || len(c.watchOrder[0]) != 1 {
		t.Fatal("source add_clause after child creation", clause, atom.refs, err)
	}
}

func TestSubpavingFastClauseOpennessAndUnitResume(t *testing.T) {
	for _, tc := range []struct {
		lowerAtom, atomOpen, boundLower, boundOpen bool
		want                                       fastTruth
	}{
		{true, false, false, false, fastUnknown},
		{true, false, false, true, fastFalse},
		{true, true, true, false, fastUnknown},
		{true, true, true, true, fastTrue},
		{false, false, true, true, fastFalse},
		{false, true, false, true, fastTrue},
	} {
		t.Run("truth", func(t *testing.T) {
			m, c, v, root := fastMonomialFixture(t, []bool{false})
			if err := m.SetInt32(v, 5); err != nil {
				t.Fatal(err)
			}
			a, err := c.NewInequality(0, v, tc.lowerAtom, tc.atomOpen)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.PropagateCandidate(root, 0, v, tc.boundLower, tc.boundOpen); err != nil {
				t.Fatal(err)
			}
			got, err := c.inequalityValue(a, root)
			if err != nil || got != tc.want {
				t.Fatalf("truth %+v: %v, %v", tc, got, err)
			}
		})
	}
	m, c, v, root := fastMonomialFixture(t, []bool{false, false})
	if err := m.SetInt32(v, 1); err != nil {
		t.Fatal(err)
	}
	a0, err := c.NewInequality(0, v, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetInt32(v, 2); err != nil {
		t.Fatal(err)
	}
	a1, err := c.NewInequality(1, v, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if clause, err := c.AddClause([]*FastInequality{a0}, true, false); err != nil || clause != nil {
		t.Fatal("source singleton clause", clause, err)
	}
	if err := c.AddUnitClause(a1, false); err != nil {
		t.Fatal(err)
	}
	if len(c.unitClauses) != 2 || !c.unitClauses[0].axiom || c.unitClauses[1].axiom {
		t.Fatal("unit axiom tags")
	}
	c.limit.Limit, c.limit.Steps = 1, 0
	if err := c.AssertUnitClauses(root); !errors.Is(err, ErrFastContextCanceled) || c.unitHead != 1 {
		t.Fatal("unit checkpoint suffix", c.unitHead, err)
	}
	fastMonomialCheckBound(t, m, root, v, 0, 1, true)
	if reason := root.lower[0].reason; reason.kind != fastReasonAxiom || reason.clause != nil {
		t.Fatal("unit axiom justification", reason)
	}
	if root.upper[1] != nil {
		t.Fatal("canceled unit published")
	}
	c.limit.Limit, c.limit.Steps = 10, 0
	if err := c.AssertUnitClauses(root); err != nil {
		t.Fatal(err)
	}
	fastMonomialCheckBound(t, m, root, v, 1, 2, false)
	if reason := root.upper[1].reason; reason.kind != fastReasonAssumption || reason.clause != nil {
		t.Fatal("unit assumption justification", reason)
	}
	if c.unitHead != 2 {
		t.Fatal("unit suffix not consumed", c.unitHead)
	}
	if _, err := c.AddClause([]*FastInequality{nil, a1}, false, true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("nil atom", err)
	}
}

func TestSubpavingFastClauseSharedAtomSourceCleanup(t *testing.T) {
	m, c, v, root := fastMonomialFixture(t, []bool{false, false, false})
	makeAtom := func(variable uint32, value int32) *FastInequality {
		t.Helper()
		if err := m.SetInt32(v, value); err != nil {
			t.Fatal(err)
		}
		a, err := c.NewInequality(variable, v, true, false)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	shared, first, second := makeAtom(0, 1), makeAtom(1, 2), makeAtom(2, 3)
	if err := c.AddUnitClause(shared, true); err != nil {
		t.Fatal(err)
	}
	problem, err := c.AddClause([]*FastInequality{first, shared}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	lemma, err := c.AddClause([]*FastInequality{second, shared}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if shared.refs != 3 || first.refs != 1 || second.refs != 1 || len(c.watchOrder[0]) != 2 {
		t.Fatal("source shared reference count and watch registration", shared.refs, first.refs, second.refs)
	}
	if err := c.releaseUnit(0); err != nil {
		t.Fatal(err)
	}
	if err := c.releaseClause(problem); err != nil {
		t.Fatal(err)
	}
	if shared.refs != 1 || first.refs != 0 || len(c.watchOrder[0]) != 1 || len(c.watchOrder[1]) != 0 {
		t.Fatal("problem clause cleanup", shared.refs, first.refs, c.watchOrder)
	}
	if _, err := first.owner.Get(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("unreferenced problem atom stayed live", err)
	}
	if err := c.PropagateClause(problem, root); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("deleted problem clause propagated", err)
	}
	if err := c.releaseClause(lemma); err != nil {
		t.Fatal(err)
	}
	if shared.refs != 0 || second.refs != 0 || len(c.watchOrder[0]) != 0 || len(c.watchOrder[2]) != 0 {
		t.Fatal("lemma cleanup", shared.refs, second.refs, c.watchOrder)
	}
	if _, err := shared.owner.Get(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("shared atom stayed live", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
