package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationObjectiveMutationClosesParetoSolverImmediately(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	x := optimizationComparisonInteger(t, c, "pareto-state-x")
	y := optimizationComparisonInteger(t, c, "pareto-state-y")
	z := optimizationComparisonInteger(t, c, "pareto-state-z")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	contextMust(t, err)
	for _, term := range []ast.ASTHandle{x, y} {
		lower, e := m.MkArithmeticGE(term, zero)
		contextMust(t, e)
		upper, e := m.MkArithmeticLE(term, one)
		contextMust(t, e)
		contextMust(t, c.Assert(lower, ""))
		contextMust(t, c.Assert(upper, ""))
	}
	key := "priority"
	pareto, err := m.Symbols.Intern("pareto")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&c.params, &key, pareto))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil {
		t.Fatal("Pareto enumeration was not left active", status, c.optimizationParetoBackend)
	}
	boolean, err := m.MkTrue()
	contextMust(t, err)
	if err = c.AddObjective(boolean, true); err == nil {
		t.Fatal("Boolean objective was accepted")
	}
	if c.optimizationParetoBackend != nil || c.optimizationParetoActive {
		t.Fatal("rejected objective mutation left the old Pareto solver active", c.optimizationParetoBackend, c.optimizationParetoActive)
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil {
		t.Fatal("Pareto enumeration did not restart after rejected objective", status, c.optimizationParetoBackend)
	}
	if err = c.AddSoftConstraint(x, big.NewRat(1, 1), "invalid"); err == nil {
		t.Fatal("integer soft constraint was accepted")
	}
	if c.optimizationParetoBackend != nil || c.optimizationParetoActive {
		t.Fatal("rejected soft constraint left the old Pareto solver active", c.optimizationParetoBackend, c.optimizationParetoActive)
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil {
		t.Fatal("Pareto enumeration did not restart after rejected soft constraint", status, c.optimizationParetoBackend)
	}
	lower, err := m.MkArithmeticGE(x, zero)
	contextMust(t, err)
	contextMust(t, c.Assert(lower, ""))
	if c.optimizationParetoBackend != nil || c.optimizationParetoActive {
		t.Fatal("hard constraint mutation left the old Pareto solver active", c.optimizationParetoBackend, c.optimizationParetoActive)
	}
	contextMust(t, c.AddObjective(z, true))
	if c.optimizationParetoBackend != nil || c.optimizationParetoActive {
		t.Fatal("objective mutation left the old Pareto solver active", c.optimizationParetoBackend, c.optimizationParetoActive)
	}
}
