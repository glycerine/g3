package core

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func splitClauseAtom(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	symbol, err := m.Symbols.Intern(name)
	simplifyMust(t, err)
	atom, err := m.MkConst(symbol, boolean)
	simplifyMust(t, err)
	simplifyPin(t, m, atom)
	return atom
}

func splitClauseGoal(t *testing.T, m *ast.ASTManager, proofs bool, clauses ...[]ast.ASTHandle) (*tactic.Goal, []ast.ASTHandle) {
	t.Helper()
	goal, err := tactic.NewGoal(m, proofs, false, false)
	simplifyMust(t, err)
	forms := make([]ast.ASTHandle, len(clauses))
	for i, literals := range clauses {
		forms[i], err = m.MkOrArgs(literals)
		simplifyMust(t, err)
		simplifyPin(t, m, forms[i])
		simplifyMust(t, goal.Assert(forms[i]))
	}
	return goal, forms
}

func TestSplitClauseSelectsFirstOrStrictlyLargestClause(t *testing.T) {
	for _, largest := range []bool{false, true} {
		t.Run(map[bool]string{false: "first", true: "largest"}[largest], func(t *testing.T) {
			m, context, _ := simplifyFixture(t)
			a := splitClauseAtom(t, m, "split-a")
			b := splitClauseAtom(t, m, "split-b")
			c := splitClauseAtom(t, m, "split-c")
			d := splitClauseAtom(t, m, "split-d")
			goal, forms := splitClauseGoal(t, m, false, []ast.ASTHandle{a, b}, []ast.ASTHandle{b, c, d})
			t.Cleanup(func() { simplifyMust(t, goal.Close()) })
			params := context.EmptyValue()
			if largest {
				name := "split_largest_clause"
				simplifyMust(t, context.SetBoolName(&params, &name, true))
			}
			runner, err := NewSplitClauseTactic(m, context, &params)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, runner.Close()) })
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			simplifyMust(t, runner.Apply(goal, &result))
			wantLiterals := []ast.ASTHandle{a, b}
			position := uint32(0)
			untouchedPosition := uint32(1)
			untouched := forms[1]
			if largest {
				wantLiterals = []ast.ASTHandle{b, c, d}
				position, untouchedPosition, untouched = 1, 0, forms[0]
			}
			if len(result.Goals) != len(wantLiterals) {
				t.Fatal("branch count", len(result.Goals), len(wantLiterals))
			}
			for i, branch := range result.Goals {
				got, getErr := branch.Form(position)
				simplifyMust(t, getErr)
				if got != wantLiterals[i] {
					t.Fatal("source literal order", i, got, wantLiterals[i])
				}
				other, getErr := branch.Form(untouchedPosition)
				simplifyMust(t, getErr)
				if other != untouched {
					t.Fatal("unselected clause changed", i, other, untouched)
				}
				depth, getErr := branch.Depth()
				simplifyMust(t, getErr)
				if depth != 1 {
					t.Fatal("branch depth", i, depth)
				}
			}
			if result.Goals[len(result.Goals)-1] != goal {
				t.Fatal("source reuses input for the final branch")
			}
		})
	}
}

func TestSplitClauseFailureAndRuntimeParameterUpdate(t *testing.T) {
	m, context, _ := simplifyFixture(t)
	a := splitClauseAtom(t, m, "split-failure-a")
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(a))
	runner, err := NewSplitClauseTactic(m, context, nil)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	err = runner.Apply(goal, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || err.Error() != "split-clause tactic failed, goal does not contain any clause" {
		t.Fatal("source tactic exception", err)
	}
	params := context.EmptyValue()
	name := "split_largest_clause"
	simplifyMust(t, context.SetBoolName(&params, &name, true))
	simplifyMust(t, runner.UpdateParams(&params))
	if !runner.largestClause {
		t.Fatal("parameter update was not published")
	}
}

func TestSplitClauseProofConverterClosesBranchHypotheses(t *testing.T) {
	m, context, _ := simplifyFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	a := splitClauseAtom(t, m, "split-proof-a")
	b := splitClauseAtom(t, m, "split-proof-b")
	goal, _ := splitClauseGoal(t, m, true, []ast.ASTHandle{a, b})
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	runner, err := NewSplitClauseTactic(m, context, nil)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if result.ProofConverter == nil || len(result.Goals) != 2 {
		t.Fatal("missing source split proof converter", result.ProofConverter, len(result.Goals))
	}
	falsehood, err := m.MkFalse()
	simplifyMust(t, err)
	branchProofs := make([]ast.ASTHandle, len(result.Goals))
	for i := range branchProofs {
		branchProofs[i], err = m.MkProofHypothesis(falsehood)
		simplifyMust(t, err)
	}
	out, err := m.NewRef(ast.ASTHandle{})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, m.DelRef(&out)) })
	simplifyMust(t, result.ProofConverter.Convert(m, branchProofs, out))
	proof, err := m.RefGet(out)
	simplifyMust(t, err)
	fact, err := m.GetProofFact(proof)
	simplifyMust(t, err)
	if fact != falsehood {
		t.Fatal("unit resolution did not reconstruct false", fact, falsehood)
	}
	if err = result.ProofConverter.Convert(m, branchProofs[:1], out); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source branch count assertion boundary", err)
	}
}
