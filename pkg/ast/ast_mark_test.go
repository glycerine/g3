package ast

// Additional finite source-model coverage: no registered mark-specific test.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func markTestNodes(t *testing.T, m *ASTManager, n uint32) []ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	out := make([]ASTHandle, n)
	for i := range out {
		out[i], e = m.MkVar(uint32(i+30000), b)
		astTestMust(t, e)
	}
	return out
}
func TestASTMarkIDDomainsReuseAndReset(t *testing.T) {
	m := astTestManager(t)
	nodes := markTestNodes(t, m, 3)
	mark, e := NewASTMark(m)
	astTestMust(t, e)
	defer mark.Close()
	expr, e := NewASTExprMark(m)
	astTestMust(t, e)
	defer expr.Close()
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	astTestMust(t, mark.Mark(nodes[0], true))
	astTestMust(t, mark.Mark(b, true))
	astTestMust(t, expr.Mark(nodes[0], true))
	for _, h := range []ASTHandle{nodes[0], b} {
		yes, e := mark.IsMarked(h)
		astTestMust(t, e)
		if !yes {
			t.Fatal("independent declaration/expression marks")
		}
	}
	if _, e = expr.IsMarked(b); !errors.Is(e, ErrASTHandle) {
		t.Fatal("expr specialization cannot dereference a declaration", e)
	}
	oldID, e := m.GetASTID(nodes[0])
	astTestMust(t, e)
	astTestMust(t, m.IncRef(nodes[0]))
	astTestMust(t, m.DecRef(nodes[0]))
	replacement, e := m.MkVar(40000, b)
	astTestMust(t, e)
	id, e := m.GetASTID(replacement)
	astTestMust(t, e)
	if id != oldID {
		t.Fatal("source recycled ID fixture")
	}
	for _, kind := range []int{0, 1} {
		var yes bool
		if kind == 0 {
			yes, e = mark.IsMarked(replacement)
		} else {
			yes, e = expr.IsMarked(replacement)
		}
		astTestMust(t, e)
		if !yes {
			t.Fatal("ID map must retain deleted-node mark")
		}
	}
	astTestMust(t, mark.Reset())
	astTestMust(t, expr.Reset())
	yes, e := mark.IsMarked(replacement)
	astTestMust(t, e)
	if yes {
		t.Fatal("reset")
	}
}
func TestASTMarkFastBothPlanesLevelsAndNonRetention(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 3)
	one, e := NewASTFastMark1(m)
	astTestMust(t, e)
	defer one.Close()
	two, e := NewASTFastMark2(m)
	astTestMust(t, e)
	defer two.Close()
	for _, n := range h {
		astTestMust(t, one.Mark(n, true))
		astTestMust(t, two.Mark(n, true))
	}
	astTestMust(t, one.Mark(h[0], true))
	if one.GetLevel() != 3 {
		t.Fatal("already-marked push")
	}
	for _, n := range h {
		rc, e := m.GetRefCount(n)
		astTestMust(t, e)
		if rc != 0 {
			t.Fatal("nonref fast list must borrow")
		}
	}
	astTestMust(t, one.SetLevel(1))
	for i, n := range h {
		a, e := one.IsMarked(n)
		astTestMust(t, e)
		b, e := two.IsMarked(n)
		astTestMust(t, e)
		if a != (i == 0) || !b {
			t.Fatal("independent planes/level restore")
		}
	}
	astTestMust(t, one.ResetMark(h[0]))
	if one.GetLevel() != 1 {
		t.Fatal("reset_mark leaves list")
	}
	astTestMust(t, one.Mark(h[0], true))
	if one.GetLevel() != 2 {
		t.Fatal("remark appends duplicate")
	}
	astTestMust(t, one.Reset())
	astTestMust(t, two.Reset())
}
func TestASTMarkFastGrowthPrefixInlineAndOwners(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 17)
	one, e := NewASTFastMark1(m)
	astTestMust(t, e)
	defer one.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for _, n := range h[:16] {
		astTestMust(t, one.Mark(n, true))
	}
	if m.BudgetField.Used() != before {
		t.Fatal("inline16 allocates")
	}
	err := one.Mark(h[16], true)
	m.BudgetField.FailAfter(-1)
	yes, e := one.IsMarked(h[16])
	astTestMust(t, e)
	if !errors.Is(err, arena.ErrInjectedFailure) || !yes || one.GetLevel() != 16 {
		t.Fatal("bit before growth source prefix", err, yes, one.GetLevel())
	}
	astTestMust(t, one.Reset())
	yes, e = one.IsMarked(h[16])
	astTestMust(t, e)
	if !yes {
		t.Fatal("reset cannot clear failed unlisted prefix")
	}
	astTestMust(t, one.ResetMark(h[16]))
	other, e := NewASTFastMark1(m)
	astTestMust(t, e)
	defer other.Close()
	astTestMust(t, one.Mark(h[0], true))
	yes, e = other.IsMarked(h[0])
	if ASTSourceDebug {
		if !errors.Is(e, ErrASTDomain) {
			t.Fatal("actual debug owner assertion", e)
		}
	} else {
		astTestMust(t, e)
		if !yes {
			t.Fatal("release shared mark")
		}
	}
}
func TestASTMarkRefFastCallbacksAndFailurePrefix(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 17)
	one, e := NewASTRefFastMark1(m)
	astTestMust(t, e)
	defer one.Close()
	two, e := NewASTRefFastMark2(m)
	astTestMust(t, e)
	defer two.Close()
	for _, n := range h[:16] {
		astTestMust(t, one.Mark(n, true))
		astTestMust(t, two.Mark(n, true))
	}
	astTestMust(t, one.ResetMark(h[0]))
	rc, e := m.GetRefCount(h[0])
	astTestMust(t, e)
	if rc != 2 {
		t.Fatal("reset_mark must not decrement")
	}
	before, e := m.GetRefCount(h[16])
	astTestMust(t, e)
	m.BudgetField.FailAfter(0)
	err := one.Mark(h[16], true)
	m.BudgetField.FailAfter(-1)
	after, e := m.GetRefCount(h[16])
	astTestMust(t, e)
	yes, e := one.IsMarked(h[16])
	astTestMust(t, e)
	if !errors.Is(err, arena.ErrInjectedFailure) || after != before+1 || !yes {
		t.Fatal("mark→INC→growth failure prefix", err, after, before, yes)
	}
	astTestMust(t, one.ResetMark(h[16]))
	astTestMust(t, m.DecRef(h[16]))
	astTestMust(t, one.Reset())
	for _, n := range h[:16] {
		rc, e := m.GetRefCount(n)
		astTestMust(t, e)
		if rc != 1 {
			t.Fatal("actual reference callbacks", rc)
		}
	}
	astTestMust(t, two.Reset())
}
func TestASTMarkLowLevelOwnerChecksAndBooleanFalse(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	astTestMust(t, m.MarkNode1(h, false, 101))
	astTestMust(t, m.MarkNode2(h, true, 202))
	yes, e := m.IsNodeMarked1(h, 101)
	astTestMust(t, e)
	if yes {
		t.Fatal("false mark still records owner only in debug")
	}
	yes, e = m.IsNodeMarked2(h, 202)
	astTestMust(t, e)
	if !yes {
		t.Fatal("mark2")
	}
	_, e = m.IsNodeMarked1(h, 102)
	if ASTSourceDebug {
		if !errors.Is(e, ErrASTDomain) {
			t.Fatal("false owner assertion", e)
		}
	} else {
		astTestMust(t, e)
	}
	astTestMust(t, m.ResetNodeMark1(h, 101))
	astTestMust(t, m.MarkNode1(h, true, 102))
	astTestMust(t, m.ResetNodeMark1(h, 102))
	astTestMust(t, m.ResetNodeMark2(h, 202))
}

