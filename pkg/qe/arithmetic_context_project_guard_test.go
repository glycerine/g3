package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticProjectContextRejectsNonArithmeticVariable(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	boolean := f.constant("arith-context-bool", f.boolean)
	fmMust(t, f.context.AddVariable(boolean))
	truth := f.own(f.r.manager.MkTrue())
	formula, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&formula)
	if _, e = f.plugin.ProjectContextVariablesNNFComplete(f.model, formula, f.r.config); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal(e)
	}
}
