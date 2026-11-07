package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtEnodeMapLookup(nodes map[ast.ASTHandle]*SMTEnode) func(ast.ASTHandle) *SMTEnode {
	return func(h ast.ASTHandle) *SMTEnode { return nodes[h] }
}

func TestExprTableAbsentHandlesReadAsZero(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "absent")
	for _, h := range []ast.ASTHandle{{}, value} {
		if context.enodes.get(h) != nil || context.relevant.get(h) || context.assignments.get(h) != lbool.Undef {
			t.Fatal("absent handle has state")
		}
		if _, found := context.assignments.lookup(h); found {
			t.Fatal("absent assignment found")
		}
		if _, found := context.relevant.lookup(h); found {
			t.Fatal("absent relevance found")
		}
	}
	if context.enodes.count() != 0 {
		t.Fatal("empty table counts enodes")
	}
}

func TestExprTableFieldsAreIndependent(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "fields")
	booleans := smtBoolVarTable{context.exprs}

	booleans.set(value, 7)
	context.assignments.set(value, lbool.True)
	context.relevant.set(value, false)
	if variable, found := booleans.lookup(value); !found || variable != 7 {
		t.Fatal("boolean variable lost", variable, found)
	}
	if relevant, found := context.relevant.lookup(value); !found || relevant {
		t.Fatal("an explicit false relevance must stay present", relevant, found)
	}
	context.relevant.set(value, true)
	if !context.relevant.get(value) {
		t.Fatal("relevance not updated")
	}
	context.assignments.del(value)
	if _, found := context.assignments.lookup(value); found {
		t.Fatal("assignment survived delete")
	}
	if variable, found := booleans.lookup(value); !found || variable != 7 || !context.relevant.get(value) {
		t.Fatal("deleting one field disturbed the others")
	}
	context.relevant.del(value)
	if _, found := context.relevant.lookup(value); found || context.relevant.get(value) {
		t.Fatal("relevance survived delete")
	}
	booleans.del(value)
	if _, found := booleans.lookup(value); found || booleans.count() != 0 {
		t.Fatal("boolean variable survived delete")
	}
	booleans.del(value)
	if booleans.count() != 0 {
		t.Fatal("double delete changed the count")
	}
}

// A zero boolean variable is a present value, distinct from absence.
func TestExprTableZeroValuesArePresent(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "zero")
	booleans := smtBoolVarTable{context.exprs}
	booleans.set(value, 0)
	if variable, found := booleans.lookup(value); !found || variable != 0 {
		t.Fatal("zero variable reads as absent")
	}
	context.assignments.set(value, lbool.Undef)
	if assignment, found := context.assignments.lookup(value); !found || assignment != lbool.Undef {
		t.Fatal("undef assignment reads as absent")
	}
}

// A reused slot must not expose the retired expression's state.
func TestExprTableRetiresOlderGenerations(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	booleans := smtBoolVarTable{context.exprs}
	var retired ast.ASTHandle
	reused := false
	for i := 0; i < 64 && !reused; i++ {
		first := smtRelevancyBool(t, m, "retired-"+string(rune('a'+i)))
		ref, err := m.NewRef(first)
		astTestMust(t, err)
		booleans.set(first, 3)
		context.relevant.set(first, true)
		context.assignments.set(first, lbool.False)
		retired = first
		astTestMust(t, m.DelRef(&ref))
		second := smtRelevancyBool(t, m, "reused-"+string(rune('a'+i)))
		if second.Slot() != retired.Slot() {
			// Not this slot: clear and retry after freeing the extra constant.
			booleans.del(first)
			context.relevant.del(first)
			context.assignments.del(first)
			continue
		}
		reused = true
		if second.Generation() == retired.Generation() {
			t.Fatal("slot reuse kept its generation")
		}
		if _, found := booleans.lookup(second); found || context.relevant.get(second) {
			t.Fatal("new expression inherited retired state")
		}
		booleans.set(second, 9)
		if booleans.count() != 1 {
			t.Fatal("retired variable stayed counted", booleans.count())
		}
		if _, found := booleans.lookup(retired); found || context.relevant.get(retired) {
			t.Fatal("retired handle reads the new record")
		}
		booleans.del(retired)
		if variable, found := booleans.lookup(second); !found || variable != 9 {
			t.Fatal("deleting a retired handle hit the new expression")
		}
	}
	if !reused {
		t.Skip("manager did not reuse a slot")
	}
}