func TestASTMarkLockedTraversalHelpers(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	m.mu.Lock()
	marker, e := newASTFastMark1Locked(m)
	astTestMust(t, e)
	m.BudgetField.FailAfter(0)
	astTestMust(t, marker.markLocked(m, h, true))
	yes, e := marker.nodeMarkedLocked(m, h)
	astTestMust(t, e)
	if !yes {
		t.Fatal("locked actual mark")
	}
	astTestMust(t, marker.closeLocked(m))
	m.BudgetField.FailAfter(-1)
	m.mu.Unlock()
	if m.nodeAt(h.slot).hasFlag(astFirstMark) {
		t.Fatal("lexical locked reset")
	}
}

func TestASTMarkUnsignedWrapPhysicalContinuationGuard(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	mark, e := NewASTMark(m)
	astTestMust(t, e)
	defer mark.Close()
	astTestMust(t, mark.Mark(h, true))
	old := m.nodeAt(h.slot).id
	defer func() { m.nodeAt(h.slot).id = old }()
	// A source uint32 ID can wrap. Resize UINT_MAX computes zero words,
	// keeps existing capacity, masks its first word, and publishes UINT_MAX
	// before set touches physically absent storage. Preserve that prefix.
	m.nodeAt(h.slot).id = ^uint32(0) - 1
	if e = mark.Mark(h, true); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("physical source-undefined access", e)
	}
	if mark.state.exprs.size != ^uint32(0) {
		t.Fatal("lost source resize publication", mark.state.exprs.size)
	}
	// Later source get at a logically in-range but absent physical word is
	// also undefined. The Go guard must return an error without a panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("physical read escaped typed guard: %v", r)
		}
	}()
	if _, e = mark.IsMarked(h); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("physical read guard", e)
	}
}

