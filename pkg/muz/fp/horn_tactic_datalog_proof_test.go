package fp_test

// Source: src/muz/base/dl_engine_base.h engine_base::get_proof and
// src/muz/fp/horn_tactic.cpp horn::operator() reachable proof publication.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestHornTacticDatalogUsesSourceDefaultProof(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, command.Close()) })
	manager, err := command.Manager()
	fpMust(t, err)
	globals, err := command.GlobalParameters()
	fpMust(t, err)
	fpMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))

	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern(name)
		fpMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		application, x := manager.MkApp(declaration, nil)
		fpMust(t, x)
		return application
	}
	p := declare("horn-datalog-proof-p")
	q := declare("horn-datalog-proof-q")
	step, err := manager.MkImplies(p, q)
	fpMust(t, err)
	query, err := manager.MkNot(q)
	fpMust(t, err)

	goal, err := tactic.NewGoal(manager, true, false, false)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, goal.Close()) })
	for _, formula := range []ast.ASTHandle{p, step, query} {
		fpMust(t, goal.Assert(formula))
	}
	horn, err := fp.NewHornTactic(manager, globals.Context, globals)
	fpMust(t, err)
	fpMust(t, horn.IncRef())
	t.Cleanup(func() { fpMust(t, horn.DecRef()) })
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, horn.Apply(goal, result))
	if !result.DecidedUnsat() {
		t.Fatal("finite Horn reachability was not decided unsat")
	}
	proof, err := result.Goals[0].Proof(0)
	fpMust(t, err)
	asserted, err := manager.IsProofKind(proof, ast.ASTPrAsserted)
	fpMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fpMust(t, err)
	if !asserted || !manager.IsTrue(fact) {
		t.Fatal("Datalog did not publish engine_base asserted true proof", asserted, fact)
	}
	if result.ProofConverter == nil {
		t.Fatal("reachable Horn result omitted its proof converter")
	}
	converted, err := manager.NewRef(ast.ASTHandle{})
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&converted)) }()
	// The converter owns the proof independently of the mutated goal and engine.
	fpMust(t, goal.Reset())
	fpMust(t, result.ProofConverter.Convert(manager, nil, converted))
	got, err := manager.RefGet(converted)
	fpMust(t, err)
	if got != proof {
		t.Fatal("proof converter returned a different proof", got, proof)
	}
}
