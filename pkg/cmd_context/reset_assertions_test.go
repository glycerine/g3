package cmd_context

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func resetAssertionsFixture(t *testing.T) (*Context, *ast.ASTManager, ast.ASTHandle) {
	t.Helper()
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("global-declarations", true))
	contextMust(t, c.SetBoolOption("auto_config", false))
	contextMust(t, c.SetLogic("ALL"))
	m, err := c.Manager()
	contextMust(t, err)
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	decl, err := c.DeclareFunction("retained-x", nil, integer)
	contextMust(t, err)
	x, err := m.MkApp(decl, nil)
	contextMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	contextMust(t, err)
	eq, err := m.MkEq(x, seven)
	contextMust(t, err)
	contextMust(t, c.Assert(eq, ""))
	contextMust(t, c.Push(2))
	return c, m, decl
}

func checkRetainedSeven(t *testing.T, c *Context, m *ast.ASTManager, decl ast.ASTHandle) {
	t.Helper()
	md, err := c.Model()
	contextMust(t, err)
	defer func() { contextMust(t, md.DecRef()) }()
	value, err := md.ConstantInterpretation(decl)
	contextMust(t, err)
	text, _, ok, err := m.ArithmeticNumeralText(value)
	contextMust(t, err)
	if !ok || text != "7" {
		t.Fatal("retained check result changed", text, ok)
	}
}

func TestCommandContextResetAssertionsPreservesWorldScopesAndResult(t *testing.T) {
	for _, useTactic := range []bool{false, true} {
		t.Run(fmt.Sprint(useTactic), func(t *testing.T) {
			c, m, decl := resetAssertionsFixture(t)
			if useTactic {
				selected, err := c.NewBuiltinTactic("smt")
				contextMust(t, err)
				contextMust(t, selected.IncRef())
				defer func() { contextMust(t, selected.DecRef()) }()
				status, err := c.CheckSatUsing(selected, nil)
				contextMust(t, err)
				if status != lbool.True {
					t.Fatal(status)
				}
			} else {
				status, err := c.CheckSat(nil)
				contextMust(t, err)
				if status != lbool.True {
					t.Fatal(status)
				}
			}
			checkRetainedSeven(t, c, m, decl)
			prior := c.solver
			for repeat := 0; repeat < 2; repeat++ {
				contextMust(t, c.ResetAssertions())
				current, err := c.Manager()
				contextMust(t, err)
				depth, err := c.solver.ScopeLevel()
				contextMust(t, err)
				assertions, err := c.solver.NumAssertions()
				contextMust(t, err)
				if current != m || depth != 2 || assertions != 0 || len(c.assertions) != 0 {
					t.Fatal("reset changed world/scopes or kept assertions", depth, assertions)
				}
				checkRetainedSeven(t, c, m, decl)
			}
			if !useTactic {
				if _, err := prior.NumAssertions(); err != nil {
					t.Fatal("last solver result was closed during reset", err)
				}
			}
			no, err := m.MkFalse()
			contextMust(t, err)
			contextMust(t, c.Assert(no, ""))
			contextMust(t, c.Pop(2))
			if _, err := c.Model(); err == nil {
				t.Fatal("pop did not invalidate the retained result")
			}
			if !useTactic {
				if _, err := prior.NumAssertions(); err == nil {
					t.Fatal("invalidated retained solver stayed open")
				}
			}
			selected, err := c.NewBuiltinTactic("smt")
			contextMust(t, err)
			contextMust(t, selected.IncRef())
			defer func() { contextMust(t, selected.DecRef()) }()
			status, err := c.CheckSatUsing(selected, nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("fresh empty assertion snapshot kept a popped false", status)
			}
		})
	}
}

func TestCommandContextResetAssertionsResultInvalidation(t *testing.T) {
	for _, action := range []string{"assert", "push", "pop-zero", "pop-invalid", "new-check", "full-reset"} {
		t.Run(action, func(t *testing.T) {
			c, m, decl := resetAssertionsFixture(t)
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			contextMust(t, c.ResetAssertions())
			checkRetainedSeven(t, c, m, decl)
			prior := c.retainedSolverResult
			switch action {
			case "assert":
				yes, err := m.MkTrue()
				contextMust(t, err)
				contextMust(t, c.Assert(yes, ""))
			case "push":
				contextMust(t, c.Push(1))
			case "pop-zero":
				contextMust(t, c.Pop(0))
			case "pop-invalid":
				if err := c.Pop(3); err == nil {
					t.Fatal("invalid pop accepted")
				}
			case "new-check":
				_, err = c.CheckSat(nil)
				contextMust(t, err)
			case "full-reset":
				contextMust(t, c.Reset())
			}
			if _, err := prior.NumAssertions(); err == nil {
				t.Fatal("invalidated solver result did not close")
			}
			if action != "new-check" {
				if _, err := c.Model(); err == nil {
					t.Fatal("invalidated model still available")
				}
			}
		})
	}
}

func TestCommandContextResetAssertionsDefaultDropsSession(t *testing.T) {
	c, m, decl := resetAssertionsFixture(t)
	// Select the source default mode before initialization in a separate world.
	contextMust(t, c.Reset())
	contextMust(t, c.SetBoolOption("global-declarations", false))
	contextMust(t, c.SetLogic("QF_UF"))
	m, err := c.Manager()
	contextMust(t, err)
	b, err := c.LookupSort("Bool")
	contextMust(t, err)
	decl, err = c.DeclareFunction("retained-x", nil, b)
	contextMust(t, err)
	contextMust(t, c.Push(2))
	contextMust(t, c.ResetAssertions())
	if m.IsLiveHandle(decl) {
		t.Fatal("default reset kept old manager")
	}
	contextMust(t, c.SetLogic("QF_LIA"))
	if err := c.Pop(1); err == nil {
		t.Fatal("default reset kept old scopes")
	}
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	_, err = c.DeclareFunction("retained-x", nil, integer)
	contextMust(t, err)
}

func TestCommandContextResetAssertionsConstructorRefusalKeepsResult(t *testing.T) {
	c, m, decl := resetAssertionsFixture(t)
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	checkRetainedSeven(t, c, m, decl)
	m.Budget().FailAfter(0)
	err = c.ResetAssertions()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("missing solver recreation refusal", err)
	}
	checkRetainedSeven(t, c, m, decl)
	contextMust(t, c.Reset())
	contextMust(t, c.SetLogic("QF_UF"))
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("full reset did not recover", status)
	}
}

func TestCommandContextResetAssertionsColdContextKeepsSourceLogicPolicy(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			c := contextFixture(t)
			contextMust(t, c.SetBoolOption("global-declarations", global))
			contextMust(t, c.SetLogic("QF_LIA"))
			contextMust(t, c.ResetAssertions())
			if c.manager != nil {
				t.Fatal("cold reset-assertions constructed a manager")
			}
			err := c.SetLogic("QF_UF")
			if global && err == nil {
				t.Fatal("global mode discarded selected logic")
			}
			if !global {
				contextMust(t, err)
			}
			_, err = c.Manager()
			contextMust(t, err)
		})
	}
}