func TestASTMarkAllNodeKindsIndependentBitOracleAndWarmPlateau(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v := markTestNodes(t, m, 1)[0]
	app, e := m.MkConst(astTestSymbol(t, m, "mark-app"), b)
	astTestMust(t, e)
	decl, e := m.GetDecl(app)
	astTestMust(t, e)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{{}}, Body: app})
	astTestMust(t, e)
	all := []ASTHandle{v, app, q, b, decl}
	marker, e := NewASTMark(m)
	astTestMust(t, e)
	defer marker.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for _, h := range all {
		astTestMust(t, marker.Mark(h, true))
	}
	m.BudgetField.FailAfter(-1)
	if m.BudgetField.Used() != before {
		t.Fatal("native marker growth charged arena backing", before, m.BudgetField.Used())
	}
	astTestMust(t, marker.Reset())
	expected := [5]bool{}
	for i := 0; i < 300; i++ {
		index := (i*37 + 11) % len(all)
		flag := i%7 < 3
		astTestMust(t, marker.Mark(all[index], flag))
		expected[index] = flag
		for j, h := range all {
			yes, e := marker.IsMarked(h)
			astTestMust(t, e)
			if yes != expected[j] {
				t.Fatal("independent bool oracle", i, j, yes, expected[j])
			}
		}
	}
	used := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	allocs := testing.AllocsPerRun(50, func() {
		for _, h := range all {
			if e := marker.Mark(h, true); e != nil {
				panic(e)
			}
		}
		if e := marker.Reset(); e != nil {
			panic(e)
		}
	})
	m.BudgetField.FailAfter(-1)
	if allocs != 0 || m.BudgetField.Used() != used {
		t.Fatal("warm mark/reset is allocation-free and retains exact backing", allocs, m.BudgetField.Used(), used)
	}
}

func TestASTMarkFastPhysicalGrowthStaleBorrowAndEscapedCleanup(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 33)
	marker, e := NewASTFastMark2(m)
	astTestMust(t, e)
	before := m.BudgetField.Used()
	for _, n := range h {
		astTestMust(t, marker.Mark(n, true))
	}
	if len(marker.heap) != 64 || marker.heapBytes != 512 || m.BudgetField.Used() != before+512 {
		t.Fatal("inline16→32→64 physical charge", len(marker.heap), marker.heapBytes)
	}
	astTestMust(t, marker.Reset())
	if m.BudgetField.Used() != before+512 {
		t.Fatal("Reset retains buffer capacity")
	}
	astTestMust(t, marker.Mark(h[0], true))
	astTestMust(t, m.IncRef(h[0]))
	astTestMust(t, m.DecRef(h[0]))
	if e = marker.Reset(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("stale nonowning pointer adapter", e)
	}
	// Manager lifetime has ended: Close must not touch its freed raw AST graph.
	retained := marker.owner.ownerBytes + marker.heapBytes
	borrowed := m.BudgetField.Used() - m.Stats().AllocatedBytes - retained
	astTestMust(t, m.Close())
	astTestMust(t, marker.Close())
	if m.BudgetField.Used() != borrowed {
		t.Fatal("escaped marker physical refund", m.BudgetField.Used(), borrowed)
	}
}

