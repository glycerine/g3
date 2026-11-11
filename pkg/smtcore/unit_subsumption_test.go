package smtcore

// Source: src/smt/tactic/unit_subsumption_tactic.cpp prune_clauses.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestUnitSubsumptionUsesIncrementalPropagationAndDeletionOrder(t *testing.T) {
	manager := astTestManager(t)
	params, err := uparams.NewParamsContext(manager.Symbols, manager.NumeralService(), uparams.ParamsConfig{Budget: manager.Budget()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = params.Close() }()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	a, err := manager.MkFreshConst("unit-subsumption-a", boolean)
	if err != nil {
		t.Fatal(err)
	}
	b, err := manager.MkFreshConst("unit-subsumption-b", boolean)
	if err != nil {
		t.Fatal(err)
	}
	weak, err := manager.MkOrArgs([]ast.ASTHandle{a, b})
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	us, err := NewUnitSubsumptionTactic(manager, nil, SMTSolverConfig{Params: params})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = us.Close() }()
	g, err := tactic.NewGoal(manager, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = g.Close() }()
	for _, f := range []ast.ASTHandle{weak, a, a, truth} {
		if err = g.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	result := &tactic.TacticResult{}
	defer func() { _ = result.Close() }()
	if err = us.Apply(g, result); err != nil {
		t.Fatal(err)
	}
	got, err := result.Goals[0].AppendFormulas(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != a {
		t.Fatalf("unit subsumption retained %v, want final duplicate of a", got)
	}
}
