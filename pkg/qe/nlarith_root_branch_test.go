package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func nonlinearQuadratic(t *testing.T, f *arithmeticQEFixture) (*qe.NonlinearArithmeticUtil, *ast.ContainsApp, *qe.NLPolynomial, ast.ASTHandle) {
	t.Helper()
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	minusOne := f.realNumeral(t, "-1")
	x2 := f.pin(f.manager.MkArithmeticMul(f.rx, f.rx))
	polyExpr := f.pin(f.manager.MkArithmeticAdd(x2, minusOne))
	p, ok, err := u.Decompose(contains, polyExpr)
	pluginMust(t, err)
	if !ok {
		t.Fatal("quadratic decomposition")
	}
	return u, contains, p, polyExpr
}

func TestNLRootComparisonAndEpsilonEliminateVariable(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, contains, p, _ := nonlinearQuadratic(t, f)
	defer func() { pluginMust(t, p.Close()); pluginMust(t, contains.Close()); pluginMust(t, u.Close()) }()
	zero := f.realNumeral(t, "0")
	two := f.realNumeral(t, "2")
	four := f.realNumeral(t, "4")
	root := qe.NLRootForm{A: zero, Multiplier: 1, C: four, D: two}
	for _, comparison := range []qe.NLComparison{qe.NLComparisonLT, qe.NLComparisonLE, qe.NLComparisonEQ, qe.NLComparisonNE} {
		result, err := u.RootCompare(p, root, comparison)
		pluginMust(t, err)
		if found, err := contains.Check(result); err != nil || found {
			t.Fatal("root comparison retained x", comparison, found, err)
		}
	}
	epsilon, err := u.PlusEpsilonCompare(p, root, qe.NLComparisonLT)
	pluginMust(t, err)
	if found, err := contains.Check(epsilon); err != nil || found {
		t.Fatal("epsilon comparison retained x", found, err)
	}
	definition, err := u.RootExpression(root)
	pluginMust(t, err)
	if found, err := contains.Check(definition); err != nil || found {
		t.Fatal("root definition retained x", found, err)
	}
}

func TestNLMinusInfinityParityEliminatesVariable(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, contains, p, _ := nonlinearQuadratic(t, f)
	defer func() { pluginMust(t, p.Close()); pluginMust(t, contains.Close()); pluginMust(t, u.Close()) }()
	for _, comparison := range []qe.NLComparison{qe.NLComparisonLT, qe.NLComparisonLE, qe.NLComparisonEQ, qe.NLComparisonNE} {
		result, err := u.MinusInfinityCompare(p, comparison)
		pluginMust(t, err)
		if found, err := contains.Check(result); err != nil || found {
			t.Fatal("infinity result retained x", comparison, found, err)
		}
	}
}
