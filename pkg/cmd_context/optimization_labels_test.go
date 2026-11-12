package cmd_context

// Source: opt_pareto.cpp gia_pareto::operator()/pareto_base::get_model,
// opt_context.cpp context::yield/get_labels and basic_cmds.cpp labels_cmd.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestOptimizationPublishesRelevantLabels(t *testing.T) {
	for _, priority := range []string{"pareto", "lex", "box"} {
		t.Run(priority, func(t *testing.T) {
			c := contextFixture(t)
			x := optimizationComparisonInteger(t, c, "labeled-opt-x")
			m := c.manager
			first, err := m.Symbols.Intern("first")
			contextMust(t, err)
			spaced, err := m.Symbols.Intern("a b")
			contextMust(t, err)
			literal, err := m.MkLabelLit([]usymbol.Symbol{first, spaced})
			contextMust(t, err)
			contextMust(t, c.Assert(literal, ""))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			contextMust(t, err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			contextMust(t, err)
			lower, err := m.MkArithmeticGE(x, zero)
			contextMust(t, err)
			contextMust(t, c.Assert(lower, ""))
			upper, err := m.MkArithmeticLE(x, two)
			contextMust(t, err)
			contextMust(t, c.Assert(upper, ""))
			contextMust(t, c.AddObjective(x, true))
			if priority == "box" {
				contextMust(t, c.AddObjective(x, false))
			}
			local, err := c.config.Params.NewValue()
			contextMust(t, err)
			defer func() { contextMust(t, c.config.Params.Del(&local)) }()
			key := "priority"
			value, err := m.Symbols.Intern(priority)
			contextMust(t, err)
			contextMust(t, c.config.Params.SetSymbolName(&local, &key, value))
			contextMust(t, c.UpdateOptimizationParameters(&local))
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("labeled optimum status", status)
			}
			var out bytes.Buffer
			contextMust(t, c.WriteLabels(&out))
			if got := out.String(); got != "(labels first |a b|)\n" {
				t.Fatalf("source optimization labels = %q", got)
			}
			labels, err := c.checkSatResult().Labels()
			contextMust(t, err)
			if len(labels) != 2 {
				t.Fatal("result labels missing", labels)
			}
			labels[0] = spaced
			again, err := c.checkSatResult().Labels()
			contextMust(t, err)
			if again[0] != first {
				t.Fatal("labels snapshot aliases the published result")
			}
			if priority == "box" {
				status, err = c.CheckSat(nil)
				contextMust(t, err)
				if status != lbool.True {
					t.Fatal("second box model status", status)
				}
				out.Reset()
				contextMust(t, c.WriteLabels(&out))
				if got := out.String(); got != "(labels first |a b|)\n" {
					t.Fatalf("cached box labels = %q", got)
				}
			}
			if priority == "pareto" {
				// optimize() runs a lone objective as execute(obj, true,
				// false) under any priority, and clear_state resets the
				// m_pareto1 toggle first: z3 answers sat again.
				status, err = c.CheckSat(nil)
				contextMust(t, err)
				if status != lbool.True {
					t.Fatal("single-objective pareto recheck status", status)
				}
			}
			if priority == "box" {
				status, err = c.CheckSat(nil)
				contextMust(t, err)
				if status != lbool.False {
					t.Fatal("optimization enumeration was not exhausted", status)
				}
				if err := c.WriteLabels(&out); err == nil {
					t.Fatal("labels command accepted an unsatisfiable result")
				}
			}
		})
	}
}

func TestParetoCapturesLastSatisfiableModelLabelsBeforeSolverPop(t *testing.T) {
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "pareto-engine-label-x")
	m := c.manager
	name, err := m.Symbols.Intern("pareto-label")
	contextMust(t, err)
	literal, err := m.MkLabelLit([]usymbol.Symbol{name})
	contextMust(t, err)
	contextMust(t, c.Assert(literal, ""))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	contextMust(t, err)
	for _, row := range []struct {
		bound ast.ASTHandle
		lower bool
	}{{zero, true}, {three, false}} {
		var constraint ast.ASTHandle
		if row.lower {
			constraint, err = m.MkArithmeticGE(x, row.bound)
		} else {
			constraint, err = m.MkArithmeticLE(x, row.bound)
		}
		contextMust(t, err)
		contextMust(t, c.Assert(constraint, ""))
	}
	contextMust(t, c.AddObjective(x, true))
	result, err := c.checkOptimizePareto(nil, false)
	contextMust(t, err)
	defer func() { contextMust(t, result.Close()) }()
	if result.Status != lbool.True || len(result.Labels) != 1 || result.Labels[0] != name {
		t.Fatal("Pareto improvement loop lost model labels", result.Status, result.Labels)
	}
}
