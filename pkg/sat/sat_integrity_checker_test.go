package sat

// Source: src/sat/sat_integrity_checker.cpp and .h; src/sat/sat_solver.cpp:2845–2850.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSATIntegrityCompleteSourceMethodsAndResourceOrder(t *testing.T) {
	g, _ := satGCFixture(t, 4)
	l := g.lifecycle()
	checker, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	for _, check := range []func() (bool, error){checker.checkClauses, checker.checkLearnedClauses, checker.checkAssignment, checker.checkBoolVars, checker.checkWatches, checker.checkReinitStack, checker.checkDisjointClauses, checker.run} {
		yes, e := check()
		satOK(t, e)
		if !yes {
			t.Fatal("valid actual source graph")
		}
	}
	satOK(t, l.bindIntegrity(g))
	before := l.propagation.limit.Steps
	satOK(t, l.checkInvariant())
	if l.propagation.limit.Steps != before+1 {
		t.Fatal("resource inc precedes checker in BOTH profiles")
	}
	l.propagation.limit.SetCancel(1)
	before = l.propagation.limit.Steps
	satOK(t, l.checkInvariant())
	if l.propagation.limit.Steps != before+1 {
		t.Fatal("canceled resource still increments")
	}
	l.propagation.limit.ResetCancel()
}
func TestSATIntegrityOriginalFrozenCountSourceQuirk(t *testing.T) {
	g, _ := satGCFixture(t, 4)
	l := g.lifecycle()
	clause := satGCClause(t, g, []Literal{0, 2, 4, 6}, 1)
	satOK(t, l.detachClause(clause))
	satOK(t, l.propagation.clauses.Freeze(clause))
	g.numFrozen = 1
	c, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	yes, e := c.checkLearnedClauses()
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal("source counts frozen ORIGINAL, not learned", yes, e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
}

func TestSATIntegrityClauseVerifyAndErasedVectorCall(t *testing.T) {
	g, _ := satGCFixture(t, 4)
	l := g.lifecycle()
	clause := satCleanerClause(t, l, []Literal{0, 2, 4}, false, false)
	satOK(t, l.attachClause(clause))
	c, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	yes, e := c.checkClause(clause)
	satOK(t, e)
	if !yes {
		t.Fatal("real ternary watches")
	}
	w, e := l.propagation.variables.watches.Get(1)
	satOK(t, e)
	satOK(t, w.SetEnd(0))
	if _, e = c.checkClause(clause); !errors.Is(e, ErrSATAssertion) {
		t.Fatal("VERIFY executes BOTH profiles", e)
	}
	yes, e = c.checkClauses()
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
	satGCAudit(t, g)
}
func TestSATIntegrityDisjointComparesIDsNotPointers(t *testing.T) {
	g, _ := satGCFixture(t, 4)
	l := g.lifecycle()
	a := l.propagation.clauses
	original := satCleanerClause(t, l, []Literal{0, 2, 4}, false, false)
	learned := satGCClause(t, g, []Literal{0, 2, 4, 6}, 1)
	id, e := a.ID(original)
	satOK(t, e)
	old, e := a.ID(learned)
	satOK(t, e)
	satOK(t, a.writeWord(learned, 0, id))
	c, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	yes, e := c.checkDisjointClauses()
	satOK(t, e)
	if yes {
		t.Fatal("distinct actual clause objects share ID")
	}
	satGCAudit(t, g)
	satOK(t, a.writeWord(learned, 0, old))
	yes, e = c.checkDisjointClauses()
	satOK(t, e)
	if !yes {
		t.Fatal("restored independent IDs")
	}
}
func TestSATIntegrityWatchesAndBoolFieldsReleaseReadMask(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	v := l.propagation.variables
	c, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	// Break only the binary counterpart invariant; no owning root is removed.
	watch, e := newSATBinaryWatch(2, false)
	satOK(t, e)
	list, e := v.watches.Get(1)
	satOK(t, e)
	satOK(t, list.Push(watch))
	yes, e := c.checkWatches()
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
	satOK(t, list.SetEnd(0))
	// Native flag lengths are checked only in the source debug profile.
	prior := v.decision
	v.decision = nil
	yes, e = c.checkBoolVars()
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal("debug flag length assertion", e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
	v.decision = prior
	satGCAudit(t, g)
}

func TestSATIntegrityActualEnabledConsumerBindings(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	p := l.propagation
	before := p.limit.Steps
	probing := g.conflict.dynamic.probing
	debug, _, _ := satIntegrityActionFixture(t, "T")
	satOK(t, debug.EnableDebug("sat_propagate"))
	satOK(t, debug.EnableDebug("sat_missed_prop"))
	satOK(t, debug.EnableDebug("sat_restart"))
	satOK(t, probing.propagate())
	satOK(t, g.conditionalInvariant("sat_restart"))
	satOK(t, l.checkInvariant())
	if satStorageSourceDebug {
		if p.limit.Steps != before+3 {
			t.Fatal("three reached check_invariant calls", before, p.limit.Steps)
		}
	} else if p.limit.Steps != before+1 {
		t.Fatal("explicit checker call reached in release")
	}
	original := satCleanerClause(t, l, []Literal{0, 2, 4}, false, false)
	satOK(t, l.attachClause(original))
	satOK(t, l.checkMissedPropagation())
	satOK(t, p.assign(1, satJustification{}))
	satOK(t, p.assign(3, satJustification{}))
	if e := l.checkMissedPropagation(); satStorageSourceDebug {
		var ex z3exception.DefaultException
		if !errors.As(e, &ex) {
			t.Fatal("real missed unit reached source UNREACHABLE", e)
		}
	} else if !errors.Is(e, ErrSATAssertion) {
		t.Fatal("real missed unit release guard", e)
	}
	satGCAudit(t, g)
}

func TestSATPropagateIndependentProcessAssertionTags(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	p := l.propagation
	debug, _, _ := satIntegrityActionFixture(t, "T")
	before := p.limit.Steps
	_, e := l.propagate(false)
	satOK(t, e)
	if p.limit.Steps != before {
		t.Fatal("disabled sat_propagate tag ran the invariant checker")
	}
	satOK(t, debug.EnableDebug("sat_propagate"))
	_, e = l.propagate(false)
	satOK(t, e)
	want := before
	if satStorageSourceDebug {
		want++
	}
	if p.limit.Steps != want {
		t.Fatal("sat_propagate tag did not independently gate the checker", p.limit.Steps, want)
	}
	satOK(t, debug.DisableDebug("sat_propagate"))
	satCleanerClause(t, l, []Literal{0, 2, 4}, false, false)
	satOK(t, p.assign(1, satJustification{}))
	satOK(t, p.assign(3, satJustification{}))
	_, e = l.propagate(false)
	satOK(t, e)
	satOK(t, debug.EnableDebug("sat_missed_prop"))
	beforeMissed := p.limit.Steps
	_, e = l.propagate(false)
	if satStorageSourceDebug {
		var ex z3exception.DefaultException
		if !errors.As(e, &ex) {
			t.Fatal("sat_missed_prop tag did not find the unattached unit clause", e)
		}
	} else {
		satOK(t, e)
	}
	if p.limit.Steps != beforeMissed {
		t.Fatal("sat_missed_prop unexpectedly ran the invariant checker")
	}
}
func TestSATIntegrityDisjointFreshSetEveryRefusal(t *testing.T) {
	failures := 0
	completed := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		g, _ := satGCFixture(t, 4)
		l := g.lifecycle()
		satCleanerClause(t, l, []Literal{0, 2, 4}, false, false)
		satGCClause(t, g, []Literal{0, 2, 4, 6}, 1)
		checker, e := newSATIntegrityChecker(l, g)
		satOK(t, e)
		l.budget.FailAfter(ordinal)
		yes, e := checker.checkDisjointClauses()
		// The real source-local uint_set destructor has no callbacks: failed
		// membership/list allocations are freed before this method returns.
		satGCAudit(t, g)
		if e != nil {
			satProbingExpectedRefusal(t, l.propagation, e)
			failures++
		} else {
			if !yes {
				t.Fatal("independent ID sets")
			}
			completed = true
		}
		l.budget.FailAfter(-1)
		if completed {
			t.Log("actual set refusals", failures, "first success", ordinal)
			break
		}
	}
	if !completed || failures == 0 {
		t.Fatal(failures, completed)
	}
}

func TestSATIntegrityNaryBlockedAndReinitProfiles(t *testing.T) {
	g, _ := satGCFixture(t, 5)
	l := g.lifecycle()
	p := l.propagation
	cl := satCleanerClause(t, l, []Literal{0, 2, 4, 6}, false, false)
	satOK(t, l.attachClause(cl))
	checker, e := newSATIntegrityChecker(l, g)
	satOK(t, e)
	list, e := p.variables.watches.Get(1)
	satOK(t, e)
	prior, e := list.Read(0)
	satOK(t, e)
	changed := prior
	satOK(t, changed.SetBlockedLiteral(8))
	satOK(t, list.Write(0, changed))
	yes, e := checker.checkClause(cl)
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal("blocked membership SASSERT", e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
	satOK(t, list.Write(0, prior))
	// Use actual charged reinit backing, not a temporary Go slice masquerading
	// as a solver member. Binary source wrappers skip the irrelevant clause token.
	satOK(t, l.pushReinitStack(cl))
	priorWrapper := l.reinit[0]
	l.reinit[0] = newSATBinaryWrapper(0, 2)
	yes, e = checker.checkReinitStack()
	satOK(t, e)
	if !yes {
		t.Fatal(yes)
	}
	l.reinit[0] = priorWrapper
	satOK(t, p.clauses.SetReinitStack(cl, false))
	yes, e = checker.checkReinitStack()
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal("actual OnReinitStack", e)
		}
	} else {
		satOK(t, e)
		if !yes {
			t.Fatal(yes)
		}
	}
	satGCAudit(t, g)
	satOK(t, p.clauses.SetReinitStack(cl, true))
}
