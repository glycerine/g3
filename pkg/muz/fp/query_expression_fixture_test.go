package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
)

type queryExpressionFixture struct {
	t       *testing.T
	context *cmd_context.Context
	manager *ast.ASTManager
	engine  *fp.PropositionalEngine
	refs    []ast.ASTRef
}

func newQueryExpressionFixture(t *testing.T) *queryExpressionFixture {
	context, err := cmd_context.NewStandaloneContext()
	queryMust(t, err)
	manager, err := context.Manager()
	queryMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	queryMust(t, err)
	f := &queryExpressionFixture{t: t, context: context, manager: manager, engine: engine}
	t.Cleanup(func() {
		queryMust(t, engine.Close())
		for i := len(f.refs) - 1; i >= 0; i-- {
			queryMust(t, manager.DelRef(&f.refs[i]))
		}
		queryMust(t, context.Close())
	})
	return f
}

func (f *queryExpressionFixture) pin(handle ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	queryMust(f.t, err)
	reference, err := f.manager.NewRef(handle)
	queryMust(f.t, err)
	f.refs = append(f.refs, reference)
	return handle
}

func (f *queryExpressionFixture) predicate(name string, domain ...ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	f.t.Helper()
	boolean := f.pin(f.manager.MkBoolSort())
	symbol, err := f.manager.Symbols.Intern(name)
	queryMust(f.t, err)
	declaration := f.pin(f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
	queryMust(f.t, f.engine.RegisterPredicate(declaration))
	if len(domain) != 0 {
		return declaration, ast.ASTHandle{}
	}
	return declaration, f.pin(f.manager.MkApp(declaration, nil))
}

func queryMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
