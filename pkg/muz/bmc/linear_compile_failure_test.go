package bmc_test

// Source: dl_bmc_engine.cpp linear::compile expr_ref_vector ownership.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestBMCLinearCompileAllocationFrontierSuppressesFormulaPrefix(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	m, err := context.Manager()
	must(err)
	budget := m.Budget()
	defer func() {
		must(context.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("linear compile fixture leaked bytes", used)
		}
	}()
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	declare := func(text string) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		must(cause)
		decl, cause := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(rm.RegisterPredicate(decl))
		return decl
	}
	predicate, goal := declare("linear-compile-frontier-p"), declare("linear-compile-frontier-goal")
	variable, err := m.MkVar(0, boolean)
	must(err)
	predicateApp, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	must(err)
	goalApp, err := m.MkApp(goal, []ast.ASTHandle{variable})
	must(err)
	rule, err := rm.NewRule(predicateApp, nil, nil, "linear-compile-fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	rule, err = rm.NewRule(goalApp, []ast.ASTHandle{predicateApp}, nil, "linear-compile-step")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	budget.FailAfter(5)
	_, preflightErr := bmc.IsLinear(set)
	budget.FailAfter(-1)
	if preflightErr != nil {
		t.Fatal("frontier stop did not pass linearity preflight", preflightErr)
	}

	complete, failures := false, 0
	// Stop five is the first compile-owned allocation after the public
	// RuleManager.HasRuleQuantifiers preflight; the unrestricted attempt then
	// supplies the successful publication side of the transaction.
	for _, stop := range []int64{5, -1} {
		budget.FailAfter(stop)
		formulas, cause := compiler.CompileLinear(set, 1)
		budget.FailAfter(-1)
		if cause == nil {
			if len(formulas) == 0 {
				t.Fatal("successful linear compile returned no formulas")
			}
			for i := len(formulas) - 1; i >= 0; i-- {
				must(m.DelRef(&formulas[i]))
			}
			complete = true
		} else {
			if len(formulas) != 0 {
				t.Fatal("failed linear compile published formula prefix", stop, len(formulas))
			}
			var construction memory.ConstructionFailure
			if errors.As(cause, &construction) {
				must(construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatalf("linear compile frontier %d: %v", stop, cause)
			}
			failures++
		}
	}
	if !complete || failures == 0 {
		t.Fatal("linear compile campaign did not cross failure and success", failures, complete)
	}
}
