package fp_test

// Source: src/muz/base/dl_engine_base.h get_model (rel_context keeps the
// empty model), src/muz/base/dl_context.cpp get_model and
// src/muz/fp/horn_tactic.cpp verify model conversion (AUDIT.md muz_fp_02-F5).

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogModelIsEmpty(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, command.Close()) })
	manager, err := command.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern(name)
		fpMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		fpMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	relation := declare("datalog-model-relation", []ast.ASTHandle{boolean})
	omitted := declare("datalog-model-query", nil)
	fpMust(t, engine.AddTableFact(relation, []uint32{1}))
	status, err := engine.QueryPredicate(omitted)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("empty finite query was reachable", status)
	}
	empty, err := engine.Model(omitted)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, empty.Close()) })
	for _, declaration := range []ast.ASTHandle{relation, omitted} {
		if present, err := empty.Core().HasInterpretation(declaration); err != nil || present {
			t.Fatal("rel_context model must be empty", present, err)
		}
	}
}

func TestHornTacticReturnsEmptyDatalogModelForFiniteGoal(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, command.Close()) })
	manager, err := command.Manager()
	fpMust(t, err)
	globals, err := command.GlobalParameters()
	fpMust(t, err)
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		symbol, x := manager.Symbols.Intern(name)
		fpMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		application, x := manager.MkApp(declaration, nil)
		fpMust(t, x)
		return declaration, application
	}
	_, pApp := declare("horn-finite-model-P")
	_, qApp := declare("horn-finite-model-Q")
	query, err := manager.MkNot(qApp)
	fpMust(t, err)
	goal, err := tactic.NewGoal(manager, false, true, false)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, goal.Close()) })
	fpMust(t, goal.Assert(pApp))
	fpMust(t, goal.Assert(query))
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	fpMust(t, err)
	fpMust(t, horn.IncRef())
	t.Cleanup(func() { fpMust(t, horn.DecRef()) })
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, horn.Apply(goal, result))
	if !result.DecidedSat() || result.ModelConverter == nil {
		t.Fatal("finite Horn non-reachability omitted its model converter")
	}
	var converted tactic.ModelRef
	t.Cleanup(func() { fpMust(t, converted.Close()) })
	fpMust(t, result.ModelConverter.ApplyModel(&converted, 0))
	core, err := converted.Get()
	fpMust(t, err)
	if count, err := core.NumDeclarations(); err != nil || count != 0 {
		t.Fatal("datalog Horn model must be empty", count, err)
	}
}
