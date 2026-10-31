package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"math"
	"testing"
)

func TestIntervalRelationLinearComparisonRecognition(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	real, e := m.MkFamilySort(family, 0, nil)
	astTestMust(t, e)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, integer)), own(m.MkVar(1, integer))
	rx, ry := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	zero, two, minus := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("2")), own(m.ArithmeticIntegerNumeralText("-1"))
	add := own(m.MkArithmeticAdd(x, two))
	le := own(m.MkArithmeticLE(x, y))
	rlt := own(m.MkArithmeticLT(rx, ry))
	tests := []struct {
		name        string
		h           ast.ASTHandle
		kind        int
		pos, neg    uint32
		k           string
		integer, ok bool
	}{
		{"integer shifted le", own(m.MkArithmeticLE(add, y)), 0, 1, 0, "-2", true, true},
		{"integer ge", own(m.MkArithmeticGE(x, y)), 0, 0, 1, "0", true, true},
		{"integer lt", own(m.MkArithmeticLT(x, y)), 0, 1, 0, "-1", true, true},
		{"integer gt source offset", own(m.MkArithmeticGT(x, y)), 0, 0, 1, "1", true, true},
		{"not integer le", own(m.MkNot(le)), 0, 0, 1, "-1", true, true},
		{"real lt", rlt, 1, 1, 0, "0", false, true},
		{"not real lt", own(m.MkNot(rlt)), 0, 0, 1, "0", false, true},
		{"equality", own(m.MkEq(add, y)), 2, 1, 0, "-2", false, true},
		{"minus one source polarity", own(m.MkArithmeticGE(own(m.MkArithmeticMul(minus, x)), y)), 0, 0, 1, "0", true, true},
		{"duplicate positive", own(m.MkArithmeticGE(own(m.MkArithmeticAdd(x, x)), zero)), 0, 0, 0, "", false, false},
		{"unary negation unsupported", own(m.MkArithmeticGE(own(m.MkArithmeticNeg(x)), zero)), 0, 0, 0, "", false, false},
		{"constant only", own(m.MkArithmeticGE(two, zero)), 0, 0, 0, "", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, c, ok, e := intervalComparison(m, tc.h)
			astTestMust(t, e)
			if ok != tc.ok {
				t.Fatalf("matched %v want %v", ok, tc.ok)
			}
			if ok && (kind != tc.kind || c.positive != tc.pos || c.negative != tc.neg || c.constant.RatString() != tc.k || c.integer != tc.integer) {
				t.Fatalf("kind=%d +%d -%d k=%s integer=%v", kind, c.positive, c.negative, c.constant.RatString(), c.integer)
			}
		})
	}
	// A missing side uses the source unsigned sentinel rather than variable zero.
	lone := own(m.MkArithmeticLE(x, two))
	kind, c, ok, e := intervalComparison(m, lone)
	astTestMust(t, e)
	if !ok || kind != 0 || c.positive != math.MaxUint32 || c.negative != 0 || c.constant.RatString() != "2" {
		t.Fatal("sentinel", kind, c, ok)
	}
}
