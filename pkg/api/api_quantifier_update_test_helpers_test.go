package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type quantifierUpdateFixture struct {
	context *SMTContext
	manager *ast.ASTManager
	sort    ast.ASTHandle
	truth   ast.ASTHandle
	false   ast.ASTHandle
	bound   ast.ASTHandle
	pattern ast.ASTHandle
	value   ast.ASTHandle
}

func newQuantifierUpdateFixture(t *testing.T) *quantifierUpdateFixture {
	t.Helper()
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	f := &quantifierUpdateFixture{context: context, manager: manager}
	value, err := manager.MkBoolSort()
	f.sort = f.pin(t, value, err)
	value, err = manager.MkTrue()
	f.truth = f.pin(t, value, err)
	value, err = manager.MkFalse()
	f.false = f.pin(t, value, err)
	value, err = manager.MkVar(0, f.sort)
	f.bound = f.pin(t, value, err)
	value, err = manager.MkNot(f.bound)
	trigger := f.pin(t, value, err)
	value, err = manager.MkPattern([]ast.ASTHandle{trigger})
	f.pattern = f.pin(t, value, err)
	name, err := manager.Symbols.Intern("x")
	astTestMust(t, err)
	qid, err := manager.Symbols.Intern("source-qid")
	astTestMust(t, err)
	skid, err := manager.Symbols.Intern("source-skid")
	astTestMust(t, err)
	value, err = manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []usymbol.Symbol{name},
		Body: f.truth, Weight: 7, QID: qid, SkolemID: skid,
		Patterns: []ast.ASTHandle{f.pattern},
	})
	f.value = f.pin(t, value, err)
	return f
}

func (f *quantifierUpdateFixture) pin(t *testing.T, value ast.ASTHandle, err error) ast.ASTHandle {
	t.Helper()
	astTestMust(t, err)
	ref, err := f.manager.NewRef(value)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.manager.DelRef(&ref)) })
	return value
}

func (f *quantifierUpdateFixture) own(t *testing.T, ref ast.ASTRef, err error) ast.ASTHandle {
	t.Helper()
	astTestMust(t, err)
	value, err := f.manager.RefGet(ref)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.manager.DelRef(&ref)) })
	return value
}
