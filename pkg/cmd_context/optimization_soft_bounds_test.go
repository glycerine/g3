package cmd_context

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationUnknownPublishesSoftSearchBounds(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	term, err := m.MkTrue()
	contextMust(t, err)
	contextMust(t, c.AddSoftConstraint(term, big.NewRat(5, 1), ""))
	result := &opt.ArithmeticResult{Status: lbool.Undef, Values: make([]opt.ObjectiveValue, 1)}
	value := &result.Values[0]
	value.Soft = true
	value.SoftBoundsValid = true
	value.SoftLower.SetFrac64(1, 2)
	value.SoftUpper.SetFrac64(5, 2)
	c.publishOptimizationSoftBounds(result)
	value.SoftLower.SetInt64(77)
	value.SoftUpper.SetInt64(88)
	lower, err := c.ObjectiveBound(0, true)
	contextMust(t, err)
	upper, err := c.ObjectiveBound(0, false)
	contextMust(t, err)
	if lower.Value.Main.Cmp(big.NewRat(1, 2)) != 0 || upper.Value.Main.Cmp(big.NewRat(5, 2)) != 0 {
		t.Fatal("partial bounds", lower.Value.Main.RatString(), upper.Value.Main.RatString())
	}
	var output bytes.Buffer
	contextMust(t, c.WriteOptimizationFrontendValues(&output))
	if got, want := output.String(), "  [(/ 1.0 2.0):(/ 5.0 2.0)]\n"; got != want {
		t.Fatalf("partial bound output = %q, want %q", got, want)
	}
	c.publishOptimizationSoftBounds(&opt.ArithmeticResult{Values: make([]opt.ObjectiveValue, 1)})
	if _, err = c.ObjectiveBound(0, true); err == nil {
		t.Fatal("unavailable bounds reused from an earlier check")
	}
}
