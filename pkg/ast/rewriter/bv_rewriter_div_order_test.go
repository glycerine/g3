package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The reference z3 is a GCC build, which evaluates call arguments right to
// left. mk_ite(mk_eq(y, 0), op0(x), op_i(x, y)) therefore creates op_i,
// then op0, then the zero numeral and finally the equality.
func TestBVRewriterDivZeroIteConstructionOrder(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	key := "hi_div0"
	astTestMust(t, c.SetBoolName(&local, &key, false))
	p, e := NewBVRewriter(m, c, g, &local)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	id := func(h ast.ASTHandle) uint32 {
		t.Helper()
		v, err := m.GetASTID(h)
		astTestMust(t, err)
		return v
	}
	for width, rewrite := range map[uint32]func(lhs, rhs ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error){
		11: p.RewriteUnsignedDivision,
		12: p.RewriteSignedDivision,
		13: p.RewriteSignedRemainder,
		14: p.RewriteSignedModulo,
		15: p.RewriteUnsignedRemainder,
	} {
		sort, err := m.BitVectorSort(width)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "div-order-x"), sort)
		astTestMust(t, err)
		astTestPin(t, m, x)
		y, err := m.MkConst(astTestSymbol(t, m, "div-order-y"), sort)
		astTestMust(t, err)
		astTestPin(t, m, y)
		_, err = rewrite(x, y, out)
		astTestMust(t, err)
		result, err := m.RefGet(out)
		astTestMust(t, err)
		// Keep each result alive so later rows cannot recycle its ids.
		astTestPin(t, m, result)
		cond, err := m.GetArg(result, 0)
		astTestMust(t, err)
		yes, err := m.GetArg(result, 1)
		astTestMust(t, err)
		no, err := m.GetArg(result, 2)
		astTestMust(t, err)
		zero, err := m.GetArg(cond, 1)
		astTestMust(t, err)
		if !(id(no) < id(yes) && id(yes) < id(zero) && id(zero) < id(cond)) {
			t.Fatal("width", width, "order", id(no), id(yes), id(zero), id(cond))
		}
	}
}
