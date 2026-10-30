package qe_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticAssignSubstituteCompleteRejectsNilValue(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	truth := f.pin(f.manager.MkTrue())
	r, e := f.manager.NewRef(truth)
	pluginMust(t, e)
	defer f.manager.DelRef(&r)
	if e = f.plugin.AssignSubstituteComplete(f.contains, r, nil, nil); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
}
