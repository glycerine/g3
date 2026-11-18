package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTTheoryOptimizationLinearAndNumeralRecognition(t *testing.T) {
	m := astTestManager(t)
	arithmetic, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	util, err := NewSMTTheoryOptimizationUtil(m)
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	real, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "opt-linear-x"), real)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "opt-linear-y"), real)
	astTestMust(t, err)
	negative, err := m.MkFamilyApp(arithmetic, smtArithOpUMinus, nil, []ast.ASTHandle{one})
	astTestMust(t, err)
	cast, err := m.MkFamilyApp(arithmetic, smtArithOpToInt, nil, []ast.ASTHandle{negative})
	astTestMust(t, err)
	if numeral, err := util.IsNumeral(cast); err != nil || !numeral {
		t.Fatal("wrapped numeral", numeral, err)
	}
	if numeral, err := util.IsNumeral(x); err != nil || numeral {
		t.Fatal("uninterpreted constant numeral", numeral, err)
	}
	product, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{cast, x})
	astTestMust(t, err)
	sum, err := m.MkFamilyApp(arithmetic, smtArithOpAdd, nil, []ast.ASTHandle{product, y, product})
	astTestMust(t, err)
	if linear, err := util.IsLinear(sum); err != nil || !linear {
		t.Fatal("linear arithmetic DAG", linear, err)
	}
	// AUDIT.md smtcore_11-F6: is_mul matches only binary products.
	ternary, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{cast, x, y})
	astTestMust(t, err)
	if linear, err := util.IsLinear(ternary); err != nil || linear {
		t.Fatal("n-ary product accepted as linear", linear, err)
	}
	nonlinear, err := m.MkFamilyApp(arithmetic, smtArithOpMul, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	if linear, err := util.IsLinear(nonlinear); err != nil || linear {
		t.Fatal("nonlinear product", linear, err)
	}
	division, err := m.MkFamilyApp(arithmetic, smtArithOpDiv, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	if linear, err := util.IsLinear(division); err != nil || linear {
		t.Fatal("unsupported arithmetic operator", linear, err)
	}
	boolean, err := m.MkTrue()
	astTestMust(t, err)
	if linear, err := util.IsLinear(boolean); err != nil || !linear {
		t.Fatal("foreign-family leaf", linear, err)
	}
	variable, err := m.MkVar(0, real)
	astTestMust(t, err)
	if linear, err := util.IsLinear(variable); err != nil || linear {
		t.Fatal("non-application", linear, err)
	}
}
