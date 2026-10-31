package api

// Source: src/test/arith_rewriter.cpp arithmetic and theory rewrites.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestArithmeticRewriterSourceExamples(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.RealSort())
	num := func(s string) ast.ASTHandle { return f.pin(f.c.Numeral(s, sort)) }
	x, y := f.pin(f.c.Constant("x", sort)), f.pin(f.c.Constant("y", sort))
	zeroProduct := f.pin(f.m.MkArithmeticMul(num("0"), num("-3")))
	ref, err := f.c.Simplify(zeroProduct)
	astTestMust(t, err)
	value, err := f.m.RefGet(ref)
	astTestMust(t, err)
	if n, ok, err := f.c.NumeralInt64(value); err != nil || !ok || n != 0 {
		t.Fatal("zero product", n, ok, err)
	}
	astTestMust(t, f.m.DelRef(&ref))
	mul := func(a, b ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkArithmeticMul(a, b)) }
	add := func(a, b ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkArithmeticAdd(a, b)) }
	sub := func(a, b ast.ASTHandle) ast.ASTHandle { return f.pin(f.m.MkArithmeticSub(a, b)) }
	first := f.pin(f.m.MkArithmeticLE(add(add(mul(mul(num("1.3"), x), y), mul(mul(num("2.3"), y), y)), sub(sub(num("1.1"), x), x)), num("2.2")))
	second := f.pin(f.c.Equal(add(add(num("4"), num("3")), add(sub(mul(mul(num("3"), x), x), mul(num("5"), y)), y)), num("0")))
	for i, formula := range []ast.ASTHandle{first, second} {
		rewritten, err := f.c.Simplify(formula)
		astTestMust(t, err)
		h, err := f.m.RefGet(rewritten)
		astTestMust(t, err)
		for _, xv := range []int64{-2, 0, 3} {
			for _, yv := range []int64{-1, 0, 4} {
				replaced, err := f.c.Substitute(h, []ast.ASTHandle{x, y}, []ast.ASTHandle{f.pin(f.c.IntNumeral(xv, sort)), f.pin(f.c.IntNumeral(yv, sort))})
				astTestMust(t, err)
				r, err := f.m.RefGet(replaced)
				astTestMust(t, err)
				simplified, err := f.c.Simplify(r)
				astTestMust(t, err)
				v, err := f.m.RefGet(simplified)
				astTestMust(t, err)
				got, err := f.c.BooleanValue(v)
				astTestMust(t, err)
				want := 13*xv*yv+23*yv*yv+11-20*xv <= 22
				if i == 1 {
					want = 7+3*xv*xv-4*yv == 0
				}
				if got == lbool.Undef || (got == lbool.True) != want {
					t.Fatal("rewritten formula", i, xv, yv, got, want)
				}
				astTestMust(t, f.m.DelRef(&simplified))
				astTestMust(t, f.m.DelRef(&replaced))
			}
		}
		astTestMust(t, f.m.DelRef(&rewritten))
	}
}
