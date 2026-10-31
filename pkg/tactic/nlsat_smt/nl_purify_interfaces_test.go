package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestNLPurifyInterfaceRewriteSeparatesAndSharesDefinitions(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "interface-x")
	h, err := m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticAdd(x, one)
	sum := analysisKeep(t, m, h, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "interface-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	h, err = m.MkApp(decl, []ast.ASTHandle{sum})
	fx := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticAdd(fx, fx)
	root := analysisKeep(t, m, h, err)
	r, err := newNLInterfaceRewriter(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	value, err := r.rewrite(root)
	astTestMust(t, err)
	if len(r.newReals) != 2 || len(r.nonlinearDefinitions) != 1 || len(r.smtDefinitions) != 1 {
		t.Fatal("interface definitions", len(r.newReals), len(r.nonlinearDefinitions), len(r.smtDefinitions))
	}
	left, err := m.GetArg(value, 0)
	astTestMust(t, err)
	right, err := m.GetArg(value, 1)
	astTestMust(t, err)
	if left != right || left != r.newReals[1] {
		t.Fatal("shared arithmetic interface", left, right, r.newReals)
	}
	firstRight, err := m.GetArg(r.nonlinearDefinitions[0], 1)
	astTestMust(t, err)
	if firstRight != sum {
		t.Fatal("nonlinear definition", firstRight, sum)
	}
	secondRight, err := m.GetArg(r.smtDefinitions[0], 1)
	astTestMust(t, err)
	arg, err := m.GetArg(secondRight, 0)
	astTestMust(t, err)
	if arg != r.newReals[0] {
		t.Fatal("SMT definition argument", arg, r.newReals[0])
	}
	again, err := r.rewrite(root)
	astTestMust(t, err)
	if again != value || len(r.newReals) != 2 {
		t.Fatal("cached rewrite", again, value, len(r.newReals))
	}
}

func TestNLPurifyInterfaceGoalRewritePublishesDefinitions(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x := analysisConst(t, m, real, "goal-interface-x")
	h, err := m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticAdd(x, one)
	sum := analysisKeep(t, m, h, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "goal-interface-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	h, err = m.MkApp(decl, []ast.ASTHandle{sum})
	fx := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticGE(fx, one)
	root := analysisKeep(t, m, h, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	nonlinear, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, nonlinear.Close()) }()
	astTestMust(t, goal.AssertWithProof(root, ast.ASTHandle{}, ast.ASTDependency{}))
	r, err := newNLInterfaceRewriter(m)
	astTestMust(t, err)
	astTestMust(t, r.rewriteGoal(goal, nonlinear))
	if len(r.smtDefinitions) != 1 || len(r.nonlinearDefinitions) != 1 {
		t.Fatal("definitions", len(r.smtDefinitions), len(r.nonlinearDefinitions))
	}
	if n, err := nonlinear.Size(); err != nil || n != 1 {
		t.Fatal("nonlinear goal", n, err)
	}
	rewritten, err := goal.Form(0)
	astTestMust(t, err)
	if rewritten == root {
		t.Fatal("mixed goal was not rewritten")
	}
	astTestMust(t, r.Close())
	if _, err := goal.Form(0); err != nil {
		t.Fatal("goal lost rewritten root", err)
	}
	if _, err := nonlinear.Form(0); err != nil {
		t.Fatal("nonlinear goal lost definition", err)
	}
}
