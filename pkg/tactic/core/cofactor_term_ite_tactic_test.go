package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestCofactorTermITEEliminatesArithmeticChoice(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	// mk_simplified_app builds every plugin rewriter; reg_decl_plugins.
	if err := m.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	runner, err := NewCofactorTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	condition, _, _ := propagateAtoms(t, m)
	numerals := make([]ast.ASTHandle, 4)
	for i, text := range []string{"1", "2", "3", "4"} {
		numerals[i], err = m.ArithmeticRealNumeralText(text)
		simplifyMust(t, err)
		simplifyPin(t, m, numerals[i])
	}
	choice, err := m.MkIte(condition, numerals[0], numerals[1])
	simplifyMust(t, err)
	simplifyPin(t, m, choice)
	sum, err := m.MkArithmeticAdd(choice, numerals[2])
	simplifyMust(t, err)
	simplifyPin(t, m, sum)
	formula, err := m.MkEq(sum, numerals[3])
	simplifyMust(t, err)
	simplifyPin(t, m, formula)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	rewritten, err := goal.Form(0)
	simplifyMust(t, err)
	if rewritten != condition {
		t.Fatal("cofactor result", rewritten, "want condition", condition)
	}
}

func TestCofactorTermITENegatedConditionPropagatesAtom(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	// mk_simplified_app builds every plugin rewriter; reg_decl_plugins.
	if err := m.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	runner, err := NewCofactorTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	p, _, _ := propagateAtoms(t, m)
	notP, err := m.MkNot(p)
	simplifyMust(t, err)
	simplifyPin(t, m, notP)
	numerals := make([]ast.ASTHandle, 5)
	for i, text := range []string{"1", "2", "3", "4", "5"} {
		numerals[i], err = m.ArithmeticRealNumeralText(text)
		simplifyMust(t, err)
		simplifyPin(t, m, numerals[i])
	}
	left, err := m.MkIte(notP, numerals[0], numerals[1])
	simplifyMust(t, err)
	simplifyPin(t, m, left)
	right, err := m.MkIte(p, numerals[2], numerals[3])
	simplifyMust(t, err)
	simplifyPin(t, m, right)
	sum, err := m.MkArithmeticAdd(left, right)
	simplifyMust(t, err)
	simplifyPin(t, m, sum)
	formula, err := m.MkEq(sum, numerals[4])
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	rewritten, err := goal.Form(0)
	simplifyMust(t, err)
	if !m.IsTrue(rewritten) {
		t.Fatal("negated cofactor did not propagate its atom", rewritten)
	}
}

func TestCofactorTermITEGuardsProofsAndCores(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	runner, err := NewCofactorTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	for _, flags := range [][2]bool{{true, false}, {false, true}} {
		goal, e := tactic.NewGoal(m, flags[0], false, flags[1])
		simplifyMust(t, e)
		var result tactic.TacticResult
		if e = runner.Apply(goal, &result); e == nil {
			t.Fatal("cofactor tactic accepted guarded goal", flags)
		}
		simplifyMust(t, result.Close())
		simplifyMust(t, goal.Close())
	}
}
