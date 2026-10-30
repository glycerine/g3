package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
)

type hornLifecycleFixture struct {
	manager *ast.ASTManager
	command *cmd_context.Context
	tactic  *fp.HornTactic
	goal    *tactic.Goal
}

func newHornLifecycleFixture(t *testing.T, simplify bool) *hornLifecycleFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := command.Manager()
	fpMust(t, err)
	globals, err := command.GlobalParameters()
	fpMust(t, err)
	var horn *fp.HornTactic
	if simplify {
		horn, err = fp.NewHornSimplifyTactic(manager, globals.Context, globals)
	} else {
		horn, err = fp.NewHornTactic(manager, globals.Context, globals)
	}
	fpMust(t, err)
	fpMust(t, horn.IncRef())
	goal, err := tactic.NewGoal(manager, false, true, false)
	fpMust(t, err)
	t.Cleanup(func() {
		fpMust(t, goal.Close())
		fpMust(t, horn.DecRef())
		fpMust(t, command.Close())
	})
	return &hornLifecycleFixture{manager: manager, command: command, tactic: horn, goal: goal}
}

func (f *hornLifecycleFixture) predicate(t *testing.T, name string) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	fpMust(t, err)
	symbol, err := f.manager.Symbols.Intern(name)
	fpMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	application, err := f.manager.MkApp(declaration, nil)
	fpMust(t, err)
	return declaration, application
}

func (f *hornLifecycleFixture) populate(t *testing.T) {
	t.Helper()
	_, p := f.predicate(t, "horn-life-P")
	_, q := f.predicate(t, "horn-life-Q")
	rule, err := f.manager.MkImplies(p, q)
	fpMust(t, err)
	query, err := f.manager.MkNot(q)
	fpMust(t, err)
	fpMust(t, f.goal.Assert(p))
	fpMust(t, f.goal.Assert(rule))
	fpMust(t, f.goal.Assert(query))
}
