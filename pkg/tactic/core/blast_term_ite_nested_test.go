package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp first-argument greedy
// expansion followed by bounded BR_REWRITE3 revisits.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestBlastTermITEMultipleArgumentsExpandRecursively(t *testing.T) {
	f := newBlastTermITEFixture(t, false)
	name, err := f.m.Symbols.Intern("blast-binary")
	simplifyMust(t, err)
	binary, err := f.m.MkFuncDecl(name, []ast.ASTHandle{f.real, f.real}, f.real, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, f.m, binary)
	_, secondCond, _ := propagateAtoms(t, f.m)
	first, err := f.m.MkIte(f.cond, f.one, f.two)
	simplifyMust(t, err)
	second, err := f.m.MkIte(secondCond, f.one, f.two)
	simplifyMust(t, err)
	simplifyPin(t, f.m, first)
	simplifyPin(t, f.m, second)
	app, err := f.m.MkApp(binary, []ast.ASTHandle{first, second})
	simplifyMust(t, err)
	formula, err := f.m.MkEq(app, f.one)
	simplifyMust(t, err)
	goal := f.goal(t, formula, false, false)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	if f.runner.numFresh != 6 {
		t.Fatal("source first-argument expansion tree", f.runner.numFresh)
	}
	rewritten, err := goal.Form(0)
	simplifyMust(t, err)
	occursFirst, err := f.m.OccursExpr(first, rewritten)
	simplifyMust(t, err)
	occursSecond, err := f.m.OccursExpr(second, rewritten)
	simplifyMust(t, err)
	if occursFirst || occursSecond {
		t.Fatal("term ITE remained under application", occursFirst, occursSecond)
	}
}