func TestExprTableRejectsForeignManagerWrites(t *testing.T) {
	first, second := astTestManager(t), astTestManager(t)
	context, err := NewSMTCheckerContext(first)
	astTestMust(t, err)
	mine, theirs := smtRelevancyBool(t, first, "mine"), smtRelevancyBool(t, second, "theirs")
	context.relevant.set(mine, true)
	if context.relevant.get(theirs) {
		t.Fatal("foreign handle read a local record")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("foreign write was accepted")
		}
	}()
	context.relevant.set(theirs, true)
}

func TestExprTableAllListsLiveEnodesInSlotOrder(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "table-all", 6)
	nodes := f.s.context.enodes.all()
	if len(nodes) != f.s.context.enodes.count() || len(nodes) != len(f.base) {
		t.Fatal("enode listing size", len(nodes), f.s.context.enodes.count(), len(f.base))
	}
	for i := 1; i < len(nodes); i++ {
		if nodes[i-1].owner.Slot() >= nodes[i].owner.Slot() {
			t.Fatal("enodes not in slot order")
		}
	}
	f.s.context.enodes.del(f.base[0])
	if f.s.context.enodes.get(f.base[0]) != nil || len(f.s.context.enodes.all()) != len(f.base)-1 {
		t.Fatal("deleted enode still listed")
	}
	f.s.context.enodes.set(f.base[0], nodes[0])
}

// Lookups on the search path must be allocation-free.
func TestExprTableLookupsDoNotAllocate(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "table-allocs", 16)
	context, h := f.s.context, f.base[3]
	if allocs := testing.AllocsPerRun(100, func() {
		context.assignments.set(h, lbool.True)
		_ = context.enodes.get(h)
		_ = context.relevant.get(h)
		_, _ = f.s.boolVar.lookup(h)
	}); allocs != 0 {
		t.Fatalf("%v allocations per lookup round", allocs)
	}
}

func TestDeterministicOrderHelpersSort(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "order-helpers", 8)
	set := map[ast.ASTHandle]struct{}{}
	for _, h := range f.base {
		set[h] = struct{}{}
	}
	sorted := smtSortedHandleSet(set)
	if len(sorted) != len(f.base) {
		t.Fatal("lost handles", len(sorted))
	}
	for i := 1; i < len(sorted); i++ {
		if smtCompareHandles(sorted[i-1], sorted[i]) >= 0 {
			t.Fatal("not sorted", sorted)
		}
	}
	nodes := map[*SMTEnode]ast.ASTHandle{nil: {}}
	for _, h := range f.base {
		nodes[f.s.context.enodes.get(h)] = h
	}
	roots := smtSortedEnodeKeys(nodes)
	if len(roots) != len(f.base) {
		t.Fatal("nil enode must be skipped", len(roots))
	}
	for i := 1; i < len(roots); i++ {
		if smtCompareHandles(roots[i-1].owner, roots[i].owner) >= 0 {
			t.Fatal("enodes not sorted")
		}
	}
	first, second := f.base[0], f.base[1]
	keys := smtSortedAUFKeys(map[smtModelFinderAUFKey]*smtModelFinderAUFNode{{second, 0}: nil, {first, 2}: nil, {first, 1}: nil})
	if keys[0] != (smtModelFinderAUFKey{first, 1}) || keys[1] != (smtModelFinderAUFKey{first, 2}) || keys[2] != (smtModelFinderAUFKey{second, 0}) {
		t.Fatal("AUF keys", keys)
	}
}
