package tactic

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestTacticGoalNativeSlicesGrowCopyUpdateAndRelease(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	const count = 129
	var forms, proofs []ast.ASTHandle
	var deps []ast.ASTDependency
	var before []uint32
	for i := 0; i < count; i++ {
		f := goalTestAtom(t, m, fmt.Sprintf("native-goal-%d", i))
		pr, e := m.MkProofAsserted(f)
		astTestMust(t, e)
		astTestPin(t, m, pr)
		d, e := m.DependencyLeaf(f)
		astTestMust(t, e)
		astTestMust(t, m.DependencyInc(d))
		t.Cleanup(func() { astTestMust(t, m.DependencyDec(d)) })
		forms, proofs, deps = append(forms, f), append(proofs, pr), append(deps, d)
		rc, e := m.GetRefCount(f)
		astTestMust(t, e)
		before = append(before, rc)
	}
	_, e := m.MkTrue()
	astTestMust(t, e)
	_, e = m.MkFalse()
	astTestMust(t, e)
	g := goalTestNew(t, m, true, true)
	copy := goalTestNew(t, m, true, true)
	astTestMust(t, copy.AssertWithProof(forms[count-1], proofs[count-1], deps[count-1]))
	used := m.Budget().Used()
	ownerBytes := g.ownerBytes + copy.ownerBytes
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	for i := range forms {
		astTestMust(t, g.AssertWithProof(forms[i], proofs[i], deps[i]))
	}
	astTestMust(t, g.CopyTo(copy))
	astTestMust(t, g.CopyTo(copy)) // Replacing a snapshot must release its old pins.
	astTestMust(t, g.CopyTo(g))
	// The copy shares the arrays (parray_manager::copy): one pin each.
	for i, f := range forms {
		rc, e := m.GetRefCount(f)
		astTestMust(t, e)
		if rc != before[i]+1 {
			t.Fatal("shared formula pins", i, rc, before[i])
		}
		rc, e = m.GetRefCount(proofs[i])
		astTestMust(t, e)
		if rc != 2 {
			t.Fatal("shared proof pins", i, rc)
		}
		rc, e = m.DependencyRefCount(deps[i])
		astTestMust(t, e)
		if rc != 2 {
			t.Fatal("shared dependency pins", i, rc)
		}
	}
	astTestMust(t, g.Update(0, forms[count-1], proofs[count-1], deps[count-1]))
	goalTestForms(t, copy, forms...)
	pr, e := copy.Proof(0)
	astTestMust(t, e)
	d, e := copy.Dependency(0)
	astTestMust(t, e)
	if pr != proofs[0] || d != deps[0] {
		t.Fatal("snapshot proof/dependency changed", pr, d)
	}
	astTestMust(t, g.Reset())
	// The replaced formula stays pinned by the older version the copy holds.
	if rc, e := m.GetRefCount(forms[0]); e != nil || rc != before[0]+1 {
		t.Fatal("copy lost the replaced formula", rc, e)
	}
	goalTestForms(t, copy, forms...)
	astTestMust(t, copy.shrink(64))
	goalTestForms(t, copy, forms[:64]...)
	astTestMust(t, g.Close())
	astTestMust(t, copy.Close())
	for i, f := range forms {
		rc, e := m.GetRefCount(f)
		astTestMust(t, e)
		if rc != before[i] {
			t.Fatal("closed goals kept a formula pin", i, rc, before[i])
		}
		rc, e = m.GetRefCount(proofs[i])
		astTestMust(t, e)
		if rc != 1 {
			t.Fatal("closed goals kept a proof pin", i, rc)
		}
	}
	if m.Budget().Used() != used-ownerBytes {
		// Owner headers are released on close; native slice backing is uncharged.
		t.Fatal("unexpected arena change during native slice operations")
	}
}

// Source: src/test/ast.cpp:115-131 (tst5). The source expr_array operations
// are represented by the retained Goal's concrete owning formula slice.
func TestASTSourceExprArraySnapshotAssertions(t *testing.T) {
	m := astTestManager(t)
	a1 := goalTestAtom(t, m, "ast-array-a1")
	a2 := goalTestAtom(t, m, "ast-array-a2")
	arr1 := goalTestNew(t, m, false, false)
	arr2 := goalTestNew(t, m, false, false)
	arr3 := goalTestNew(t, m, false, false)
	astTestMust(t, arr1.Assert(a1))
	astTestMust(t, arr1.Assert(a2))
	astTestMust(t, arr1.CopyTo(arr2))
	astTestMust(t, arr2.shrink(1))
	astTestMust(t, arr2.CopyTo(arr3))
	astTestMust(t, arr3.Update(0, a2, ast.ASTHandle{}, ast.ASTDependency{}))
	goalTestForms(t, arr1, a1, a2)
	goalTestForms(t, arr2, a1)
	goalTestForms(t, arr3, a2)
}

func TestTacticGoalNativeSlicesKeepActualPublicationPrefix(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	f := goalTestAtom(t, m, "goal-prefix")
	pr, e := m.MkProofAsserted(f)
	astTestMust(t, e)
	astTestPin(t, m, pr)
	other := astTestManager(t)
	foreign, e := other.DependencyLeaf(goalTestAtom(t, other, "foreign-goal-core"))
	astTestMust(t, e)
	astTestMust(t, other.DependencyInc(foreign))
	defer func() { astTestMust(t, other.DependencyDec(foreign)) }()
	g := goalTestNew(t, m, true, true)
	if e = g.pushBack(f, pr, foreign); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("expected real dependency guard", e)
	}
	goalTestForms(t, g, f)
	got, e := g.Proof(0)
	astTestMust(t, e)
	if got != pr || int(g.dependencies.size()) != 0 {
		t.Fatal("formula/proof publication must precede rejected core", got, g.dependencies)
	}
	astTestMust(t, g.Close())
	if rc, e := m.GetRefCount(pr); e != nil || rc != 1 {
		t.Fatal("prefix proof cleanup", rc, e)
	}
}
