package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// A quantifier instance whose first disjunct is already true is satisfied.
// Source checker::any_arg reads just that child; it neither copies the rest
// of the disjunction nor recreates its memoization tables per candidate.
func TestSMTCheckerSatisfiedGuardDoesNotAllocatePerInstance(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	guard, err := m.MkConst(astTestSymbol(t, m, "checker-guard"), boolean)
	astTestMust(t, err)
	args := []ast.ASTHandle{guard}
	for i := 0; i < 512; i++ {
		child, e := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("checker-child-%d", i)), boolean)
		astTestMust(t, e)
		args = append(args, child)
	}
	body, err := m.MkOrArgs(args)
	astTestMust(t, err)
	owned, err := m.NewRef(body)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&owned)) })
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	astTestMust(t, context.SetRelevant(guard, true))
	astTestMust(t, context.SetAssignment(guard, lbool.True))
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	allocations := testing.AllocsPerRun(20, func() {
		satisfied, e := checker.IsSatisfiable(body, nil)
		if e != nil || !satisfied {
			panic("satisfied guard was not recognized")
		}
	})
	if allocations != 0 {
		t.Fatalf("satisfied guard allocated %.0f times per instance; want 0", allocations)
	}
	// Reusing storage must not reuse truth values across different assignments.
	astTestMust(t, context.SetAssignment(guard, lbool.False))
	if satisfied, e := checker.IsSatisfiable(body, nil); e != nil || satisfied {
		t.Fatal("checker reused a stale satisfaction result", satisfied, e)
	}
	if len(checker.truthCache) != 0 || len(checker.enodeCache) != 0 || checker.bindings != nil {
		t.Fatal("checker retained results/bindings after a check")
	}
}

// Native checker::check memoizes only shared nodes (refcount > 1). Filling
// a memo table with unshared leaves adds work without avoiding any traversal.
func TestSMTCheckerMemoizesOnlySharedExpressions(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	guard, err := m.MkConst(astTestSymbol(t, m, "checker-shared-guard"), boolean)
	astTestMust(t, err)
	other, err := m.MkConst(astTestSymbol(t, m, "checker-shared-other"), boolean)
	astTestMust(t, err)
	body, err := m.MkOr(guard, other)
	astTestMust(t, err)
	root, err := m.NewRef(body)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
	if count, e := m.GetRefCount(body); e != nil || count != 1 {
		t.Fatal("expected unshared root", count, e)
	}
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	astTestMust(t, context.SetRelevant(guard, true))
	astTestMust(t, context.SetAssignment(guard, lbool.True))
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	satisfied, err := checker.check(body, true)
	astTestMust(t, err)
	if !satisfied || len(checker.truthCache) != 0 {
		t.Fatal("unshared expressions populated the truth cache", satisfied, len(checker.truthCache))
	}
	shared, err := m.NewRef(body)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&shared)) })
	satisfied, err = checker.check(body, true)
	astTestMust(t, err)
	if !satisfied || len(checker.truthCache) != 1 {
		t.Fatal("shared root was not memoized", satisfied, len(checker.truthCache))
	}
	checker.clear()
}

// Source get_enode_eq_to caches a null result even when core lookup stopped
// because a child had no enode. Otherwise a shared non-Boolean DAG is walked
// again for every predicate that mentions it.
func TestSMTCheckerMemoizesMissingEnodeInSharedTerm(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "checker-missing-sort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	innerFunction := smtQuickPredicate(t, m, "checker-missing-inner", sort, sort)
	outerFunction := smtQuickPredicate(t, m, "checker-missing-outer", sort, sort)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	inner, err := m.MkApp(innerFunction, []ast.ASTHandle{variable})
	astTestMust(t, err)
	term, err := m.MkApp(outerFunction, []ast.ASTHandle{inner})
	astTestMust(t, err)
	first, err := m.NewRef(term)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&first)) })
	second, err := m.NewRef(term)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&second)) })
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	owner, binding := smtEnodeConstant(t, m, 33000, sort, 0)
	astTestMust(t, context.RegisterEnode(binding, true))
	checker.bindings = []*SMTEnode{binding}
	// The variable has a valid binding, but inner(binding) has no enode yet.
	node, err := checker.enodeEqualTo(term)
	astTestMust(t, err)
	if node != nil {
		t.Fatal("missing inner application unexpectedly had an enode")
	}
	cached, found := checker.enodeCache[term]
	if !found || cached != nil {
		t.Fatal("shared term's missing-enode result was not memoized")
	}
	checker.clear()
	groundInner, err := m.MkApp(innerFunction, []ast.ASTHandle{owner})
	astTestMust(t, err)
	innerNode, err := NewSMTEnode(m, groundInner, context.enodes.get, SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(innerNode, true))
	ground, err := m.MkApp(outerFunction, []ast.ASTHandle{groundInner})
	astTestMust(t, err)
	parent, err := NewSMTEnode(m, ground, context.enodes.get, SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(parent, true))
	checker.bindings = []*SMTEnode{binding}
	node, err = checker.enodeEqualTo(term)
	astTestMust(t, err)
	if node != parent {
		t.Fatal("cleared negative cache hid the next instance's enode", node)
	}
	checker.clear()
}
