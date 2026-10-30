package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationBoundsPreserveStrictLexicographicEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name             string
		maximize         bool
		wantLowerEpsilon *big.Rat
		wantUpperEpsilon *big.Rat
	}{
		{name: "maximize", maximize: true, wantLowerEpsilon: new(big.Rat), wantUpperEpsilon: big.NewRat(-1, 1)},
		{name: "minimize", maximize: false, wantLowerEpsilon: big.NewRat(1, 1), wantUpperEpsilon: new(big.Rat)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := contextFixture(t)
			contextMust(t, c.SetLogic("QF_LRA"))
			real, err := c.LookupSort("Real")
			contextMust(t, err)
			_, err = c.DeclareFunction("strict-bound-x", nil, real)
			contextMust(t, err)
			_, err = c.DeclareFunction("strict-bound-y", nil, real)
			contextMust(t, err)
			x, err := c.MakeApp("strict-bound-x", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			y, err := c.MakeApp("strict-bound-y", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			zero, err := c.manager.ArithmeticRealNumeralText("0")
			contextMust(t, err)
			five, err := c.manager.ArithmeticRealNumeralText("5")
			contextMust(t, err)
			positive, err := c.manager.MkArithmeticGT(x, zero)
			contextMust(t, err)
			belowFive, err := c.manager.MkArithmeticLT(x, five)
			contextMust(t, err)
			yEqualsX, err := c.manager.MkEq(y, x)
			contextMust(t, err)
			for _, assertion := range []ast.ASTHandle{positive, belowFive, yEqualsX} {
				contextMust(t, c.Assert(assertion, ""))
			}
			contextMust(t, c.AddObjective(x, tc.maximize))
			contextMust(t, c.AddObjective(y, !tc.maximize))
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("optimization status", status)
			}
			lower, err := c.ObjectiveBound(0, true)
			contextMust(t, err)
			upper, err := c.ObjectiveBound(0, false)
			contextMust(t, err)
			value, err := c.ObjectiveValue(0)
			contextMust(t, err)
			wantLower, wantUpper := value.ModelValue, value.Value
			if !tc.maximize {
				wantLower, wantUpper = value.Value, value.ModelValue
			}
			if lower.Value.Infinity.Sign() != 0 || lower.Value.Main.Cmp(&wantLower.Main) != 0 || lower.Value.Epsilon.Cmp(tc.wantLowerEpsilon) != 0 {
				t.Fatalf("lower objective endpoint %s + %s epsilon, want model/value %s + %s epsilon", lower.Value.Main.RatString(), lower.Value.Epsilon.RatString(), wantLower.Main.RatString(), tc.wantLowerEpsilon.RatString())
			}
			if upper.Value.Infinity.Sign() != 0 || upper.Value.Main.Cmp(&wantUpper.Main) != 0 || upper.Value.Epsilon.Cmp(tc.wantUpperEpsilon) != 0 {
				t.Fatalf("upper objective endpoint %s + %s epsilon, want value/model %s + %s epsilon", upper.Value.Main.RatString(), upper.Value.Epsilon.RatString(), wantUpper.Main.RatString(), tc.wantUpperEpsilon.RatString())
			}
		})
	}
}

func TestOptimizationBoundsPreserveUnboundedLexicographicEndpoints(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_LRA"))
	real, err := c.LookupSort("Real")
	contextMust(t, err)
	for _, name := range []string{"unbounded-bound-x", "unbounded-bound-y"} {
		_, err = c.DeclareFunction(name, nil, real)
		contextMust(t, err)
	}
	x, err := c.MakeApp("unbounded-bound-x", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	y, err := c.MakeApp("unbounded-bound-y", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, false))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("optimization status", status)
	}
	cases := []struct {
		index        uint32
		lower        bool
		wantInfinity int64
	}{
		{index: 0, lower: true, wantInfinity: 1},
		{index: 0, lower: false, wantInfinity: 1},
		{index: 1, lower: true, wantInfinity: -1},
		{index: 1, lower: false, wantInfinity: 1},
	}
	for _, tc := range cases {
		bound, boundErr := c.ObjectiveBound(tc.index, tc.lower)
		contextMust(t, boundErr)
		if bound.Value.Infinity.Cmp(big.NewRat(tc.wantInfinity, 1)) != 0 || bound.Value.Main.Sign() != 0 || bound.Value.Epsilon.Sign() != 0 {
			t.Fatalf("objective %d lower=%t endpoint infinity=%s main=%s epsilon=%s; want infinity=%d", tc.index, tc.lower, bound.Value.Infinity.RatString(), bound.Value.Main.RatString(), bound.Value.Epsilon.RatString(), tc.wantInfinity)
		}
		if !bound.Integer {
			t.Fatalf("objective %d lower=%t infinity endpoint should have integer numeral sort", tc.index, tc.lower)
		}
	}
}
