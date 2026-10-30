package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type datalogBackgroundFixture struct {
	manager *ast.ASTManager
	context *base.DatalogContext
	first   ast.ASTHandle
	second  ast.ASTHandle
}

func newDatalogBackgroundFixture(t *testing.T) *datalogBackgroundFixture {
	t.Helper()
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	first, err := f.manager.MkTrue()
	ruleMust(t, err)
	second, err := f.manager.MkFalse()
	ruleMust(t, err)
	return &datalogBackgroundFixture{manager: f.manager, context: context, first: first, second: second}
}

func TestDatalogContextBackgroundAssertionsPreserveInsertionOrder(t *testing.T) {
	f := newDatalogBackgroundFixture(t)
	ruleMust(t, f.context.AssertExpression(f.first))
	ruleMust(t, f.context.AssertExpression(f.second))
	if count, err := f.context.BackgroundAssertionCount(); err != nil || count != 2 {
		t.Fatal("assertion count", count, err)
	}
	for i, want := range []ast.ASTHandle{f.first, f.second} {
		got, err := f.context.BackgroundAssertion(i)
		if err != nil || got != want {
			t.Fatal("assertion order", i, got, err)
		}
	}
}
