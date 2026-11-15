package ast

import (
	"testing"
)

func TestSortDependencyOrderRetainedSourceTraversal(t *testing.T) {
	// Handles stand in for distinct live sort objects. The omitted dependency
	// is a built-in sort that decl_collector did not add to m_sorts. Each
	// handle's slot serves as its hash, so m_deps' table order is a, b, c, d.
	a := ASTHandle{origin: 1, slot: 1, generation: 1}
	b := ASTHandle{origin: 1, slot: 2, generation: 1}
	c := ASTHandle{origin: 1, slot: 3, generation: 1}
	d := ASTHandle{origin: 1, slot: 4, generation: 1}
	builtin := ASTHandle{origin: 1, slot: 5, generation: 1}
	var order sortDependencyOrder
	order.insert(a, a.slot, []ASTHandle{b, builtin})
	order.insert(b, b.slot, []ASTHandle{c})
	order.insert(c, c.slot, nil)
	order.insert(d, d.slot, []ASTHandle{b})
	order.insert(a, a.slot, []ASTHandle{b, builtin}) // repeated collected sort
	want := []ASTHandle{c, b, a, d}
	if got := order.order(); !equalSortHandles(got, want) {
		t.Fatalf("dependency-first order = %v, want %v", got, want)
	}
	if got := order.order(); !equalSortHandles(got, want) {
		t.Fatalf("repeat order = %v, want %v", got, want)
	}
}

func TestSortDependencyOrderCycleAndUncollectedRoot(t *testing.T) {
	a := ASTHandle{origin: 2, slot: 1, generation: 1}
	b := ASTHandle{origin: 2, slot: 2, generation: 1}
	c := ASTHandle{origin: 2, slot: 3, generation: 1}
	var order sortDependencyOrder
	order.insert(a, a.slot, []ASTHandle{b})
	order.insert(b, b.slot, []ASTHandle{a, c})
	// c is not inserted and must not appear in the result.
	if got, want := order.order(), []ASTHandle{b, a}; !equalSortHandles(got, want) {
		t.Fatalf("source cycle traversal = %v, want %v", got, want)
	}
}

func TestSortDependencyOrderCollectsActualSortParameters(t *testing.T) {
	m := astTestManager(t)
	a, e := m.MkSort(astTestSymbol(t, m, "A"), ASTSortInfo{})
	astTestMust(t, e)
	astTestPin(t, m, a)
	mkSort := func(name string, children []ASTHandle) ASTHandle {
		t.Helper()
		ps := make([]ASTParameter, 0, len(children)+1)
		for _, child := range children {
			p, e := m.NewParameterAST(child)
			astTestMust(t, e)
			ps = append(ps, p)
		}
		value, e := m.NewParameterInt(9)
		astTestMust(t, e)
		ps = append(ps, value)
		info, e := m.NewSortInfo(NullFamilyID, 0, ps, InfiniteSortSize(), false)
		astTestMust(t, e)
		h, e := m.MkSort(astTestSymbol(t, m, name), info)
		astTestMust(t, e)
		astTestPin(t, m, h)
		astTestMust(t, m.DelSortInfo(&info))
		for i := range ps {
			astTestMust(t, m.DelParameter(&ps[i]))
		}
		return h
	}
	b := mkSort("B", []ASTHandle{a})
	expr, e := m.MkVar(0, a)
	astTestMust(t, e)
	astTestPin(t, m, expr)
	c := mkSort("C", []ASTHandle{a, expr, b, a})
	var collected sortDependencyOrder
	for _, h := range []ASTHandle{c, b, a} {
		astTestMust(t, collected.insertASTSort(m, h))
	}
	if got, want := collected.order(), []ASTHandle{a, b, c}; !equalSortHandles(got, want) {
		t.Fatalf("real sort dependency order = %v, want %v", got, want)
	}
	var missing sortDependencyOrder
	astTestMust(t, missing.insertASTSort(m, b))
	if got, want := missing.order(), []ASTHandle{b}; !equalSortHandles(got, want) {
		t.Fatalf("uncollected dependency entered output: %v, want %v", got, want)
	}
}

func equalSortHandles(a, b []ASTHandle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
