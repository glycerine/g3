package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTUTVPILinearizationNormalizationAndRefusals(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "utvpi-x"), real)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "utvpi-y"), real)
	astTestMust(t, err)
	z, err := m.MkConst(astTestSymbol(t, m, "utvpi-z"), real)
	astTestMust(t, err)
	numeral := func(text string) ast.ASTHandle {
		n, err := m.ArithmeticRealNumeralText(text)
		astTestMust(t, err)
		return n
	}
	app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
		n, err := m.MkFamilyApp(family, op, nil, args)
		astTestMust(t, err)
		return n
	}
	l, err := newSMTUTVPILinearizer(m)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "utvpi-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	for _, tc := range []struct {
		name        string
		left, right ast.ASTHandle
		want        map[ast.ASTHandle]int64
		weight      string
		accepted    bool
	}{
		{"difference", app(smtArithOpSub, x, y), numeral("2/3"), map[ast.ASTHandle]int64{x: 1, y: -1}, "-2/3", true},
		{"cancellation", app(smtArithOpSub, x, x), numeral("0"), map[ast.ASTHandle]int64{}, "0", true},
		{"scaled-cancellation", app(smtArithOpMul, numeral("2"), app(smtArithOpAdd, x, y)), app(smtArithOpAdd, x, y), map[ast.ASTHandle]int64{x: 1, y: 1}, "0", true},
		{"rational-multiplier", app(smtArithOpMul, app(smtArithOpAdd, x, x), numeral("1/2")), ast.ASTHandle{}, map[ast.ASTHandle]int64{x: 1}, "0", true},
		{"negative-offset", app(smtArithOpUMinus, x), app(smtArithOpAdd, y, numeral("5/2")), map[ast.ASTHandle]int64{x: -1, y: -1}, "-5/2", true},
		{"three-variables", app(smtArithOpAdd, x, y, z), numeral("0"), nil, "", false},
		{"non-unit", app(smtArithOpAdd, x, x), numeral("0"), nil, "", false},
		{"nonlinear", app(smtArithOpMul, x, y), numeral("0"), nil, "", false},
		{"function-application", fx, numeral("0"), nil, "", false},
		{"reuse-after-refusal", x, numeral("7/3"), map[ast.ASTHandle]int64{x: 1}, "-7/3", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accepted, err := l.linearize(tc.left, tc.right)
			astTestMust(t, err)
			if accepted != tc.accepted {
				t.Fatal("UTVPI fragment classification", accepted)
			}
			if !accepted {
				return
			}
			weight, ok := new(big.Rat).SetString(tc.weight)
			if !ok || testQCmp(&l.weight, weight) != 0 || len(l.terms) != len(tc.want) {
				t.Fatal("UTVPI normalization changed the polynomial")
			}
			for _, term := range l.terms {
				coefficient, ok := tc.want[term.expression]
				if !ok || testQCmp(&term.coefficient, big.NewRat(coefficient, 1)) != 0 {
					t.Fatal("UTVPI coefficient", term)
				}
			}
		})
	}
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	i, err := m.MkConst(astTestSymbol(t, m, "utvpi-i"), integer)
	astTestMust(t, err)
	cast := app(smtArithOpToReal, i)
	accepted, err := l.linearize(cast, numeral("1/2"))
	astTestMust(t, err)
	if !accepted || len(l.terms) != 1 || l.terms[0].expression != i || testQCmp(&l.weight, big.NewRat(-1, 2)) != 0 {
		t.Fatal("UTVPI to_real traversal lost the original integer variable")
	}
}
