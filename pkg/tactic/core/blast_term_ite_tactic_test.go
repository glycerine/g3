package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp end-to-end hoisting.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type blastTermITEFixture struct {
	m      *ast.ASTManager
	runner *BlastTermITETactic
	cond   ast.ASTHandle
	real   ast.ASTHandle
	one    ast.ASTHandle
	two    ast.ASTHandle
	f      ast.ASTHandle
}

func newBlastTermITEFixture(t *testing.T, proofs bool) blastTermITEFixture {
	t.Helper()
	m, c, g, services := solveEqFixture(t)
	if proofs {
		simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	}
	runner, err := NewBlastTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	cond, _, _ := propagateAtoms(t, m)
	real, err := m.MkFamilySort(func() ast.FamilyID { id, e := m.RegisterArithmeticPlugin(); simplifyMust(t, e); return id }(), 0, nil)
	simplifyMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	simplifyMust(t, err)
	two, err := m.ArithmeticRealNumeralText("2")
	simplifyMust(t, err)
	name, err := m.Symbols.Intern("blast-f")
	simplifyMust(t, err)
	f, err := m.MkFuncDecl(name, []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	for _, h := range []ast.ASTHandle{one, two, f} {
		simplifyPin(t, m, h)
	}
	return blastTermITEFixture{m: m, runner: runner, cond: cond, real: real, one: one, two: two, f: f}
}

func (f blastTermITEFixture) goal(t *testing.T, formula ast.ASTHandle, proofs, cores bool) *tactic.Goal {
	t.Helper()
	g, err := tactic.NewGoal(f.m, proofs, false, cores)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, g.Close()) })
	if proofs {
		pr, err := f.m.MkProofAsserted(formula)
		simplifyMust(t, err)
		var dep ast.ASTDependency
		if cores {
			dep, err = f.m.DependencyLeaf(formula)
			simplifyMust(t, err)
		}
		simplifyMust(t, g.AssertWithProof(formula, pr, dep))
	} else {
		simplifyMust(t, g.Assert(formula))
	}
	return g
}

func TestBlastTermITEHoistsApplicationArgument(t *testing.T) {
	f := newBlastTermITEFixture(t, false)
	choice, err := f.m.MkIte(f.cond, f.one, f.two)
	simplifyMust(t, err)
	app, err := f.m.MkApp(f.f, []ast.ASTHandle{choice})
	simplifyMust(t, err)
	formula, err := f.m.MkEq(app, f.one)
	simplifyMust(t, err)
	goal := f.goal(t, formula, false, false)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	left, err := f.m.MkApp(f.f, []ast.ASTHandle{f.one})
	simplifyMust(t, err)
	right, err := f.m.MkApp(f.f, []ast.ASTHandle{f.two})
	simplifyMust(t, err)
	leftEq, err := f.m.MkEq(left, f.one)
	simplifyMust(t, err)
	rightEq, err := f.m.MkEq(right, f.one)
	simplifyMust(t, err)
	want, err := f.m.MkIte(f.cond, leftEq, rightEq)
	simplifyMust(t, err)
	got, err := goal.Form(0)
	simplifyMust(t, err)
	if got != want || f.runner.numFresh != 2 {
		t.Fatal("term ITE hoist", got, want, f.runner.numFresh)
	}
	depth, err := goal.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("missing depth increment", depth)
	}
}
