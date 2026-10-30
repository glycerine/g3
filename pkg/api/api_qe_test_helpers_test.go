package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func apiQEBoolVariable(t *testing.T, f *smtAPIFixture, name string) ast.ASTHandle {
	t.Helper()
	sort := f.pin(f.m.MkBoolSort())
	symbol, err := f.m.Symbols.Intern(name)
	astTestMust(t, err)
	return f.pin(f.m.MkConst(symbol, sort))
}

func apiQECheckedModel(t *testing.T, f *smtAPIFixture, assertion ast.ASTHandle) *modelOwner {
	t.Helper()
	astTestMust(t, f.c.Assert(assertion))
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatalf("model status %v", status)
	}
	core, err := f.c.Model()
	astTestMust(t, err)
	owner := &modelOwner{core: core}
	t.Cleanup(func() {
		if owner.core != nil {
			astTestMust(t, owner.core.DecRef())
		}
	})
	return owner
}

type modelOwner struct{ core *model.ModelCore }
