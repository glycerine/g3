package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatalogContextBackgroundFormulaEmptySingletonAndConjunction(t *testing.T) {
	f := newDatalogBackgroundFixture(t)
	empty, err := f.context.BackgroundFormula()
	ruleMust(t, err)
	emptyHandle, err := f.manager.RefGet(empty)
	ruleMust(t, err)
	if !f.manager.IsTrue(emptyHandle) {
		t.Fatal("empty background is not true")
	}
	ruleMust(t, f.manager.DelRef(&empty))

	ruleMust(t, f.context.AssertExpression(f.first))
	single, err := f.context.BackgroundFormula()
	ruleMust(t, err)
	singleHandle, err := f.manager.RefGet(single)
	ruleMust(t, err)
	if singleHandle != f.first {
		t.Fatal("singleton background was rebuilt")
	}
	ruleMust(t, f.manager.DelRef(&single))

	ruleMust(t, f.context.AssertExpression(f.second))
	conjunction, err := f.context.BackgroundFormula()
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&conjunction)) }()
	handle, err := f.manager.RefGet(conjunction)
	ruleMust(t, err)
	if op, e := f.manager.GetDeclKind(handle); e != nil || op != ast.ASTOpAnd {
		t.Fatal("background conjunction", op, e)
	}
}
