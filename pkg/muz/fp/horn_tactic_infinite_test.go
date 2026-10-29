package fp_test

// Source: src/muz/fp/horn_tactic.cpp verify through the infinite-domain PDR
// engine selected by the fixedpoint auto-configuration path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
)

func hornInfiniteMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestHornTacticSelectsPDRForInfiniteDomainGoal(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, command.Close()) })
	manager, err := command.Manager()
	hornInfiniteMust(t, err)
	globals, err := command.GlobalParameters()
	hornInfiniteMust(t, err)
	boolean, err := manager.MkBoolSort()
	hornInfiniteMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	hornInfiniteMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	hornInfiniteMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	hornInfiniteMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern(name)
		hornInfiniteMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		hornInfiniteMust(t, x)
		return declaration
	}
	p, q := declare("horn-infinite-P"), declare("horn-infinite-Q")
	pZero, err := manager.MkApp(p, []ast.ASTHandle{zero})
	hornInfiniteMust(t, err)
	qZero, err := manager.MkApp(q, []ast.ASTHandle{zero})
	hornInfiniteMust(t, err)
	step, err := manager.MkImplies(pZero, qZero)
	hornInfiniteMust(t, err)
	query, err := manager.MkNot(qZero)
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	goal, err := tactic.NewGoal(manager, true, false, false)
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, goal.Close()) })
	for _, formula := range []ast.ASTHandle{pZero, step, query} {
		hornInfiniteMust(t, goal.Assert(formula))
	}
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, horn.IncRef())
	t.Cleanup(func() { hornInfiniteMust(t, horn.DecRef()) })
	result := &tactic.TacticResult{}
	t.Cleanup(func() { hornInfiniteMust(t, result.Close()) })
	hornInfiniteMust(t, horn.Apply(goal, result))
	if !result.DecidedUnsat() {
		t.Fatal("infinite-domain Horn reachability was not decided unsat")
	}
	proof, err := result.Goals[0].Proof(0)
	hornInfiniteMust(t, err)
	hyper, err := manager.IsProofKind(proof, ast.ASTPrHyperResolve)
	hornInfiniteMust(t, err)
	asserted, err := manager.IsProofKind(proof, ast.ASTPrAsserted)
	hornInfiniteMust(t, err)
	if !hyper && !asserted {
		t.Fatal("infinite-domain Horn tactic omitted the PDR proof trace")
	}
}

func TestHornTacticReturnsPDRModelForInfiniteDomainGoal(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, command.Close()) })
	manager, err := command.Manager()
	hornInfiniteMust(t, err)
	globals, err := command.GlobalParameters()
	hornInfiniteMust(t, err)
	boolean, err := manager.MkBoolSort()
	hornInfiniteMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	hornInfiniteMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	hornInfiniteMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	hornInfiniteMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern(name)
		hornInfiniteMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		hornInfiniteMust(t, x)
		return declaration
	}
	p, q := declare("horn-model-P"), declare("horn-model-Q")
	pZero, err := manager.MkApp(p, []ast.ASTHandle{zero})
	hornInfiniteMust(t, err)
	qZero, err := manager.MkApp(q, []ast.ASTHandle{zero})
	hornInfiniteMust(t, err)
	query, err := manager.MkNot(qZero)
	hornInfiniteMust(t, err)
	goal, err := tactic.NewGoal(manager, false, true, false)
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, goal.Close()) })
	for _, formula := range []ast.ASTHandle{pZero, query} {
		hornInfiniteMust(t, goal.Assert(formula))
	}
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, horn.IncRef())
	t.Cleanup(func() { hornInfiniteMust(t, horn.DecRef()) })
	result := &tactic.TacticResult{}
	t.Cleanup(func() { hornInfiniteMust(t, result.Close()) })
	hornInfiniteMust(t, horn.Apply(goal, result))
	if !result.DecidedSat() || result.ModelConverter == nil {
		t.Fatal("infinite-domain Horn non-reachability omitted its model converter")
	}
	var converted tactic.ModelRef
	t.Cleanup(func() { hornInfiniteMust(t, converted.Close()) })
	hornInfiniteMust(t, result.ModelConverter.ApplyModel(&converted, 0))
	modelCore, err := converted.Get()
	hornInfiniteMust(t, err)
	if modelCore == nil {
		t.Fatal("PDR Horn model converter returned no model")
	}
}
