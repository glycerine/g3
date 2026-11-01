package cmd_context

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationResourceLimitedSoftBoundsEndToEnd(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	variables := make([]ast.ASTHandle, 6)
	for i := range variables {
		name := fmt.Sprintf("partial-bound-%d", i)
		_, err = c.DeclareFunction(name, nil, boolean)
		contextMust(t, err)
		variables[i], err = c.MakeApp(name, nil, nil, ast.ASTHandle{})
		contextMust(t, err)
		contextMust(t, c.AddSoftConstraint(variables[i], big.NewRat(int64(i+1), 1), ""))
	}
	all, err := m.MkAndArray(variables)
	contextMust(t, err)
	notAll, err := m.MkNot(all)
	contextMust(t, err)
	contextMust(t, c.Assert(notAll, ""))
	contextMust(t, c.SetUIntOption("rlimit", 160))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatalf("limited optimization status = %v, want unknown", status)
	}
	lower, err := c.ObjectiveBound(0, true)
	contextMust(t, err)
	upper, err := c.ObjectiveBound(0, false)
	contextMust(t, err)
	// The cheapest way to violate the conjunction is to falsify the
	// weight-one term, so the exact optimum is one.
	optimum := big.NewRat(1, 1)
	if lower.Value.Main.Cmp(optimum) > 0 || upper.Value.Main.Cmp(optimum) < 0 || lower.Value.Main.Cmp(&upper.Value.Main) >= 0 {
		t.Fatalf("partial interval [%s:%s] does not bracket optimum 1", lower.Value.Main.RatString(), upper.Value.Main.RatString())
	}
	var output bytes.Buffer
	contextMust(t, c.WriteOptimizationFrontendValues(&output))
	if got := output.String(); !strings.HasPrefix(got, "  [") || !strings.HasSuffix(got, "]\n") {
		t.Fatalf("partial bounds output = %q", got)
	}
}
