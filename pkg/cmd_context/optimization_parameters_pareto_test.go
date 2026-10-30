package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestActiveParetoEnumerationSurvivesParameterUpdate(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	x := optimizationComparisonInteger(t, c, "pareto-params-x")
	y := optimizationComparisonInteger(t, c, "pareto-params-y")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	contextMust(t, err)
	xNonnegative, err := m.MkArithmeticGE(x, zero)
	contextMust(t, err)
	yNonnegative, err := m.MkArithmeticGE(y, zero)
	contextMust(t, err)
	sum, err := m.MkArithmeticAdd(x, y)
	contextMust(t, err)
	sumIsOne, err := m.MkEq(sum, one)
	contextMust(t, err)
	contextMust(t, c.Assert(xNonnegative, ""))
	contextMust(t, c.Assert(yNonnegative, ""))
	contextMust(t, c.Assert(sumIsOne, ""))

	initial, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&initial)) }()
	priority, err := m.Symbols.Intern("pareto")
	contextMust(t, err)
	priorityName := "priority"
	contextMust(t, c.config.Params.SetSymbolName(&initial, &priorityName, priority))
	contextMust(t, c.UpdateOptimizationParameters(&initial))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoBackend == nil {
		t.Fatal("first Pareto point", status, c.optimizationParetoBackend)
	}

	updated, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&updated)) }()
	lex, err := m.Symbols.Intern("lex")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&updated, &priorityName, lex))
	timeoutName := "timeout"
	contextMust(t, c.config.Params.SetUIntName(&updated, &timeoutName, 23))
	contextMust(t, c.UpdateOptimizationParameters(&updated))
	backendParams, err := c.optimizationParetoBackend.Params()
	contextMust(t, err)
	gotTimeout, err := c.config.Params.GetUIntName(backendParams, &timeoutName, 0)
	contextMust(t, err)
	if gotTimeout != 23 {
		t.Fatal("active Pareto backend missed parameter update", gotTimeout)
	}

	for i, want := range []lbool.LBool{lbool.True, lbool.False} {
		status, err = c.CheckSat(nil)
		contextMust(t, err)
		if status != want {
			t.Fatal("continued Pareto status", i, status, "want", want)
		}
		if (i == 0) != (c.optimizationParetoBackend != nil) {
			t.Fatal("Pareto engine lifecycle after parameter update", i, c.optimizationParetoBackend)
		}
	}
}

func TestActiveBoxEnumerationSurvivesPriorityUpdate(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	x := optimizationComparisonInteger(t, c, "box-params-x")
	y := optimizationComparisonInteger(t, c, "box-params-y")
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

	parameters, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&parameters)) }()
	priorityName := "priority"
	box, err := m.Symbols.Intern("box")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&parameters, &priorityName, box))
	contextMust(t, c.UpdateOptimizationParameters(&parameters))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || !c.optimizationBoxActive {
		t.Fatal("first box result", status, c.optimizationBoxActive)
	}

	pareto, err := m.Symbols.Intern("pareto")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&parameters, &priorityName, pareto))
	contextMust(t, c.UpdateOptimizationParameters(&parameters))
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || !c.optimizationBoxActive || c.optimizationBoxNext != 2 {
		t.Fatal("box continuation after priority update", status, c.optimizationBoxActive, c.optimizationBoxNext)
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False || c.optimizationBoxActive {
		t.Fatal("box terminal status after priority update", status, c.optimizationBoxActive)
	}
}
