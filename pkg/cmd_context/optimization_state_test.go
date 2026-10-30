package cmd_context

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
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
	if c.optimizationParetoBackend == nil || !c.optimizationParetoActive {
		t.Fatal("prevalidated soft-constraint rejection cleared Pareto state", c.optimizationParetoBackend, c.optimizationParetoActive)
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

func TestPopResetsActiveParetoOptimization(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	x := optimizationComparisonInteger(t, c, "pareto-pop-x")
	y := optimizationComparisonInteger(t, c, "pareto-pop-y")
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
	priority := "priority"
	pareto, err := m.Symbols.Intern("pareto")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&c.params, &priority, pareto))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil {
		t.Fatal("initial Pareto result", status, c.optimizationParetoBackend)
	}
	oldBackend := c.optimizationParetoBackend
	contextMust(t, c.Push(1))
	contextMust(t, c.Pop(1))
	if c.optimizationParetoActive || c.optimizationParetoBackend != nil {
		t.Fatal("scope pop left Pareto enumeration active", c.optimizationParetoActive, c.optimizationParetoBackend)
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil || c.optimizationParetoBackend == oldBackend {
		t.Fatal("Pareto enumeration did not restart after scope pop", status, c.optimizationParetoBackend == oldBackend)
	}
	contextMust(t, c.Pop(0))
	if c.optimizationParetoActive || c.optimizationParetoBackend != nil {
		t.Fatal("zero-count pop left Pareto enumeration active", c.optimizationParetoActive, c.optimizationParetoBackend)
	}
}

func TestPopResetsActiveBoxOptimization(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	x := optimizationComparisonInteger(t, c, "box-pop-x")
	y := optimizationComparisonInteger(t, c, "box-pop-y")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	contextMust(t, err)
	for _, term := range []ast.ASTHandle{x, y} {
		lower, e := m.MkArithmeticGE(term, zero)
		contextMust(t, e)
		contextMust(t, c.Assert(lower, ""))
	}
	sum, err := m.MkArithmeticAdd(x, y)
	contextMust(t, err)
	sumIsOne, err := m.MkEq(sum, one)
	contextMust(t, err)
	contextMust(t, c.Assert(sumIsOne, ""))
	priority := "priority"
	box, err := m.Symbols.Intern("box")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&c.params, &priority, box))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || !c.optimizationBoxActive || len(c.optimizationBoxModels) != 2 {
		t.Fatal("initial box result", status, c.optimizationBoxActive, len(c.optimizationBoxModels))
	}
	oldModels := append([]*model.ModelCore(nil), c.optimizationBoxModels...)
	contextMust(t, c.Push(1))
	contextMust(t, c.Pop(1))
	if c.optimizationBoxActive || c.optimizationBoxNext != 0 || len(c.optimizationBoxModels) != 2 || len(c.optimizationBoxValues) != 2 {
		t.Fatal("scope pop left box enumeration state", c.optimizationBoxActive, c.optimizationBoxNext, len(c.optimizationBoxModels), len(c.optimizationBoxValues))
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || !c.optimizationBoxActive || len(c.optimizationBoxModels) != 2 {
		t.Fatal("box enumeration did not restart after scope pop", status, c.optimizationBoxActive, len(c.optimizationBoxModels))
	}
	if c.optimizationBoxModels[0] == oldModels[0] || c.optimizationBoxModels[1] == oldModels[1] {
		t.Fatal("box enumeration reused models from the popped optimization run")
	}
}
