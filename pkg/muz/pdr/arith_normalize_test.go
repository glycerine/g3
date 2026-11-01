package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestNormalizeArithmeticDividesRealComparisonByLeastCoefficient(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	defer func() {
		closureMust(t, transformer.Close())
		closureMust(t, owner.Close())
		closeBase()
	}()
	_, err := manager.RegisterArithmeticPlugin()
	closureMust(t, err)
	real, err := manager.ArithmeticRealNumeralText("0")
	closureMust(t, err)
	realSort, err := manager.GetSort(real)
	closureMust(t, err)
	xName, err := manager.Symbols.Intern("normalize-x")
	closureMust(t, err)
	yName, err := manager.Symbols.Intern("normalize-y")
	closureMust(t, err)
	x, err := manager.MkConst(xName, realSort)
	closureMust(t, err)
	y, err := manager.MkConst(yName, realSort)
	closureMust(t, err)
	two, err := manager.ArithmeticRealNumeralText("2")
	closureMust(t, err)
	four, err := manager.ArithmeticRealNumeralText("4")
	closureMust(t, err)
	six, err := manager.ArithmeticRealNumeralText("6")
	closureMust(t, err)
	twoX, err := manager.MkArithmeticMul(two, x)
	closureMust(t, err)
	fourY, err := manager.MkArithmeticMul(four, y)
	closureMust(t, err)
	left, err := manager.MkFamilyApp(mustArithmeticFamily(t, manager), 6, nil, []ast.ASTHandle{twoX, fourY})
	closureMust(t, err)
	formula, err := manager.MkFamilyApp(mustArithmeticFamily(t, manager), 2, nil, []ast.ASTHandle{left, six})
	closureMust(t, err)
	normalized, err := NormalizeArithmetic(manager, formula)
	closureMust(t, err)
	if normalized == formula {
		t.Fatal("comparison was not normalized")
	}
	zeroBound, err := manager.MkFamilyApp(mustArithmeticFamily(t, manager), 2, nil, []ast.ASTHandle{twoX, real})
	closureMust(t, err)
	unchanged, err := NormalizeArithmetic(manager, zeroBound)
	closureMust(t, err)
	if unchanged != zeroBound {
		t.Fatal("source zero coefficient must reset the divisor and suppress scaling")
	}
	nested, err := manager.MkNot(formula)
	closureMust(t, err)
	nested, err = NormalizeArithmetic(manager, nested)
	closureMust(t, err)
	child, err := manager.GetArg(nested, 0)
	closureMust(t, err)
	if child != normalized {
		t.Fatal("nested comparison did not receive the same normalization")
	}
	quantified, err := manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{realSort}, Names: []usymbol.Symbol{xName},
		Body: formula, Weight: 7, QID: xName, SkolemID: yName, NoPatterns: []ast.ASTHandle{formula},
	})
	closureMust(t, err)
	quantified, err = NormalizeArithmetic(manager, quantified)
	closureMust(t, err)
	body, err := manager.GetQuantifierBody(quantified)
	closureMust(t, err)
	if body != normalized {
		t.Fatal("quantifier body was not normalized")
	}
	annotation, err := manager.GetNoPattern(quantified, 0)
	closureMust(t, err)
	if annotation != formula {
		t.Fatal("normalizer rewrote the no-pattern annotation")
	}
	weight, err := manager.GetQuantifierWeight(quantified)
	closureMust(t, err)
	qid, skid, err := manager.GetQuantifierIDs(quantified)
	closureMust(t, err)
	if weight != 7 || qid != xName || skid != yName {
		t.Fatal("normalizer changed quantifier metadata", weight, qid, skid)
	}
	left, err = manager.GetArg(normalized, 0)
	closureMust(t, err)
	count, err := manager.GetNumArgs(left)
	closureMust(t, err)
	if count != 2 {
		t.Fatalf("normalized left side has %d terms, want 2", count)
	}
	first, err := manager.GetArg(left, 0)
	closureMust(t, err)
	if first != x {
		t.Fatalf("first normalized term = %v, want x=%v", first, x)
	}
	second, err := manager.GetArg(left, 1)
	closureMust(t, err)
	coefficient, err := manager.GetArg(second, 0)
	closureMust(t, err)
	if text, _, ok, err := manager.ArithmeticNumeralText(coefficient); err != nil || !ok || text != "2" {
		t.Fatalf("second normalized coefficient = %q, %v, %v", text, ok, err)
	}
	right, err := manager.GetArg(normalized, 1)
	closureMust(t, err)
	if text, _, ok, err := manager.ArithmeticNumeralText(right); err != nil || !ok || text != "3" {
		t.Fatalf("normalized bound = %q, %v, %v", text, ok, err)
	}
	strict, err := manager.MkFamilyApp(mustArithmeticFamily(t, manager), 4, nil, []ast.ASTHandle{twoX, six})
	closureMust(t, err)
	strict, err = NormalizeArithmetic(manager, strict)
	closureMust(t, err)
	if family, err := manager.GetFamilyID(strict); err != nil || family != 0 {
		t.Fatalf("strict comparison family = %d, %v", family, err)
	}
	if op, err := manager.GetDeclKind(strict); err != nil || op != ast.ASTOpNot {
		t.Fatalf("strict comparison rewrite = %d, %v", op, err)
	}
	ge, err := manager.GetArg(strict, 0)
	closureMust(t, err)
	if op, err := manager.GetDeclKind(ge); err != nil || op != 3 {
		t.Fatalf("strict comparison inner relation = %d, %v", op, err)
	}
	owned, err := manager.NewRef(formula)
	closureMust(t, err)
	closureMust(t, NormalizeArithmeticRef(manager, owned))
	published, err := manager.RefGet(owned)
	closureMust(t, err)
	if published != normalized {
		t.Fatal("owning normalizer did not publish the rewritten formula")
	}
	closureMust(t, manager.DelRef(&owned))
}

func mustArithmeticFamily(t *testing.T, manager *ast.ASTManager) ast.FamilyID {
	t.Helper()
	family, err := manager.ArithmeticFamilyID()
	closureMust(t, err)
	return family
}
