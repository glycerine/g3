package cmd_context

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestDebugExpressionOwnershipReplacementAndReset(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("debug-owned-expression")
	if err != nil {
		t.Fatal(err)
	}
	expression, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.StoreDebugExpression("saved", expression); err != nil {
		t.Fatal(err)
	}
	if err = m.DelRef(&ref); err != nil {
		t.Fatal(err)
	}
	stored, err := c.DebugExpression("saved")
	if err != nil || stored != expression {
		t.Fatal(stored, err)
	}
	if err = c.StoreDebugExpression("saved", stored); err != nil {
		t.Fatal(err)
	}
	truth, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = c.StoreDebugExpression("saved", truth); err != nil {
		t.Fatal(err)
	}
	if m.IsLiveHandle(expression) {
		t.Fatal("replaced expression was not released")
	}
	if err = c.StoreDebugExpression("invalid", sort); err == nil {
		t.Fatal("accepted a non-expression AST")
	}
	if err = c.Reset(); err != nil {
		t.Fatal(err)
	}
	if got, err := c.DebugExpression("saved"); err == nil || got != (ast.ASTHandle{}) {
		t.Fatal("reset kept named expression", got, err)
	}
}

func TestDebugExpressionFailedReplacementPreservesValue(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	original, err := m.MkTrue()
	contextMust(t, err)
	replacement, err := m.MkFalse()
	contextMust(t, err)
	contextMust(t, c.StoreDebugExpression("saved", original))
	var padding []ast.ASTRef
	defer func() {
		m.Budget().FailAfter(-1)
		for i := range padding {
			contextMust(t, m.DelRef(&padding[i]))
		}
	}()
	m.Budget().FailAfter(0)
	failed := false
	for i := 0; i < 100000; i++ {
		ref, e := m.NewRef(original)
		if e != nil {
			if !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatal(e)
			}
			failed = true
			break
		}
		padding = append(padding, ref)
	}
	if !failed {
		t.Fatal("did not reach a reference allocation frontier")
	}
	if err = c.StoreDebugExpression("saved", replacement); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("replacement failure", err)
	}
	m.Budget().FailAfter(-1)
	got, err := c.DebugExpression("saved")
	contextMust(t, err)
	if got != original {
		t.Fatal("failed replacement changed named expression")
	}
	contextMust(t, c.StoreDebugExpression("saved", replacement))
	got, err = c.DebugExpression("saved")
	contextMust(t, err)
	if got != replacement {
		t.Fatal("replacement did not succeed after retry")
	}
}
