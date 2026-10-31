package api

// Source: src/test/polynorm.cpp, two polynomial normalization examples.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPolynomialNormalizationSourceExamples(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.IntegerSort())
	x := f.pin(f.c.Constant("x", sort))
	y := f.pin(f.c.Constant("y", sort))
	num := func(n int64) ast.ASTHandle { return f.pin(f.c.IntNumeral(n, sort)) }
	xx := f.pin(f.m.MkArithmeticMul(x, x))
	twoy := f.pin(f.m.MkArithmeticMul(num(2), y))
	body := f.pin(f.m.MkArithmeticSub(xx, twoy))
	first := f.pin(f.m.MkArithmeticAdd(body, y))
	second := f.pin(f.m.MkArithmeticAdd(f.pin(f.m.MkArithmeticAdd(num(4), num(3))), first))
	for example, term := range []ast.ASTHandle{first, second} {
		normalized, err := f.c.Simplify(term)
		astTestMust(t, err)
		h, err := f.m.RefGet(normalized)
		astTestMust(t, err)
		family, err := f.m.ArithmeticFamilyID()
		astTestMust(t, err)
		isArithmetic := func(term ast.ASTHandle, op int32) bool {
			id, err := f.m.GetFamilyID(term)
			astTestMust(t, err)
			kind, err := f.m.GetDeclKind(term)
			astTestMust(t, err)
			return id == family && kind == op
		}
		terms := []ast.ASTHandle{h}
		if isArithmetic(h, 6) {
			args, err := f.m.AppArgumentView(h)
			astTestMust(t, err)
			terms = nil
			for i := uint32(0); i < args.Count(); i++ {
				term, err := args.At(i)
				astTestMust(t, err)
				terms = append(terms, term)
			}
		}
		constant := int64(0)
		factors := make(map[ast.ASTHandle]int64)
		for _, term := range terms {
			if n, ok, err := f.c.NumeralInt64(term); err == nil && ok {
				constant = n
				continue
			}
			coefficient, factor := int64(1), term
			if isArithmetic(term, 9) {
				args, err := f.m.AppArgumentView(term)
				astTestMust(t, err)
				first, err := args.At(0)
				astTestMust(t, err)
				if n, ok, err := f.c.NumeralInt64(first); err == nil && ok {
					coefficient = n
					if args.Count() != 2 {
						t.Fatal("unexpected factored term arity", args.Count())
					}
					factor, err = args.At(1)
					astTestMust(t, err)
				}
			}
			factors[factor] = coefficient
		}
		if constant != int64(example)*7 || len(factors) != 2 || factors[xx] != 1 || factors[y] != -1 {
			t.Fatal("polynomial decomposition", constant, factors)
		}
		for _, xv := range []int64{-2, 0, 3} {
			for _, yv := range []int64{-1, 0, 4} {
				replaced, err := f.c.Substitute(h, []ast.ASTHandle{x, y}, []ast.ASTHandle{num(xv), num(yv)})
				astTestMust(t, err)
				r, err := f.m.RefGet(replaced)
				astTestMust(t, err)
				value, err := f.c.Simplify(r)
				astTestMust(t, err)
				v, err := f.m.RefGet(value)
				astTestMust(t, err)
				got, ok, err := f.c.NumeralInt64(v)
				astTestMust(t, err)
				want := xv*xv - yv + int64(example)*7
				if !ok || got != want {
					t.Fatalf("example=%d x=%d y=%d got=%d want=%d", example, xv, yv, got, want)
				}
				astTestMust(t, f.m.DelRef(&value))
				astTestMust(t, f.m.DelRef(&replaced))
			}
		}
		astTestMust(t, f.m.DelRef(&normalized))
	}
}
