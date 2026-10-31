package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:483-541,563-597,612-683.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBV2RealSymbolicReducersIndependentAssignments(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	m := f.m
	vars := []ast.ASTHandle{
		bv2RealSymbolicBV(t, m, "a", 3), bv2RealSymbolicBV(t, m, "b", 4),
		bv2RealSymbolicBV(t, m, "c", 3), bv2RealSymbolicBV(t, m, "d", 4),
	}
	encode := func(a, b ast.ASTHandle, divisor int64) ast.ASTHandle {
		return bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
			return c.encoding(a, b, big.NewRat(divisor, 1), big.NewRat(3, 1))
		})
	}
	s, tv := encode(vars[0], vars[1], 2), encode(vars[2], vars[3], 1)
	var coefficients [3][2]ast.ASTHandle
	for i, op := range []int32{6, 7, 9} {
		h := bv2RealRewrite(t, r, fixDLApp(t, m, op, s, tv))
		c := &bv2RealCall{u: u}
		term, found := c.matchTerm(h)
		equationMust(t, c.err)
		if !found {
			t.Fatal("symbolic arithmetic refused", op)
		}
		for j, value := range []ast.ASTHandle{term.first, term.second} {
			fixDLPin(t, m, value)
			coefficients[i][j] = value
		}
		if divisor := c.rationalValue(term.divisor); divisor.Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("symbolic common divisor", divisor)
		}
		equationMust(t, c.close())
	}
	equality := bv2RealRewrite(t, r, bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
		return c.basic(ast.ASTOpEq, s, tv)
	}))
	le := bv2RealRewrite(t, r, fixDLApp(t, m, 2, s, tv))
	sub, err := m.NewExprSubstitution()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelExprSubstitution(&sub)) })
	replace, err := rewriter.NewDefaultExprReplacer(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, replace.Close()) })
	out := bv2RealOut(t, m)
	instantiate := func(h ast.ASTHandle) ast.ASTHandle {
		equationMust(t, replace.Rewrite(h, out))
		value, err := m.RefGet(out)
		equationMust(t, err)
		return value
	}
	// Rewriting happens once, while coefficients are still symbolic. These 256
	// independent valuations include negative extrema and unequal operand widths.
	for bits := 0; bits < 256; bits++ {
		var values [4]int64
		for i, variable := range vars {
			values[i] = []int64{-4, -1, 0, 3}[(bits>>(2*i))&3]
			width := uint32(3)
			if i%2 != 0 {
				width = 4
			}
			n := values[i]
			if n < 0 {
				n += 1 << width
			}
			literal := f.bv(t, uint32(n), width)
			equationMust(t, m.ExprSubstitutionInsert(sub, variable, literal, ast.ASTHandle{}, ast.ASTDependency{}))
		}
		equationMust(t, replace.SetSubstitution(&sub))
		equationMust(t, replace.Reset())
		a, b, c, d := values[0], values[1], values[2], values[3]
		want := [3][2]int64{{a + 2*c, b + 2*d}, {a - 2*c, b - 2*d}, {a*c + 3*b*d, a*d + b*c}}
		for i := range coefficients {
			for j, h := range coefficients[i] {
				if got := bv2RealSignedValue(t, f, instantiate(h)); got.Cmp(big.NewInt(want[i][j])) != 0 {
					t.Fatal("symbolic coefficient", values, i, j, got, want[i][j])
				}
			}
		}
		f.wantBool(t, instantiate(equality), a == 2*c && b == 2*d)
		// Exact sign of x+y*sqrt(3), using integer squares rather than floats.
		x, y := 2*c-a, 2*d-b
		nonnegative := x >= 0 && y >= 0
		if x >= 0 && y < 0 {
			nonnegative = x*x >= 3*y*y
		} else if x < 0 && y >= 0 {
			nonnegative = 3*y*y >= x*x
		}
		f.wantBool(t, instantiate(le), nonnegative)
	}
}

func TestBV2RealQuantifiedBodyReducesBoundCoefficients(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	m := f.m
	sort, err := m.BitVectorSort(4)
	equationMust(t, err)
	v, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, v)
	s := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
		return c.encoding(v, c.numeral(big.NewRat(0, 1), 4), big.NewRat(2, 1), big.NewRat(3, 1))
	})
	sum := fixDLApp(t, m, 6, s, s)
	body := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
		return c.basic(ast.ASTOpEq, sum, s)
	})
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(17)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, q)
	result := bv2RealRewrite(t, r, q)
	contains, err := u.ContainsBV2Real(result)
	equationMust(t, err)
	if contains {
		t.Fatal("quantifier body kept a supported real encoding")
	}
	actual, err := m.GetQuantifierBody(result)
	equationMust(t, err)
	if actual == body {
		t.Fatal("quantifier body was not visited")
	}
	// Decode the resulting Boolean body at each assignment to its bound BV.
	// (s+s)=s with s=sbv(v)/2 is true exactly when v is zero.
	replace, err := rewriter.NewDefaultExprReplacer(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, replace.Close()) })
	out := bv2RealOut(t, m)
	for value := uint32(0); value < 16; value++ {
		equationMust(t, m.SetRef(out, actual))
		equationMust(t, replace.ApplySubstitution(v, f.bv(t, value, 4), ast.ASTHandle{}, out))
		h, err := m.RefGet(out)
		equationMust(t, err)
		f.wantBool(t, h, value == 0)
	}
}
