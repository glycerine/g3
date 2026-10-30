package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleManagerEnsureAndCheckBooleanApplication(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	application, err := f.rules.EnsureBooleanApplication(f.head)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&application)) }()
	applicationHandle, err := f.manager.RefGet(application)
	ruleMust(t, err)
	if applicationHandle != f.head {
		t.Fatal("application identity was not preserved")
	}
	quantifier := f.quantified(t, true, f.head)
	wrapped, err := f.rules.EnsureBooleanApplication(quantifier)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&wrapped)) }()
	wrappedHandle, err := f.manager.RefGet(wrapped)
	ruleMust(t, err)
	if op, e := f.manager.GetDeclKind(wrappedHandle); e != nil || op != ast.ASTOpEq && op != ast.ASTOpIff {
		t.Fatal("quantifier wrapper", op, e)
	}
	ruleMust(t, f.rules.CheckApplication(wrappedHandle))
	if err = f.rules.CheckApplication(quantifier); !errors.Is(err, base.ErrRuleFormula) {
		t.Fatal("non-application accepted", err)
	}
	if _, err = f.rules.EnsureBooleanApplication(f.x); !errors.Is(err, base.ErrRuleFormula) {
		t.Fatal("non-Boolean expression accepted", err)
	}
}
