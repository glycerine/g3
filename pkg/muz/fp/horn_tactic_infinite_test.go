package fp_test

// Source: src/muz/fp/horn_tactic.cpp verify through the infinite-domain PDR
// engine selected by the fixedpoint auto-configuration path.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
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

// AUDIT.md muz_fp_03-F3: a Boolean bound variable disjunct is classified with
// m.is_not / is_predicate, both false for a variable, so it becomes a negated
// body atom instead of aborting the tactic.
func TestHornTacticBooleanVariableDisjunct(t *testing.T) {
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
	name, err := manager.Symbols.Intern("horn-boolvar-p")
	hornInfiniteMust(t, err)
	p, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	hornInfiniteMust(t, err)
	// (forall ((x Int) (b Bool)) (=> (not (p x)) b))
	x, err := manager.MkVar(1, integer)
	hornInfiniteMust(t, err)
	b, err := manager.MkVar(0, boolean)
	hornInfiniteMust(t, err)
	px, err := manager.MkApp(p, []ast.ASTHandle{x})
	hornInfiniteMust(t, err)
	notPx, err := manager.MkNot(px)
	hornInfiniteMust(t, err)
	body, err := manager.MkImplies(notPx, b)
	hornInfiniteMust(t, err)
	xName, err := manager.Symbols.Intern("x")
	hornInfiniteMust(t, err)
	bName, err := manager.Symbols.Intern("b")
	hornInfiniteMust(t, err)
	formula, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, boolean}, Names: []usymbol.Symbol{xName, bName}, Body: body})
	hornInfiniteMust(t, err)
	goal, err := tactic.NewGoal(manager, false, false, false)
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, goal.Close()) })
	hornInfiniteMust(t, goal.Assert(formula))
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, horn.IncRef())
	t.Cleanup(func() { hornInfiniteMust(t, horn.DecRef()) })
	result := &tactic.TacticResult{}
	t.Cleanup(func() { hornInfiniteMust(t, result.Close()) })
	hornInfiniteMust(t, horn.Apply(goal, result))
	if !result.DecidedSat() {
		t.Fatal("Horn goal with a Boolean variable disjunct was not decided sat")
	}
}

// AUDIT.md muz_fp_03-F4: the horn tactic's datalog::context resolves its
// engine from the tactic params (context::configure_engine), so engine=datalog
// rejects an Int goal with the infinite-sort exception instead of auto-config
// selecting PDR.
func TestHornTacticHonorsEngineParameter(t *testing.T) {
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
	name, err := manager.Symbols.Intern("horn-engine-P")
	hornInfiniteMust(t, err)
	p, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	hornInfiniteMust(t, err)
	pZero, err := manager.MkApp(p, []ast.ASTHandle{zero})
	hornInfiniteMust(t, err)
	query, err := manager.MkNot(pZero)
	hornInfiniteMust(t, err)
	goal, err := tactic.NewGoal(manager, false, false, false)
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, goal.Close()) })
	for _, formula := range []ast.ASTHandle{pZero, query} {
		hornInfiniteMust(t, goal.Assert(formula))
	}
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, horn.IncRef())
	t.Cleanup(func() { hornInfiniteMust(t, horn.DecRef()) })
	value, err := globals.Context.NewValue()
	hornInfiniteMust(t, err)
	t.Cleanup(func() { hornInfiniteMust(t, globals.Context.Del(&value)) })
	key := "engine"
	datalog, err := manager.Symbols.Intern("datalog")
	hornInfiniteMust(t, err)
	hornInfiniteMust(t, globals.Context.SetSymbolName(&value, &key, datalog))
	hornInfiniteMust(t, horn.UpdateParams(&value))
	result := &tactic.TacticResult{}
	t.Cleanup(func() { hornInfiniteMust(t, result.Close()) })
	err = horn.Apply(goal, result)
	var exception *fp.HornTacticException
	if !errors.As(err, &exception) || !strings.HasPrefix(exception.Error(), "Rule contains infinite sorts") {
		t.Fatalf("engine=datalog Horn goal: %v, want the infinite-sort tactic exception", err)
	}
}
