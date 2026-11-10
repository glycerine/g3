package ast

// No registered shared_occs test exists; these are additional finite source models.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func sharedTestApp(t *testing.T, m *ASTManager, args ...ASTHandle) ASTHandle {
	t.Helper()
	h, e := m.MkFamilyApp(0, ASTOpAnd, nil, args)
	astTestMust(t, e)
	return h
}
func TestASTSharedOccurrencesActualMultiplicityAndDefaults(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 2)
	sub := sharedTestApp(t, m, h[0], h[1])
	root := sharedTestApp(t, m, sub, sub)
	collect, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer collect.Close()
	cfg := DefaultASTSharedOccurrencesConfig()
	if cfg.TrackAtomic || !cfg.VisitQuantifiers || cfg.VisitPatterns {
		t.Fatal("source defaults", cfg)
	}
	before, e := m.GetRefCount(sub)
	astTestMust(t, e)
	astTestMust(t, collect.Collect(root))
	yes, e := collect.IsShared(sub)
	astTestMust(t, e)
	if !yes || collect.NumShared() != 1 {
		t.Fatal("shared composite")
	}
	after, e := m.GetRefCount(sub)
	astTestMust(t, e)
	if after != before+1 {
		t.Fatal("actual unique-set reference retain", before, after)
	}
	astTestMust(t, collect.Collect(root))
	again, e := m.GetRefCount(sub)
	astTestMust(t, e)
	if again != after {
		t.Fatal("default reset then discovery")
	}
	astTestMust(t, collect.Reset())
	after, e = m.GetRefCount(sub)
	astTestMust(t, e)
	if after != before {
		t.Fatal("real reset DEC", after, before)
	}
	atomicRoot := sharedTestApp(t, m, h[0], h[0])
	astTestMust(t, collect.Collect(atomicRoot))
	if collect.NumShared() != 0 {
		t.Fatal("default ignores atomic vars")
	}
	atom, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true, VisitQuantifiers: true})
	astTestMust(t, e)
	defer atom.Close()
	astTestMust(t, atom.Collect(atomicRoot))
	yes, e = atom.IsShared(h[0])
	astTestMust(t, e)
	if !yes || atom.NumShared() != 1 {
		t.Fatal("actual atomic specialization")
	}
}
func TestASTSharedOccurrencesExternalVisitedAndPlaneIndependence(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	pins := []ASTHandle{h, h}
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range pins {
			astTestMust(t, m.DecRef(n))
		}
	}()
	visited, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer visited.Close()
	collect, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true, VisitQuantifiers: true})
	astTestMust(t, e)
	defer collect.Close()
	astTestMust(t, m.MarkNode1(h, true, 111))
	astTestMust(t, m.MarkNode2(h, true, 222))
	astTestMust(t, collect.CollectWithMark(h, visited))
	if collect.NumShared() != 0 {
		t.Fatal("RC>1 alone is not repeated occurrence")
	}
	astTestMust(t, collect.CollectWithMark(h, visited))
	if collect.NumShared() != 1 {
		t.Fatal("external visited overload must persist visitation")
	}
	astTestMust(t, visited.Reset())
	for plane := 1; plane <= 2; plane++ {
		var yes bool
		if plane == 1 {
			yes, e = m.IsNodeMarked1(h, 111)
		} else {
			yes, e = m.IsNodeMarked2(h, 222)
		}
		astTestMust(t, e)
		if !yes {
			t.Fatal("shared flag must be separate from public mark planes")
		}
	}
	astTestMust(t, m.ResetNodeMark1(h, 111))
	astTestMust(t, m.ResetNodeMark2(h, 222))
}
func TestASTSharedOccurrencesNativeVisitedGrowthAndReset(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 257)
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for _, n := range h {
		astTestMust(t, v.Mark(n, true))
		astTestMust(t, v.Mark(n, true))
	}
	for _, n := range h {
		yes, e := v.IsMarked(n)
		astTestMust(t, e)
		if !yes {
			t.Fatal("missing visited key")
		}
	}
	astTestMust(t, v.Reset())
	for _, n := range h {
		yes, e := v.IsMarked(n)
		astTestMust(t, e)
		if yes {
			t.Fatal("native overflow lost reset key")
		}
		rc, e := m.GetRefCount(n)
		astTestMust(t, e)
		if rc != 0 {
			t.Fatal("visited marker retained node", rc)
		}
	}
	m.BudgetField.FailAfter(-1)
	if m.BudgetField.Used() != before {
		t.Fatal("native visitation charged container storage")
	}
}

