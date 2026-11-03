package bmc_test

// Source: dl_bmc_engine.cpp nonlinear::level_replacer_star temporary expr_ref
// ownership and replace_by_level_predicates result publication.

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
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCReplacePredicatesAllocationFrontierSuppressesPartialResult(t *testing.T) {
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
			t.Fatal("nonlinear level fixture leaked bytes", used)
		}
	}()
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("level-frontier-p")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	variable, err := m.MkVar(0, boolean)
	must(err)
	argument := variable
	for i := 0; i < 256; i++ {
		argument, err = m.MkNot(argument)
		must(err)
	}
	body, err := m.MkApp(predicate, []ast.ASTHandle{argument})
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	})
	must(err)
	root, err := m.NewRef(quantified)
	must(err)
	defer func() { must(m.DelRef(&root)) }()
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()

	complete, failures := false, 0
	for stop := int64(0); stop < 512 && !complete; stop++ {
		budget.FailAfter(stop)
		candidate, cause := compiler.ReplacePredicates(rules, quantified, 3)
		budget.FailAfter(-1)
		if cause == nil {
			if candidate == (ast.ASTRef{}) {
				t.Fatal("successful level replacement omitted result")
			}
			must(m.DelRef(&candidate))
			complete = true
		} else {
			if candidate != (ast.ASTRef{}) {
				t.Fatal("failed level replacement published partial result", stop)
			}
			var construction memory.ConstructionFailure
			if errors.As(cause, &construction) {
				must(construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatalf("level replacement frontier %d: %v", stop, cause)
			}
			failures++
		}
	}
	if !complete || failures == 0 {
		t.Fatal("level replacement campaign did not cross failure and success", failures, complete)
	}
}