func TestASTMarkConstructorsAllFailurePrefixesImmediatePhysicalOwnership(t *testing.T) {
	for kind := 0; kind < 6; kind++ {
		for after := int64(0); after < 8; after++ {
			m := astTestManager(t)
			borrowed := m.BudgetField.Used() - m.Stats().AllocatedBytes
			m.BudgetField.FailAfter(after)
			var err error
			switch kind {
			case 0:
				v, e := NewASTExprMark(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			case 1:
				v, e := NewASTMark(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			case 2:
				v, e := NewASTFastMark1(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			case 3:
				v, e := NewASTFastMark2(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			case 4:
				v, e := NewASTRefFastMark1(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			case 5:
				v, e := NewASTRefFastMark2(m)
				err = e
				if e == nil {
					err = v.Close()
				}
			}
			m.BudgetField.FailAfter(-1)
			if err != nil && !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(kind, after, err)
			}
			// Successful physical allocator prefixes can be retained by the actual
			// AST owner. No unreachable marker header/backing may remain, BEFORE
			// manager Close or any numeral cleanup can conceal a leak.
			if m.BudgetField.Used() != borrowed+m.Stats().AllocatedBytes {
				t.Fatal("unaccounted marker constructor prefix", kind, after, m.BudgetField.Used(), borrowed+m.Stats().AllocatedBytes)
			}
		}
	}
}

func TestASTMarkRefFastActualAscendingReleaseAndWarmOperations(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 3)
	marker, e := NewASTRefFastMark2(m)
	astTestMust(t, e)
	defer marker.Close()
	ids := [3]uint32{}
	for i, n := range h {
		ids[i], e = m.GetASTID(n)
		astTestMust(t, e)
		astTestMust(t, marker.Mark(n, true))
	}
	astTestMust(t, marker.Mark(h[0], false))
	rc, e := m.GetRefCount(h[0])
	astTestMust(t, e)
	if rc != 1 {
		t.Fatal("flag false clears only bit")
	}
	astTestMust(t, marker.Reset())
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	for i := uint32(0); i < 3; i++ {
		n, e := m.MkVar(45000+i, b)
		astTestMust(t, e)
		id, e := m.GetASTID(n)
		astTestMust(t, e)
		if id != ids[2-i] {
			t.Fatal("actual ascending reference release/ID recycle", i, id, ids)
		}
		h[i] = n
	}
	pin := h
	for _, n := range pin {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range pin {
			astTestMust(t, m.DecRef(n))
		}
	}()
	used := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	allocs := testing.AllocsPerRun(30, func() {
		for _, n := range h {
			if e := marker.Mark(n, true); e != nil {
				panic(e)
			}
		}
		if e := marker.Reset(); e != nil {
			panic(e)
		}
	})
	m.BudgetField.FailAfter(-1)
	if allocs != 0 || m.BudgetField.Used() != used {
		t.Fatal("warm real retain/release allocates", allocs, m.BudgetField.Used(), used)
	}
}

func TestASTMarkCopiedOwnersAndNilLifetimeGuards(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	marker, e := NewASTFastMark1(m)
	astTestMust(t, e)
	defer marker.Close()
	copyOwner := *marker
	if e = copyOwner.Mark(h, true); !errors.Is(e, ErrASTHandle) {
		t.Fatal("copied mark owner accepted", e)
	}
	if e = copyOwner.Close(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("copied owner refunded original", e)
	}
	var empty *ASTFastMark1
	if e = empty.Mark(h, true); !errors.Is(e, ErrASTHandle) {
		t.Fatal("nil cold owner", e)
	}
	astTestMust(t, marker.Close())
	if e = marker.Reset(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed marker", e)
	}
}

func TestASTMarkHeaderRefusalPrecedesGoAllocation(t *testing.T) {
	m := astTestManager(t)
	constructors := []func() error{
		func() error { _, e := NewASTExprMark(m); return e },
		func() error { _, e := NewASTMark(m); return e },
		func() error { _, e := NewASTFastMark1(m); return e },
		func() error { _, e := NewASTFastMark2(m); return e },
		func() error { _, e := NewASTRefFastMark1(m); return e },
		func() error { _, e := NewASTRefFastMark2(m); return e },
	}
	for i, construct := range constructors {
		allocs := testing.AllocsPerRun(20, func() {
			m.BudgetField.FailAfter(0)
			if e := construct(); !errors.Is(e, arena.ErrInjectedFailure) {
				panic(e)
			}
		})
		m.BudgetField.FailAfter(-1)
		if allocs != 0 {
			t.Fatalf("constructor %d allocated %v uncharged headers before refusal", i, allocs)
		}
	}
}
