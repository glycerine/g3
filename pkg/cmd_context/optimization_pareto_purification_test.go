package cmd_context

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func purifiedParetoFixture(t *testing.T) (*Context, ast.ASTHandle) {
	t.Helper()
	c := contextFixture(t)
	x := optimizationComparisonInteger(t, c, "purified-x")
	for _, bound := range []struct {
		value string
		lower bool
	}{{"0", true}, {"2", false}} {
		n, err := c.manager.ArithmeticIntegerNumeralText(bound.value)
		contextMust(t, err)
		var formula ast.ASTHandle
		if bound.lower {
			formula, err = c.manager.MkArithmeticGE(x, n)
		} else {
			formula, err = c.manager.MkArithmeticLE(x, n)
		}
		contextMust(t, err)
		contextMust(t, c.Assert(formula, ""))
	}
	p, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&p)) }()
	priority, err := c.manager.Symbols.Intern("pareto")
	contextMust(t, err)
	key := "priority"
	contextMust(t, c.config.Params.SetSymbolName(&p, &key, priority))
	contextMust(t, c.UpdateOptimizationParameters(&p))
	return c, x
}

func TestParetoPurificationPreparationFailureCleanup(t *testing.T) {
	success, failures := false, 0
	for after := int64(0); after < 256; after++ {
		c, x := purifiedParetoFixture(t)
		negative, err := c.manager.MkArithmeticNeg(x)
		contextMust(t, err)
		contextMust(t, c.AddObjective(negative, true))
		c.manager.Budget().FailAfter(after)
		err = c.prepareParetoPurification()
		c.manager.Budget().FailAfter(-1)
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
			failures++
			if c.optimizationParetoPurification != nil {
				t.Fatal("failed preparation owner", after)
			}
		} else {
			success = true
		}
		contextMust(t, c.closeParetoBackend())
		if c.objectives[0].expression != negative {
			t.Fatal("failed preparation replaced objective")
		}
		if success {
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("preparation frontier", failures, success)
	}
}

func TestParetoBackendConstructionRefusalClosesUnpublishedSolver(t *testing.T) {
	c, x := purifiedParetoFixture(t)
	contextMust(t, c.AddObjective(x, true))
	contextMust(t, c.prepareParetoPurification())
	backend, err := c.newParetoBackend([]ast.ASTHandle{{}})
	if err == nil || backend != nil {
		t.Fatal("invalid assumption accepted", backend, err)
	}
	contextMust(t, c.closeParetoBackend())
	// contextFixture closes all shared services and requires zero live budget;
	// an unpublished solver must not disappear through a named nil return.
}

func TestParetoLivePurificationPersistsAndFiltersFrontierModels(t *testing.T) {
	for _, maximize := range []bool{false, true} {
		t.Run(map[bool]string{false: "minimize", true: "maximize"}[maximize], func(t *testing.T) {
			c, x := purifiedParetoFixture(t)
			negative, err := c.manager.MkArithmeticNeg(x)
			contextMust(t, err)
			contextMust(t, c.AddObjective(negative, maximize))
			contextMust(t, c.AddObjective(x, maximize))
			decl, err := c.manager.GetDecl(x)
			contextMust(t, err)
			seen := make(map[string]bool)
			var state *paretoPurification
			var freshDecl ast.ASTHandle
			for i := 0; i < 3; i++ {
				status, err := c.CheckSat(nil)
				contextMust(t, err)
				if status != lbool.True {
					t.Fatal("frontier point", i, status)
				}
				current := c.optimizationParetoPurification
				if current == nil {
					t.Fatal("live check skipped purification")
				}
				if i == 0 {
					state = current
					stats, err := state.purifier.Statistics()
					contextMust(t, err)
					if stats.ArithmeticDefinitions != 1 || len(state.hard) != 3 {
						t.Fatal("definitions", stats, len(state.hard))
					}
					freshDecl, err = c.manager.GetDecl(state.objectives[0].expression)
					contextMust(t, err)
				} else if current != state {
					t.Fatal("continuation regenerated definitions")
				}
				if c.objectives[0].expression != negative {
					t.Fatal("published objective was replaced")
				}
				md, err := c.Model()
				contextMust(t, err)
				hidden, err := md.HasConstantInterpretation(freshDecl)
				contextMust(t, err)
				if hidden {
					t.Fatal("fresh definition leaked into published model")
				}
				value, err := md.ConstantInterpretation(decl)
				contextMust(t, err)
				text, _, matched, err := c.manager.ArithmeticNumeralText(value)
				contextMust(t, err)
				if !matched || seen[text] {
					t.Fatal("frontier value", text, matched, seen)
				}
				seen[text] = true
				contextMust(t, md.DecRef())
			}
			if !seen["0"] || !seen["1"] || !seen["2"] {
				t.Fatal(seen)
			}
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			// set_pareto(nullptr) ends the engine; its opt_solver lives until the next init_solver.
			if status != lbool.False || c.optimizationParetoActive {
				t.Fatal("exhausted state", status)
			}
		})
	}
}

func TestParetoLiveSoftPurificationAndMutationCleanup(t *testing.T) {
	c, x := purifiedParetoFixture(t)
	zero, err := c.manager.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	condition, err := c.manager.MkArithmeticGT(x, zero)
	contextMust(t, err)
	contextMust(t, c.AddSoftConstraint(condition, big.NewRat(3, 2), "soft"))
	contextMust(t, c.AddObjective(x, false))
	var state *paretoPurification
	for i := 0; i < 2; i++ {
		status, err := c.CheckSat(nil)
		contextMust(t, err)
		if status != lbool.True {
			t.Fatal("soft frontier", i, status)
		}
		if i == 0 {
			state = c.optimizationParetoPurification
			if state == nil {
				t.Fatal("missing purification")
			}
			stats, err := state.purifier.Statistics()
			contextMust(t, err)
			if stats.SoftDefinitions != 1 || state.objectives[0].softTerms[0].expression == condition {
				t.Fatal(stats)
			}
		} else if state != c.optimizationParetoPurification {
			t.Fatal("soft definitions changed")
		}
		fresh, err := c.manager.GetDecl(state.objectives[0].softTerms[0].expression)
		contextMust(t, err)
		md, err := c.Model()
		contextMust(t, err)
		found, err := md.HasConstantInterpretation(fresh)
		contextMust(t, err)
		if found {
			t.Fatal("soft auxiliary leaked")
		}
		contextMust(t, md.DecRef())
	}
	// Any mutation ends the engine (clear_state); the next init_solver
	// replaces the backend and its owned definitions.
	contextMust(t, c.Assert(condition, ""))
	if c.optimizationParetoActive {
		t.Fatal("mutation left purification alive")
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True || c.optimizationParetoPurification == state {
		t.Fatal("fresh run", status)
	}
}
