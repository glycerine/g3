package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fixedClauseFixture(t *testing.T, isInt []bool) (*numeral.FixedPointManager, *FixedContext, *numeral.FixedPoint, *FixedNode) {
	t.Helper()
	m, err := numeral.NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFixedContext(m, &control.Reslimit{Limit: 10000}, isInt, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	n, err := c.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, err := range []error{c.Close(), v.Close(), m.Close()} {
			if err != nil {
				t.Error(err)
			}
		}
	})
	return m, c, v, n
}

func fixedClauseAtom(t *testing.T, m *numeral.FixedPointManager, c *FixedContext, v *numeral.FixedPoint, x uint32, value int64, lower, open bool) *FixedInequality {
	t.Helper()
	if err := m.SetInt64(v, value); err != nil {
		t.Fatal(err)
	}
	a, err := c.NewInequality(x, v, lower, open)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func fixedClauseBound(t *testing.T, m *numeral.FixedPointManager, c *FixedContext, n *FixedNode, v *numeral.FixedPoint, x uint32, value int64, lower bool) {
	t.Helper()
	if err := m.SetInt64(v, value); err != nil {
		t.Fatal(err)
	}
	if err := c.PropagateBound(n, x, v, lower, false); err != nil {
		t.Fatal(err)
	}
}

func TestSubpavingFixedClauseWatchPropagationAndConflict(t *testing.T) {
	m, c, v, root := fixedClauseFixture(t, []bool{false, false})
	yUpper := fixedClauseAtom(t, m, c, v, 1, 2, false, false)
	xLower := fixedClauseAtom(t, m, c, v, 0, 5, true, false)
	xUpper := fixedClauseAtom(t, m, c, v, 0, 9, false, true)
	clause, err := c.AddClause([]*FixedInequality{yUpper, xLower, xUpper}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(clause.atoms) != 3 || clause.atoms[0] != xLower || clause.atoms[1] != xUpper || clause.atoms[2] != yUpper ||
		len(c.watchOrder[0]) != 1 || len(c.watchOrder[1]) != 1 || xLower.refs != 1 || xUpper.refs != 1 || yUpper.refs != 1 {
		t.Fatal("stable source order, unique watches and atom references")
	}
	fixedClauseBound(t, m, c, root, v, 0, 4, false)
	if err := c.PropagateQueued(root); err != nil {
		t.Fatal(err)
	}
	if root.upper[1] != nil || clause.visited == 0 {
		t.Fatal("satisfied clause propagated or was not visited")
	}

	m2, c2, v2, root2 := fixedClauseFixture(t, []bool{false, false})
	a0 := fixedClauseAtom(t, m2, c2, v2, 0, 5, true, false)
	a1 := fixedClauseAtom(t, m2, c2, v2, 1, 2, false, false)
	unit, err := c2.AddClause([]*FixedInequality{a0, a1}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	fixedClauseBound(t, m2, c2, root2, v2, 0, 4, false)
	if err := c2.PropagateQueued(root2); err != nil {
		t.Fatal(err)
	}
	if root2.upper[1] == nil || root2.inconsistent || root2.upper[1].reason != (fixedBoundReason{kind: fixedReasonClause, clause: unit}) {
		t.Fatal("sole unknown atom was not propagated with its clause reason")
	}
	if cmp, err := m2.Compare(root2.upper[1].value, a1.value); err != nil || cmp != 0 {
		t.Fatal("propagated bound value", cmp, err)
	}

	m3, c3, v3, root3 := fixedClauseFixture(t, []bool{false, false})
	fixedClauseBound(t, m3, c3, root3, v3, 0, 4, false)
	fixedClauseBound(t, m3, c3, root3, v3, 1, 3, true)
	b0 := fixedClauseAtom(t, m3, c3, v3, 0, 5, true, false)
	b1 := fixedClauseAtom(t, m3, c3, v3, 1, 2, false, false)
	conflict, err := c3.AddClause([]*FixedInequality{b1, b0}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := c3.PropagateClause(conflict, root3); err != nil {
		t.Fatal(err)
	}
	if !root3.inconsistent || root3.conflictVar != 0 || root3.lower[0].reason != (fixedBoundReason{kind: fixedReasonClause, clause: conflict}) {
		t.Fatal("all-false clause did not conflict on first sorted atom")
	}
}

func TestSubpavingFixedClauseUnitsNoResumeAndCleanup(t *testing.T) {
	m, c, v, root := fixedClauseFixture(t, []bool{false, false})
	a0 := fixedClauseAtom(t, m, c, v, 0, 1, true, false)
	a1 := fixedClauseAtom(t, m, c, v, 1, 2, false, false)
	if clause, err := c.AddClause([]*FixedInequality{a0}, true, false); err != nil || clause != nil {
		t.Fatal("singleton clause must become an axiom unit", clause, err)
	}
	if err := c.AddUnitClause(a1, false); err != nil {
		t.Fatal(err)
	}
	if !c.unitClauses[0].axiom || c.unitClauses[1].axiom {
		t.Fatal("unit reason tags")
	}
	c.limit.Limit, c.limit.Steps = 1, 0
	if err := c.InitializeRoot(); !errors.Is(err, ErrFixedContextCanceled) || c.unitHead != 1 || c.initStage != 0 {
		t.Fatal("unit cancellation suffix", c.unitHead, c.initStage, err)
	}
	if root.lower[0] == nil || root.lower[0].reason.kind != fixedReasonAxiom || root.upper[1] != nil {
		t.Fatal("first unit publication or canceled suffix")
	}
	queued := len(c.queue)
	c.limit.Limit, c.limit.Steps = 100, 0
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	if c.initStage != 0 || c.unitHead != 1 || root.upper[1] != nil || root.lower[0] == nil || len(c.queue) != queued {
		t.Fatal("later call resumed the canceled source root initialization", c.initStage, c.unitHead)
	}
	if _, err := c.NewVariable(false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("variable growth after root initialization started", err)
	}
	if _, err := c.DefineMonomial([]FixedPower{{Variable: 0, Degree: 2}}); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("monomial definition after root initialization started", err)
	}
	if _, err := c.DefinePolynomial(a0.value, []FixedPolynomialTerm{{Variable: 0, Coefficient: a0.value}}); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("polynomial definition after root initialization started", err)
	}
	if _, err := c.NewInequality(0, a0.value, true, false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("inequality creation after root initialization started", err)
	}
	if err := c.AddUnitClause(a1, false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("unit addition after root initialization started", err)
	}
	if _, err := c.AddClause([]*FixedInequality{a0, a1}, true, true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("clause addition after root initialization started", err)
	}
	if err := c.releaseUnit(0); err != nil {
		t.Fatal(err)
	}
	if _, err := m.IsZero(a0.value); !errors.Is(err, numeral.ErrFixedPointClosed) {
		t.Fatal("released unit retained numeral", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.AddUnitClause(a1, true); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed context accepted unit", err)
	}
}

func TestSubpavingFixedInequalityThreeValuedOpenEndpoints(t *testing.T) {
	tests := []struct {
		name                string
		atomLower, atomOpen bool
		atomValue           int64
		boundLower          bool
		boundOpen           bool
		boundValue          int64
		secondBound         bool
		secondLower         bool
		secondOpen          bool
		secondValue         int64
		want                fixedTruth
	}{
		{name: "unknown without bounds", atomLower: true, atomValue: 5, want: fixedUnknown},
		{name: "lower false below upper", atomLower: true, atomValue: 5, boundLower: false, boundValue: 4, want: fixedFalse},
		{name: "lower false at open upper", atomLower: true, atomValue: 5, boundLower: false, boundOpen: true, boundValue: 5, want: fixedFalse},
		{name: "lower false at open atom", atomLower: true, atomOpen: true, atomValue: 5, boundLower: false, boundValue: 5, want: fixedFalse},
		{name: "lower true above atom", atomLower: true, atomValue: 5, boundLower: true, boundValue: 6, want: fixedTrue},
		{name: "lower true at closed atom", atomLower: true, atomValue: 5, boundLower: true, boundValue: 5, want: fixedTrue},
		{name: "lower true at open lower bound", atomLower: true, atomOpen: true, atomValue: 5, boundLower: true, boundOpen: true, boundValue: 5, want: fixedTrue},
		{name: "lower unknown at open atom equality", atomLower: true, atomOpen: true, atomValue: 5, boundLower: true, boundValue: 5, want: fixedUnknown},
		{name: "lower unknown between bounds", atomLower: true, atomValue: 5, boundLower: true, boundValue: 4, secondBound: true, secondValue: 6, want: fixedUnknown},
		{name: "upper unknown without bounds", atomLower: false, atomValue: 5, want: fixedUnknown},
		{name: "upper false above lower", atomLower: false, atomValue: 5, boundLower: true, boundValue: 6, want: fixedFalse},
		{name: "upper false at open lower", atomLower: false, atomValue: 5, boundLower: true, boundOpen: true, boundValue: 5, want: fixedFalse},
		{name: "upper false at open atom", atomLower: false, atomOpen: true, atomValue: 5, boundLower: true, boundValue: 5, want: fixedFalse},
		{name: "upper true below upper", atomLower: false, atomValue: 5, boundLower: false, boundValue: 4, want: fixedTrue},
		{name: "upper true at closed atom", atomLower: false, atomValue: 5, boundLower: false, boundValue: 5, want: fixedTrue},
		{name: "upper true at open upper", atomLower: false, atomOpen: true, atomValue: 5, boundLower: false, boundOpen: true, boundValue: 5, want: fixedTrue},
		{name: "upper unknown at open atom equality", atomLower: false, atomOpen: true, atomValue: 5, boundLower: false, boundValue: 5, want: fixedUnknown},
		{name: "upper unknown between bounds", atomLower: false, atomValue: 5, boundLower: true, boundValue: 4, secondBound: true, secondLower: false, secondValue: 6, want: fixedUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, c, v, root := fixedClauseFixture(t, []bool{false})
			bound := func(value int64, lower, open bool) {
				t.Helper()
				if err := m.SetInt64(v, value); err != nil {
					t.Fatal(err)
				}
				if err := c.PropagateBound(root, 0, v, lower, open); err != nil {
					t.Fatal(err)
				}
			}
			if tc.boundLower || tc.boundOpen || tc.boundValue != 0 {
				bound(tc.boundValue, tc.boundLower, tc.boundOpen)
			}
			if tc.secondBound {
				bound(tc.secondValue, tc.secondLower, tc.secondOpen)
			}
			atom := fixedClauseAtom(t, m, c, v, 0, tc.atomValue, tc.atomLower, tc.atomOpen)
			got, err := c.inequalityValue(atom, root)
			if err != nil || got != tc.want {
				t.Fatalf("value = %v, want %v (err %v)", got, tc.want, err)
			}
		})
	}
}

func TestSubpavingFixedClauseReasonLivesUntilNodeThenCloses(t *testing.T) {
	m, c, v, root := fixedClauseFixture(t, []bool{false, false})
	x := fixedClauseAtom(t, m, c, v, 0, 0, true, false)
	y := fixedClauseAtom(t, m, c, v, 1, 0, true, false)
	clause, err := c.AddClause([]*FixedInequality{x, y}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	fixedClauseBound(t, m, c, root, v, 0, -1, false)
	if err := c.PropagateQueued(root); err != nil {
		t.Fatal(err)
	}
	if root.lower[1] == nil || root.lower[1].reason != (fixedBoundReason{kind: fixedReasonClause, clause: clause}) || clause.deleted {
		t.Fatal("bound lost its live clause reason")
	}
	if x.refs != 1 || y.refs != 1 || len(c.watchOrder[0]) != 1 || len(c.watchOrder[1]) != 1 {
		t.Fatal("clause did not own its atoms and unique watches")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if !root.closed || !clause.deleted || !clause.watchesRemoved || x.refs != 0 || y.refs != 0 || len(c.watchOrder[0]) != 0 || len(c.watchOrder[1]) != 0 {
		t.Fatal("context teardown did not close nodes before releasing clause ownership")
	}
	if _, err := m.IsZero(x.value); !errors.Is(err, numeral.ErrFixedPointClosed) {
		t.Fatal("clause atom value remained live after context close", err)
	}
}
