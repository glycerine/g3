package solver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type namedOwnerCore struct {
	namedCoreFixture
	assert func(ast.ASTHandle) error
}

func (c *namedOwnerCore) AssertFormula(h ast.ASTHandle) error { return c.assert(h) }
func namedOwnerConst(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	solverMust(t, e)
	symbol, e := m.Symbols.Intern(name)
	solverMust(t, e)
	h, e := m.MkConst(symbol, b)
	solverMust(t, e)
	ref, e := m.NewRef(h)
	solverMust(t, e)
	t.Cleanup(func() { solverMust(t, m.DelRef(&ref)) })
	return h
}
func TestNamedAssertionImplicationOwnedAcrossCoreAndErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			m := solverTestManager(t)
			formula := namedOwnerConst(t, m, "owned-na-formula")
			tag := namedOwnerConst(t, m, "owned-na-tag")
			stop := errors.New("core assertion stopped")
			core := &namedOwnerCore{}
			n, e := NewNamedAssertions(m, core)
			solverMust(t, e)
			t.Cleanup(func() { solverMust(t, n.Close()) })
			var implication ast.ASTHandle
			var rootRefs, formulaRefs, tagRefs uint32
			core.assert = func(h ast.ASTHandle) error {
				implication = h
				var err error
				rootRefs, err = m.GetRefCount(h)
				solverMust(t, err)
				formulaRefs, err = m.GetRefCount(formula)
				solverMust(t, err)
				tagRefs, err = m.GetRefCount(tag)
				solverMust(t, err)
				if len(n.assumptions) != 1 {
					t.Fatal("assumption not published before core call")
				}
				if fail {
					return stop
				}
				return nil
			}
			err := n.AssertTracked(formula, tag)
			if fail {
				if !errors.Is(err, stop) {
					t.Fatal("lost core error", err)
				}
			} else {
				solverMust(t, err)
			}
			if rootRefs != 1 || formulaRefs != 2 || tagRefs != 3 {
				t.Fatal("source new_t ownership missing at core call", rootRefs, formulaRefs, tagRefs)
			}
			if m.IsLiveHandle(implication) {
				t.Fatal("implication local survived core call")
			}
			refs, e := m.GetRefCount(formula)
			solverMust(t, e)
			if refs != 1 {
				t.Fatal("formula retained by ended local", refs)
			}
			refs, e = m.GetRefCount(tag)
			solverMust(t, e)
			if refs != 2 || len(n.assumptions) != 1 {
				t.Fatal("published assumption rolled back", refs)
			}
			solverMust(t, n.Close())
			refs, e = m.GetRefCount(tag)
			solverMust(t, e)
			if refs != 1 {
				t.Fatal("assumption member not retired", refs)
			}
		})
	}
}
func TestNamedAssertionNullTagBorrowsInput(t *testing.T) {
	m := solverTestManager(t)
	formula := namedOwnerConst(t, m, "borrowed-na-formula")
	core := &namedOwnerCore{}
	n, e := NewNamedAssertions(m, core)
	solverMust(t, e)
	t.Cleanup(func() { solverMust(t, n.Close()) })
	var got uint32
	core.assert = func(h ast.ASTHandle) error {
		if h != formula {
			t.Fatal("null-tag argument changed")
		}
		var e error
		got, e = m.GetRefCount(h)
		return e
	}
	m.Budget().FailAfter(0)
	err := n.AssertTracked(formula, ast.ASTHandle{})
	m.Budget().FailAfter(-1)
	solverMust(t, err)
	if got != 1 || len(n.assumptions) != 0 || len(n.pending) != 0 {
		t.Fatal("null-tag branch acquired source locals", got)
	}
}
