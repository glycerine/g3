package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationArithmeticEngineForcesInfiniteArithmeticMode(t *testing.T) {
	for _, tc := range []struct {
		engine string
		want   string
	}{{"basic", "2"}, {"farkas", "5"}, {"symba", "5"}} {
		t.Run(tc.engine, func(t *testing.T) {
			c := contextFixture(t)
			if tc.engine != "basic" {
				key := "optsmt_engine"
				value, err := c.config.Params.SymbolsField.Intern(tc.engine)
				contextMust(t, err)
				contextMust(t, c.config.Params.SetSymbolName(&c.params, &key, value))
			}
			x := optimizationComparisonInteger(t, c, "engine-"+tc.engine+"-x")
			// This test targets the engine-to-arithmetic-mode setup. A bounded
			// interval makes the Farkas optimizer leave a temporary Boolean bound
			// without a SAT assignment, triggering the source assertion at
			// smt_model_generator.cpp:96 while generating an otherwise valid model.
			two, err := c.manager.ArithmeticIntegerNumeralText("2")
			contextMust(t, err)
			equality, err := c.manager.MkEq(x, two)
			contextMust(t, err)
			contextMust(t, c.Assert(equality, ""))
			contextMust(t, c.AddObjective(x, true))
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("optimization status", status)
			}
			mode, err := c.config.Globals.SourceGetValueName("smt.arith.solver")
			contextMust(t, err)
			if mode != tc.want {
				t.Fatal("arithmetic solver mode", mode, "want", tc.want)
			}
		})
	}
}

func TestOptimizationArithmeticEngineSetupSkipsParetoContinuation(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	parameters, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&parameters)) }()
	priorityName := "priority"
	priority, err := m.Symbols.Intern("pareto")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&parameters, &priorityName, priority))
	engineName := "optsmt_engine"
	engine, err := m.Symbols.Intern("basic")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&parameters, &engineName, engine))
	contextMust(t, c.UpdateOptimizationParameters(&parameters))
	x := optimizationComparisonInteger(t, c, "engine-pareto-x")
	y := optimizationComparisonInteger(t, c, "engine-pareto-y")
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
	sum, err := m.MkArithmeticAdd(x, y)
	contextMust(t, err)
	sumIsOne, err := m.MkEq(sum, one)
	contextMust(t, err)
	contextMust(t, c.Assert(sumIsOne, ""))
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.AddObjective(y, true))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || !c.optimizationParetoActive {
		t.Fatal("initial Pareto result", status, c.optimizationParetoActive)
	}
	symba, err := m.Symbols.Intern("symba")
	contextMust(t, err)
	contextMust(t, c.config.Params.SetSymbolName(&parameters, &engineName, symba))
	contextMust(t, c.UpdateOptimizationParameters(&parameters))
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status == lbool.Undef {
		t.Fatal("Pareto continuation became unknown", status)
	}
	mode, err := c.config.Globals.SourceGetValueName("smt.arith.solver")
	contextMust(t, err)
	if mode != "2" {
		t.Fatal("continuation reconfigured arithmetic solver", mode, "want 2")
	}
}