func TestASTSharedOccurrencesNativeTableOrderGrowthAndOwnership(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 257)
	pins := append(append([]ASTHandle{}, h...), h...)
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range pins {
			astTestMust(t, m.DecRef(n))
		}
	}()
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	c, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true})
	astTestMust(t, e)
	defer c.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for _, n := range h {
		astTestMust(t, v.Mark(n, true))
		astTestMust(t, c.CollectWithMark(n, v))
		astTestMust(t, c.CollectWithMark(n, v))
	}
	if c.NumShared() != uint32(len(h)) || m.BudgetField.Used() != before {
		t.Fatal("native owning dictionary growth")
	}
	// begin_shared walks m_shared (an obj_hashtable) in table order.
	layout := NewObjMap(m)
	for _, n := range h {
		_, e := layout.InsertIfNotThere(n)
		astTestMust(t, e)
	}
	var order []ASTHandle
	for _, slot := range layout.Entries() {
		order = append(order, slot.Key)
	}
	it, e := c.BeginShared()
	astTestMust(t, e)
	next := 0
	for {
		done, e := it.Done()
		astTestMust(t, e)
		if done {
			break
		}
		got, e := it.Value()
		astTestMust(t, e)
		rc, e := m.GetRefCount(got)
		astTestMust(t, e)
		if next >= len(order) || got != order[next] || rc != 3 {
			t.Fatal("table order and exact owning retain", next, got, rc)
		}
		next++
		astTestMust(t, it.Next())
	}
	if next != len(h) {
		t.Fatal("iterator skipped keys", next)
	}
	m.BudgetField.FailAfter(-1)
	astTestMust(t, v.Reset())
}

func TestASTSharedOccurrencesAllTraversalPolicies(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 2)
	sub := sharedTestApp(t, m, h[0], h[1])
	pattern, e := m.MkFamilyApp(2, 0, nil, []ASTHandle{sub})
	astTestMust(t, e)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: sub, Patterns: []ASTHandle{pattern}})
	astTestMust(t, e)
	// sub occurs once as body and once under the pattern. RC alone does not
	// qualify it: actual traversal must encounter it twice.
	for bits := 0; bits < 8; bits++ {
		cfg := ASTSharedOccurrencesConfig{TrackAtomic: bits&1 != 0, VisitQuantifiers: bits&2 != 0, VisitPatterns: bits&4 != 0}
		c, e := NewASTSharedOccurrencesWithConfig(m, cfg)
		astTestMust(t, e)
		astTestMust(t, c.Collect(q))
		yes, e := c.IsShared(sub)
		astTestMust(t, e)
		if yes != (cfg.VisitQuantifiers && cfg.VisitPatterns) {
			t.Fatal("actual quantifier/pattern child routing", cfg, yes)
		}
		astTestMust(t, c.Close())
	}
	// Repeated quantifiers qualify before the VisitQuantifiers test.
	outer := sharedTestApp(t, m, q, q)
	c, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{})
	astTestMust(t, e)
	defer c.Close()
	astTestMust(t, c.Collect(outer))
	yes, e := c.IsShared(q)
	astTestMust(t, e)
	if !yes || c.NumShared() != 1 {
		t.Fatal("quantifier sharing before descent policy")
	}
}

func TestASTSharedOccurrencesNativeWarmBudgetAndHeaderRefusal(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 2)
	sub := sharedTestApp(t, m, h[0], h[1])
	root := sharedTestApp(t, m, sub, sub)
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer c.Close()
	astTestMust(t, c.Collect(root))
	beforeCollect := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for i := 0; i < 100; i++ {
		astTestMust(t, c.Collect(root))
	}
	m.BudgetField.FailAfter(-1)
	if m.BudgetField.Used() != beforeCollect {
		t.Fatal("warm collection charged native dictionary storage")
	}
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	if n := testing.AllocsPerRun(20, func() {
		_, e := NewASTSharedOccurrences(m)
		if !errors.Is(e, arena.ErrInjectedFailure) {
			panic(e)
		}
	}); n != 0 {
		t.Fatal("cold header allocation before budget refusal", n)
	}
	m.BudgetField.FailAfter(-1)
	if m.BudgetField.Used() != before {
		t.Fatal("refusal changed owner graph")
	}
}

