package core

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func occfConstraint(t *testing.T, m *ast.ASTManager, name string, argument ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	symbol, err := m.Symbols.Intern(name)
	simplifyMust(t, err)
	decl, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	term, err := m.MkApp(decl, []ast.ASTHandle{argument})
	simplifyMust(t, err)
	simplifyPin(t, m, term)
	return term
}

func occfConstraintCount(t *testing.T, runner *OCCFTactic, formula ast.ASTHandle) int {
	t.Helper()
	count, err := runner.manager.GetNumArgs(formula)
	simplifyMust(t, err)
	result := 0
	for i := uint32(0); i < count; i++ {
		argument, getErr := runner.manager.GetArg(formula, i)
		simplifyMust(t, getErr)
		literal, getErr := runner.isLiteral(argument)
		simplifyMust(t, getErr)
		if !literal {
			result++
		}
	}
	return result
}

func TestOCCFReusesAuxiliaryAndLeavesOneConstraintPerClause(t *testing.T) {
	m, _, _, services := solveEqFixture(t)
	a := splitClauseAtom(t, m, "occf-a")
	b := splitClauseAtom(t, m, "occf-b")
	p := occfConstraint(t, m, "occf-p", a)
	q := occfConstraint(t, m, "occf-q", b)
	r := occfConstraint(t, m, "occf-r", a)
	first, err := m.MkOrArgs([]ast.ASTHandle{p, q, a})
	simplifyMust(t, err)
	second, err := m.MkOrArgs([]ast.ASTHandle{r, q, b})
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(first))
	simplifyMust(t, goal.Assert(second))
	runner, err := NewOCCFTactic(m, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if len(result.Goals) != 1 || result.Goals[0] != goal {
		t.Fatal("source result goal", len(result.Goals))
	}
	size, err := goal.Size()
	simplifyMust(t, err)
	if size != 3 {
		t.Fatal("one shared auxiliary should add one implication", size)
	}
	for i := uint32(0); i < size; i++ {
		formula, getErr := goal.Form(i)
		simplifyMust(t, getErr)
		if count := occfConstraintCount(t, runner, formula); count > 1 {
			t.Fatal("clause retained multiple constraints", i, count)
		}
	}
	firstOut, err := goal.Form(0)
	simplifyMust(t, err)
	lastArg, err := m.GetArg(firstOut, 2)
	simplifyMust(t, err)
	if lastArg != p {
		t.Fatal("first source constraint was not kept last", lastArg, p)
	}
	firstAux, err := m.GetArg(firstOut, 0)
	simplifyMust(t, err)
	secondOut, err := goal.Form(1)
	simplifyMust(t, err)
	secondAux, err := m.GetArg(secondOut, 0)
	simplifyMust(t, err)
	if firstAux != secondAux {
		t.Fatal("constraint auxiliary was not reused", firstAux, secondAux)
	}
	depth, err := goal.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("goal depth", depth)
	}
}

func TestOCCFModelConverterAndProofGuard(t *testing.T) {
	for _, models := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-model", true: "model"}[models], func(t *testing.T) {
			m, _, _, services := solveEqFixture(t)
			a := splitClauseAtom(t, m, "occf-model-a")
			p := occfConstraint(t, m, "occf-model-p", a)
			q := occfConstraint(t, m, "occf-model-q", a)
			clause, err := m.MkOr(p, q)
			simplifyMust(t, err)
			goal, err := tactic.NewGoal(m, false, models, false)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, goal.Close()) })
			simplifyMust(t, goal.Assert(clause))
			runner, err := NewOCCFTactic(m, services)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, runner.Close()) })
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			simplifyMust(t, runner.Apply(goal, &result))
			if (result.ModelConverter != nil) != models {
				t.Fatal("source model filtering boundary", result.ModelConverter, models)
			}
		})
	}
	m, _, _, services := solveEqFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := splitClauseAtom(t, m, "occf-proof-a")
	p := occfConstraint(t, m, "occf-proof-p", a)
	q := occfConstraint(t, m, "occf-proof-q", a)
	clause, err := m.MkOr(p, q)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, true, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(clause))
	runner, err := NewOCCFTactic(m, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	var result tactic.TacticResult
	err = runner.Apply(goal, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || exception.Error() != "occf does not support proof production" {
		t.Fatal("proof-generation guard", err)
	}
}

func TestOCCFChecksCancellationBeforeCooperation(t *testing.T) {
	m, _, _, services := solveEqFixture(t)
	a := splitClauseAtom(t, m, "occf-cancel-a")
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(a))
	runner, err := NewOCCFTactic(m, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	m.ResourceLimit().IncCancel()
	var result tactic.TacticResult
	err = runner.Apply(goal, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || exception.Error() != tactic.TacticCanceledMessage {
		t.Fatal("source cancellation checkpoint", err)
	}
}
