package bmc_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
)

// qlinear::get_model replaces a null result from model::eval with a variable
// of the original index and sort, while other Go failures remain errors.
func TestBMCQuantifiedLinearBindingEvaluatorException(t *testing.T) {
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
			t.Errorf("binding fixture left %d bytes", used)
		}
	}()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("qlinear-exception-binding")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, boolean)
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{x, y})
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	must(rm.RegisterPredicate(predicate))
	rule, err := rm.NewRule(head, nil, nil, "exception-binding")
	must(err)
	defer func() { must(rule.Close()) }()
	md, err := model.NewModel(m)
	must(err)
	defer func() { must(md.Close()) }()
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	services, err := context.ModelEvaluatorConfig()
	must(err)
	params, err := services.Context.NewValue()
	must(err)
	defer func() { must(services.Context.Del(&params)) }()
	key := "max_steps"
	must(services.Context.SetUIntName(&params, &key, 0))
	services.Params = &params

	// Check that an actual binding expression triggers the source catch.
	indexSort, err := m.BitVectorSort(4)
	must(err)
	argName, err := m.Symbols.Intern("qlinear-exception-binding#0")
	must(err)
	arg, err := m.MkFuncDecl(argName, []ast.ASTHandle{indexSort}, integer, ast.ASTFuncDeclInfo{})
	must(err)
	index, err := m.BitVectorNumeralRat(big.NewRat(1, 1), 4)
	must(err)
	term, err := m.MkApp(arg, []ast.ASTHandle{index})
	must(err)
	evaluator, err := model.NewModelEvaluator(md.Core(), services)
	must(err)
	value, err := evaluator.EvaluateValue(term)
	if value != (ast.ASTRef{}) || err == nil || err.Error() != messages.MaxSteps || !rewriter.IsRewriterException(err) {
		t.Fatal("expected evaluator exception", value, err)
	}
	must(evaluator.Close())

	bindings, err := compiler.QuantifiedLinearModelBindings(rule, 0, 1, 4, md.Core(), services)
	must(err)
	if len(bindings) != 2 {
		t.Fatal("binding count", len(bindings))
	}
	for i, want := range []ast.ASTHandle{x, y} {
		got, e := m.RefGet(bindings[i])
		must(e)
		if got != want {
			t.Errorf("binding %d: got %v, want variable %v", i, got, want)
		}
		must(m.DelRef(&bindings[i]))
	}

	// Allocation failures anywhere in extraction must escape, even when
	// the evaluator would otherwise throw a caught rewriter exception.
	failures, complete := 0, false
	for stop := int64(0); stop < 1024; stop++ {
		budget.FailAfter(stop)
		result, cause := compiler.QuantifiedLinearModelBindings(rule, 0, 1, 4, md.Core(), services)
		budget.FailAfter(-1)
		if cause == nil {
			for i := range result {
				must(m.DelRef(&result[i]))
			}
			complete = true
			break
		}
		if result != nil {
			t.Fatal("failed extraction published bindings", stop, result)
		}
		var pending memory.ConstructionFailure
		if errors.As(cause, &pending) {
			must(pending.Cleanup())
		}
		var memout memory.Failure
		if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
			t.Fatalf("allocation frontier %d: %v", stop, cause)
		}
		failures++
	}
	if !complete || failures == 0 {
		t.Fatal("allocation campaign missed failure or success", failures, complete)
	}
}