func TestASTSharedOccurrencesNativeDeepFrameTraversalAndCleanup(t *testing.T) {
	m := astTestManager(t)
	leaf := markTestNodes(t, m, 1)[0]
	root := leaf
	for i := 0; i < 257; i++ {
		root = sharedTestApp(t, m, root)
	}
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer c.Close()
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	astTestMust(t, c.CollectWithMark(root, v))
	if len(c.stack) != 0 || c.NumShared() != 0 || m.BudgetField.Used() != before {
		t.Fatal("native deep traversal")
	}
	astTestMust(t, c.Reset())
	astTestMust(t, c.CollectWithMark(leaf, v))
	astTestMust(t, c.Cleanup())
	m.BudgetField.FailAfter(-1)
	if c.stack != nil {
		t.Fatal("cleanup retained traversal frames")
	}
}

func TestASTSharedOccurrencesNativeResetTableOrderDestruction(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 10)
	pins := append(append([]ASTHandle{}, h...), h...)
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	c, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true})
	astTestMust(t, e)
	defer c.Close()
	for _, n := range h {
		astTestMust(t, v.Mark(n, true))
		astTestMust(t, c.CollectWithMark(n, v))
	}
	astTestMust(t, v.Reset()) // borrowed visitation list must outlive its nodes
	for _, n := range pins {
		astTestMust(t, m.DecRef(n))
	} // leave the collector's sole retain
	// dec_ref_collection_values walks m_shared (an obj_hashtable) in table
	// order; an obj_map of the same keys has the same layout.
	layout := NewObjMap(m)
	for _, n := range h {
		_, e := layout.InsertIfNotThere(n)
		astTestMust(t, e)
	}
	var expected []uint32
	for _, slot := range layout.Entries() {
		id, e := m.GetASTID(slot.Key)
		astTestMust(t, e)
		expected = append(expected, id)
	}
	astTestMust(t, c.Reset())
	// Each erase/recycle occurs in table order. The project ID helper
	// returns these IDs in reverse, independently checking actual DEC callbacks.
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	for i := len(expected) - 1; i >= 0; i-- {
		n, e := m.MkVar(uint32(80000+i), b)
		astTestMust(t, e)
		id, e := m.GetASTID(n)
		astTestMust(t, e)
		if id != expected[i] {
			t.Fatal("discovery-order destruction/LIFO ID reuse", i, id, expected[i])
		}
	}
}

func TestASTSharedOccurrencesNativeWideDeletionReset(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 20)
	root := sharedTestApp(t, m, h...)
	pins := []ASTHandle{root, root}
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer c.Close()
	astTestMust(t, v.Mark(root, true))
	astTestMust(t, c.CollectWithMark(root, v))
	astTestMust(t, v.Reset())
	for _, n := range pins {
		astTestMust(t, m.DecRef(n))
	}
	r, e := m.internal(root)
	astTestMust(t, e)
	before := m.liveNodes
	m.BudgetField.FailAfter(0)
	e = c.Reset()
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	if m.validNode(r) == nil || m.liveNodes != before-21 || c.NumShared() != 0 || c.resetPending {
		t.Fatal("native wide deletion reset", m.liveNodes, before, c.NumShared(), c.resetPending)
	}
	astTestMust(t, c.Reset())
	if c.NumShared() != 0 || c.resetPending || m.liveNodes != before-21 {
		t.Fatal("retry DEC already consumed node")
	}
}

func TestASTSharedOccurrencesForcedManagerCloseAndIteratorBorrow(t *testing.T) {
	m := astTestManager(t)
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	s, e := NewASTExprSparseMark(m)
	astTestMust(t, e)
	it, e := c.BeginShared()
	astTestMust(t, e)
	charged := c.owner.ownerBytes + v.owner.ownerBytes + s.owner.ownerBytes
	astTestMust(t, m.Close())
	if _, e = it.Done(); e == nil {
		t.Fatal("iterator must reject forced closed manager lifetime")
	}
	before := m.BudgetField.Used()
	astTestMust(t, c.Close())
	astTestMust(t, v.Close())
	astTestMust(t, s.Close())
	if m.BudgetField.Used() != before-charged {
		t.Fatal("forced teardown refunds only external storage, with no dead AST callbacks")
	}
}

func TestASTSharedOccurrencesNativeIteratorInvalidationAndDuplicates(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 2)
	pins := []ASTHandle{h[0], h[0], h[1], h[1]}
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range pins {
			astTestMust(t, m.DecRef(n))
		}
	}()
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	c, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true})
	astTestMust(t, e)
	defer c.Close()
	for _, n := range h {
		astTestMust(t, v.Mark(n, true))
	}
	astTestMust(t, c.CollectWithMark(h[0], v))
	it, e := c.BeginShared()
	astTestMust(t, e)
	end, e := c.EndShared()
	astTestMust(t, e)
	astTestMust(t, c.CollectWithMark(h[0], v))
	got, e := it.Value()
	astTestMust(t, e)
	if got != h[0] {
		t.Fatal("duplicate changed native view")
	}
	astTestMust(t, c.CollectWithMark(h[1], v))
	if _, e := it.Value(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("new key did not invalidate view", e)
	}
	if _, e := end.Done(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("new key did not invalidate endpoint", e)
	}
	it, e = c.BeginShared()
	astTestMust(t, e)
	astTestMust(t, c.Reset())
	if _, e := it.Value(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("reset retained stale borrowed iterator", e)
	}
	astTestMust(t, v.Reset())
}

