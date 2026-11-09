package fpa

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func fpaTacticMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestFPA2BVTacticRewritesGoalAndPublishesSingleResult(t *testing.T) {
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	fpaTacticMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	fpaTacticMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	fpaTacticMust(t, err)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	fpaTacticMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	fpaTacticMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	fpaTacticMust(t, err)
	fpaTacticMust(t, globals.InitializeSource())
	pass, err := NewPass(manager, context, globals, nil)
	fpaTacticMust(t, err)
	goal, err := tactic.NewGoal(manager, false, true, false)
	fpaTacticMust(t, err)
	floatSort, err := manager.FloatingPointSort(8, 24)
	fpaTacticMust(t, err)
	name, err := symbols.Intern("tactic-float")
	fpaTacticMust(t, err)
	value, err := manager.MkConst(name, floatSort)
	fpaTacticMust(t, err)
	family, err := manager.RegisterFloatingPointPlugin()
	fpaTacticMust(t, err)
	formula, err := manager.MkFamilyApp(family, int32(ast.FPAIsNaN), nil, []ast.ASTHandle{value})
	fpaTacticMust(t, err)
	fpaTacticMust(t, goal.Assert(formula))
	result, err := pass.Apply(goal)
	fpaTacticMust(t, err)
	if result.Goal != goal || result.ModelConverter == nil {
		t.Fatal("unexpected tactic result", result.Goal, result.ModelConverter)
	}
	rewritten, err := goal.Form(0)
	fpaTacticMust(t, err)
	if rewritten == formula {
		t.Fatal("floating-point formula was not rewritten")
	}
	depth, err := goal.Depth()
	fpaTacticMust(t, err)
	if depth != 1 {
		t.Fatal("goal depth", depth)
	}
	// The pass updates the caller's goal in place; the caller closes it.
	fpaTacticMust(t, goal.Close())
	fpaTacticMust(t, result.ModelConverter.Close())
	fpaTacticMust(t, pass.Close())
	adapter, err := NewFPA2BVTactic(manager, context, globals, nil)
	fpaTacticMust(t, err)
	fpaTacticMust(t, adapter.IncRef())
	adaptedGoal, err := tactic.NewGoal(manager, false, true, false)
	fpaTacticMust(t, err)
	adaptedSort, err := manager.FloatingPointSort(8, 24)
	fpaTacticMust(t, err)
	adaptedValue, err := manager.MkConst(name, adaptedSort)
	fpaTacticMust(t, err)
	adaptedFormula, err := manager.MkFamilyApp(family, int32(ast.FPAIsNaN), nil, []ast.ASTHandle{adaptedValue})
	fpaTacticMust(t, err)
	fpaTacticMust(t, adaptedGoal.Assert(adaptedFormula))
	var adapted tactic.TacticResult
	fpaTacticMust(t, adapter.Apply(adaptedGoal, &adapted))
	if len(adapted.Goals) != 1 || adapted.Goals[0] != adaptedGoal || adapted.ModelConverter == nil {
		t.Fatal("unexpected common tactic result", adapted.Goals, adapted.ModelConverter)
	}
	fpaTacticMust(t, adapted.Close())
	fpaTacticMust(t, adaptedGoal.Close())
	fpaTacticMust(t, adapter.DecRef())
	fpaTacticMust(t, globals.Close())
	fpaTacticMust(t, catalog.Close())
	fpaTacticMust(t, context.Close())
	fpaTacticMust(t, manager.Close())
	fpaTacticMust(t, numerals.Close())
	fpaTacticMust(t, symbols.Close())
	if budget.Used() != 0 {
		t.Fatalf("retained %d bytes", budget.Used())
	}
}
