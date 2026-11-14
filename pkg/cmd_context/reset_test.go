package cmd_context

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandContextResetRebuildsSessionAndPreservesOptions(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			c := contextFixture(t)
			services, parameters := c.owned, c.parameters
			contextMust(t, c.Reset())
			if c.manager != nil {
				t.Fatal("empty reset eagerly constructed a manager")
			}
			contextMust(t, c.SetBoolOption("print-success", true))
			contextMust(t, c.SetBoolOption("produce-models", true))
			contextMust(t, c.SetBoolOption("global-declarations", global))
			contextMust(t, c.SetLogic("QF_UF"))
			old, err := c.Manager()
			contextMust(t, err)
			sort, err := c.DeclareSort("U")
			contextMust(t, err)
			decl, err := c.DeclareFunction("x", nil, sort)
			contextMust(t, err)
			x, err := old.MkApp(decl, nil)
			contextMust(t, err)
			contextMust(t, c.DefineFunction("same-name", nil, x))
			contextMust(t, c.Push(2))
			yes, err := old.MkTrue()
			contextMust(t, err)
			contextMust(t, c.Assert(yes, "old-assertion"))
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("old session not satisfiable", status)
			}
			contextMust(t, c.Reset())
			if old.IsLiveHandle(x) {
				t.Fatal("old manager still accepts handles")
			}
			if c.owned != services || c.parameters != parameters || !c.PrintSuccess || c.GlobalDeclarations != global {
				t.Fatal("reset discarded session option/services")
			}
			if _, err := c.Model(); err == nil {
				t.Fatal("old check result survived reset")
			}
			contextMust(t, c.SetLogic("QF_LIA"))
			fresh, err := c.Manager()
			contextMust(t, err)
			if fresh == old {
				t.Fatal("reset reused old AST manager")
			}
			integer, err := c.LookupSort("Int")
			contextMust(t, err)
			decl, err = c.DeclareFunction("x", nil, integer)
			contextMust(t, err)
			x, err = fresh.MkApp(decl, nil)
			contextMust(t, err)
			contextMust(t, c.DefineFunction("same-name", nil, x))
			if err := c.Pop(1); err == nil {
				t.Fatal("old scopes survived reset")
			}
			status, err = c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("new session failed", status)
			}
			contextMust(t, c.Reset())
			contextMust(t, c.SetBoolOption("produce-proofs", true))
			// A reset releases initialization restrictions along with the old manager.
			contextMust(t, c.SetBoolOption("global-declarations", !global))
		})
	}
}

func TestCommandContextResetClearsInitializationRefusal(t *testing.T) {
	for _, stop := range []int64{0, 8} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := contextFixture(t)
			contextMust(t, c.SetLogic("QF_UF"))
			c.owned.budget.FailAfter(stop)
			_, err := c.Manager()
			c.owned.budget.FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("missing injected initialization refusal", err)
			}
			if c.initializationError == nil {
				t.Fatal("initialization failure was not cached")
			}
			contextMust(t, c.Reset())
			contextMust(t, c.SetLogic("QF_LIA"))
			status, err := c.CheckSat(nil)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("reset did not recover initialization", status)
			}
		})
	}
}
