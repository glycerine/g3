package rewriter

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestRepeatedAlgebraicFactorsAndReciprocalPowers(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	previous, err := uparams.BindProcessGlobalParamsService(g)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, err := uparams.BindProcessGlobalParamsService(previous)
		astTestMust(t, err)
	})
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	am, err := m.ArithmeticAlgebraicManager()
	astTestMust(t, err)
	for _, tc := range []struct {
		power             uint32
		numerator         int32
		denominator       int32
		ordinary, inverse string
	}{
		{2, 3, 2, "3/2", "2/3"},
		{3, -2, 5, "-2/5", "-5/2"},
		{4, 7, 1, "7", "1/7"},
	} {
		q := unumeral.Rational{Num: unumeral.NewInt(tc.numerator), Den: unumeral.NewInt(tc.denominator)}
		var base, root polynomial.Algebraic
		astTestMust(t, am.SetRational(&base, &q))
		astTestMust(t, am.Root(&base, tc.power, &root))
		h, err := m.ArithmeticAlgebraicNumeral(&root)
		astTestMust(t, err)
		astTestPin(t, m, h)
		astTestMust(t, am.Del(&root))
		astTestMust(t, am.Del(&base))
		args := make([]ast.ASTHandle, tc.power)
		for i := range args {
			args[i] = h
		}
		status, result := arithRewriterCore(t, r, arithOpMul, args...)
		if status != RewriteDone {
			t.Fatalf("degree %d multiplication returned %v", tc.power, status)
		}
		arithRewriterWantNumeral(t, m, result, tc.ordinary, false)
		for _, v := range []struct {
			exponent int
			want     string
		}{
			{int(tc.power), tc.ordinary}, {-int(tc.power), tc.inverse},
		} {
			exp := arithRewriterNumeral(t, m, fmt.Sprint(v.exponent), false)
			status, result := arithRewriterCore(t, r, arithOpPower, h, exp)
			if v.exponent < 0 && (ast.ASTSourceDebug || diagnostic.TraceCompiled()) && status == Rewrite2 {
				// These builds expose the intermediate (^ (/ 1 h) k) step.
				continue
			}
			if status != RewriteDone {
				t.Fatalf("exponent %d returned %v", v.exponent, status)
			}
			arithRewriterWantNumeral(t, m, result, v.want, false)
		}
		if _, _, ok, err := m.ArithmeticBinomialPower(h, tc.power+1); err != nil || ok {
			t.Fatalf("different exponent %d recognized (%v, %v)", tc.power+1, ok, err)
		}
	}
}
