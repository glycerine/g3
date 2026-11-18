package rewriter

// Rewriting with numerals wider than an inline mpz must free every rational
// local: a second identical pass (whose result ASTs already exist) leaves
// the live mpz cell count unchanged.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithRewriterWideNumeralsFreeRationalLocals(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { _ = r.Close() })
	iSort, err := m.MkFamilySort(r.family, 1, nil)
	astTestMust(t, err)
	rSort, err := m.MkFamilySort(r.family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "wide-cells-x"), iSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "wide-cells-y"), rSort)
	astTestMust(t, err)
	wide := strings.Repeat("9", 60)
	a := arithRewriterNumeral(t, m, wide, true)
	b := arithRewriterNumeral(t, m, "-"+wide+"7", true)
	ar := arithRewriterNumeral(t, m, wide+"/7", false)
	br := arithRewriterNumeral(t, m, "3"+wide, false)
	two := arithRewriterNumeral(t, m, "2", true)
	ax, err := r.app(arithOpMul, a, x)
	astTestMust(t, err)
	bx, err := r.app(arithOpMul, b, x)
	astTestMust(t, err)
	ary, err := r.app(arithOpMul, ar, y)
	astTestMust(t, err)
	sum, err := r.app(arithOpAdd, ax, bx, a)
	astTestMust(t, err)
	rsum, err := r.app(arithOpAdd, ary, ar, y)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{x, y, a, b, ar, br, two, ax, bx, ary, sum, rsum} {
		astTestPin(t, m, h)
	}
	pass := func() {
		for _, c := range []struct {
			kind int32
			args []ast.ASTHandle
		}{
			{arithOpAdd, []ast.ASTHandle{a, b, ax, bx}},
			{arithOpAdd, []ast.ASTHandle{ary, ar, br, y}},
			{arithOpMul, []ast.ASTHandle{a, b, x}},
			{arithOpMul, []ast.ASTHandle{ar, br, y, y}},
			{arithOpSub, []ast.ASTHandle{sum, ax}},
			{arithOpLE, []ast.ASTHandle{sum, b}},
			{arithOpGE, []ast.ASTHandle{rsum, br}},
			{arithOpLE, []ast.ASTHandle{a, b}},
			{arithOpDiv, []ast.ASTHandle{ar, br}},
			{arithOpDiv, []ast.ASTHandle{ary, br}},
			{arithOpIDiv, []ast.ASTHandle{a, b}},
			{arithOpMod, []ast.ASTHandle{b, a}},
			{arithOpRem, []ast.ASTHandle{b, a}},
			{arithOpMod, []ast.ASTHandle{sum, a}},
			{arithOpPower, []ast.ASTHandle{a, two}},
			{arithOpPower, []ast.ASTHandle{br, two}},
			{arithOpToInt, []ast.ASTHandle{ar}},
			{arithOpToReal, []ast.ASTHandle{a}},
			{arithOpIsInt, []ast.ASTHandle{ar}},
			{arithOpUMinus, []ast.ASTHandle{b}},
		} {
			arithRewriterCore(t, r, c.kind, c.args...)
		}
		out := astTestPin(t, m, sum)
		_, err := r.MkEqCore(sum, b, out)
		astTestMust(t, err)
	}
	pass()
	z := m.NumeralService().Manager.Z
	before := z.StorageStats().LiveCells
	pass()
	if after := z.StorageStats().LiveCells; after != before {
		t.Fatal("wide-numeral rewrites leaked mpz cells", before, after)
	}
}
