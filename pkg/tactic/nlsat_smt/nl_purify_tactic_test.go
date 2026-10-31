package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestNLPurifyTacticReuseAfterCleanup(t *testing.T) {
	m := astTestManager(t)
	_, _, config := nlModelEvaluatorFixture(t, m)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "lifecycle-x")
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "lifecycle-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	h, err := m.MkApp(decl, []ast.ASTHandle{x})
	fx := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.ArithmeticRealNumeralText("0")
	zero := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticGE(fx, one)
	positive := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticLE(fx, zero)
	negative := analysisKeep(t, m, h, err)
	engine, err := NewNLPurifyTactic(m, config.Context, config.Globals, nil, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, engine.Close()) }()
	run := func(unsat bool) {
		t.Helper()
		goal, err := tactic.NewGoal(m, false, true, false)
		astTestMust(t, err)
		defer func() { astTestMust(t, goal.Close()) }()
		astTestMust(t, goal.AssertWithProof(positive, ast.ASTHandle{}, ast.ASTDependency{}))
		if unsat {
			astTestMust(t, goal.AssertWithProof(negative, ast.ASTHandle{}, ast.ASTDependency{}))
		}
		var result tactic.TacticResult
		defer func() { astTestMust(t, result.Close()) }()
		astTestMust(t, engine.Apply(goal, &result))
		if unsat && !result.DecidedUnsat() || !unsat && (!result.DecidedSat() || result.ModelConverter == nil) {
			t.Fatal("tactic outcome", unsat, result.Goals)
		}
	}
	run(false)
	astTestMust(t, engine.Cleanup())
	run(true)
	astTestMust(t, engine.Cleanup())
	run(false)
}