func TestASTSharedOccurrencesNativeRepeatedDeepBranch(t *testing.T) {
	m := astTestManager(t)
	branch := markTestNodes(t, m, 1)[0]
	for i := 0; i < 257; i++ {
		branch = sharedTestApp(t, m, branch)
	}
	root := sharedTestApp(t, m, branch, branch)
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer c.Close()
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	astTestMust(t, c.Collect(root))
	yes, e := c.IsShared(branch)
	astTestMust(t, e)
	rc, e := m.GetRefCount(branch)
	astTestMust(t, e)
	if !yes || c.NumShared() != 1 || rc != 3 || len(c.stack) != 0 || m.BudgetField.Used() != before {
		t.Fatal("deep repeated branch multiplicity", yes, rc)
	}
	astTestMust(t, c.Reset())
	rc, e = m.GetRefCount(branch)
	astTestMust(t, e)
	if rc != 2 {
		t.Fatal("reset lost parent retains", rc)
	}
	m.BudgetField.FailAfter(-1)
}

func TestASTSharedOccurrencesNativeIteratorCopiesEndpointsAndClose(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 1)[0]
	pins := []ASTHandle{h, h}
	for _, n := range pins {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range pins {
			astTestMust(t, m.DecRef(n))
		}
	}()
	v, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer v.Close()
	c, e := NewASTSharedOccurrencesWithConfig(m, ASTSharedOccurrencesConfig{TrackAtomic: true})
	astTestMust(t, e)
	astTestMust(t, v.Mark(h, true))
	astTestMust(t, c.CollectWithMark(h, v))
	it, e := c.BeginShared()
	astTestMust(t, e)
	copy := it
	same, e := it.Equal(copy)
	astTestMust(t, e)
	if !same {
		t.Fatal("copied native iterator changed view")
	}
	end, e := c.EndShared()
	astTestMust(t, e)
	astTestMust(t, copy.Next())
	same, e = copy.Equal(end)
	astTestMust(t, e)
	if !same {
		t.Fatal("native endpoint mismatch")
	}
	if _, e := end.Value(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("read past end", e)
	}
	if e := end.Next(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("advance past end", e)
	}
	astTestMust(t, v.Reset())
	astTestMust(t, c.Close())
	if _, e := it.Done(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed owner iterator accepted", e)
	}
}

func TestASTSparseSharedExpressionDomainsAndASTWideVisited(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	truth, e := m.MkFamilyApp(0, ASTOpTrue, nil, nil)
	astTestMust(t, e)
	decl, e := m.GetDecl(truth)
	astTestMust(t, e)
	sparse, e := NewASTExprSparseMark(m)
	astTestMust(t, e)
	defer sparse.Close()
	c, e := NewASTSharedOccurrences(m)
	astTestMust(t, e)
	defer c.Close()
	visited, e := NewASTSharedOccurrencesMark(m)
	astTestMust(t, e)
	defer visited.Close()
	for _, h := range []ASTHandle{sort, decl} {
		if e = sparse.Mark(h, true); !errors.Is(e, ErrASTDomain) {
			t.Error("source expr* mark facade accepts nonexpression", e)
		}
		if e = sparse.Mark(h, false); !errors.Is(e, ErrASTDomain) {
			t.Error("source expr* erase facade accepts nonexpression", e)
		}
		if _, e = sparse.IsMarked(h); !errors.Is(e, ErrASTDomain) {
			t.Error("source expr* contains facade accepts nonexpression", e)
		}
		if _, e = c.IsShared(h); !errors.Is(e, ErrASTDomain) {
			t.Error("source expr* shared facade accepts nonexpression", e)
		}
		astTestMust(t, visited.Mark(h, true))
		yes, e := visited.IsMarked(h)
		astTestMust(t, e)
		if !yes {
			t.Fatal("shared_occs_mark intentionally takes ast*, not expr*")
		}
	}
	if len(sparse.entries) != 0 {
		t.Fatal("rejected domains publish no sparse keys")
	}
	astTestMust(t, visited.Reset())
}
