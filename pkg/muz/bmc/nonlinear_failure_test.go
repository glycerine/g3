package bmc_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

// Source: dl_bmc_engine.cpp nonlinear::get_proof/get_model. A selected join
// recursively owns both fact branches while assembling its hyper-resolution
// proof, and every partial owner must be retired on construction failure.
func TestBMCNonlinearTraceAndProofAllocationFrontiers(t *testing.T) {
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
			t.Errorf("nonlinear frontier fixture left %d bytes", used)
		}
	}()
	services, err := context.ModelEvaluatorConfig()
	must(err)
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
		decl, cause := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(rm.RegisterPredicate(decl))
		return decl
	}
	a, b, goal := declare("nonlinear-frontier-a"), declare("nonlinear-frontier-b"), declare("nonlinear-frontier-goal")
	application := func(decl ast.ASTHandle) ast.ASTHandle {
		value, cause := m.MkApp(decl, nil)
		must(cause)
		return value
	}
	add := func(name string, head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, cause := rm.NewRule(head, tails, nil, name)
		must(cause)
		formula, cause := rule.ToFormula()
		must(cause)
		value, cause := m.RefGet(formula)
		must(cause)
		proof, cause := m.MkProofAsserted(value)
		must(cause)
		must(rule.SetProof(proof))
		must(m.DelRef(&formula))
		must(set.Add(rule))
		must(rule.Close())
	}
	scope, err := ast.NewScopedProof(m)
	must(err)
	add("fact-a", application(a))
	add("fact-b", application(b))
	add("join", application(goal), application(a), application(b))
	must(scope.Close())
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("nonlinear frontier fixture did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	raw, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(raw.Close()) }()
	solver, ok := raw.(*smtcore.SMTSolver)
	if !ok {
		t.Fatal("expected native SMT solver")
	}
	status, level, err := compiler.SearchNonlinear(set, goal, solver)
	must(err)
	if status != lbool.True || level != 1 {
		t.Fatal("nonlinear frontier search", status, level)
	}
	model, err := solver.AcquireModel()
	must(err)
	defer func() { must(model.DecRef()) }()

	// Warm deterministic declarations and evaluator caches before requiring
	// per-call live-byte equality at each refused allocation.
	trace, err := compiler.NonlinearTrace(set, goal, level, model, services)
	must(err)
	for i := len(trace) - 1; i >= 0; i-- {
		must(trace[i].Close())
	}
	proof, err := compiler.NonlinearProof(set, goal, level, model, services)
	must(err)
	must(m.DelRef(&proof))

	checkFailure := func(stop int64, cause error) {
		t.Helper()
		var construction memory.ConstructionFailure
		if errors.As(cause, &construction) {
			must(construction.Cleanup())
		}
		var memout memory.Failure
		if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
			t.Fatalf("allocation frontier %d: %v", stop, cause)
		}
	}
	run := func(name string, limit int64, invoke func() ([]*base.Rule, ast.ASTRef, error)) {
		t.Helper()
		complete, failures := false, 0
		for stop := int64(0); stop < limit && !complete; stop++ {
			before := budget.Used()
			budget.FailAfter(stop)
			candidateTrace, candidateProof, cause := invoke()
			budget.FailAfter(-1)
			for i := len(candidateTrace) - 1; i >= 0; i-- {
				must(candidateTrace[i].Close())
			}
			if candidateProof != (ast.ASTRef{}) {
				must(m.DelRef(&candidateProof))
			}
			if cause == nil {
				complete = true
			} else {
				checkFailure(stop, cause)
				failures++
			}
			if after := budget.Used(); after != before {
				t.Fatalf("%s frontier %d changed live bytes: %d -> %d", name, stop, before, after)
			}
		}
		if !complete || failures == 0 {
			t.Fatalf("%s campaign did not cross failure and success: %d %v", name, failures, complete)
		}
	}
	run("trace", 512, func() ([]*base.Rule, ast.ASTRef, error) {
		candidate, cause := compiler.NonlinearTrace(set, goal, level, model, services)
		return candidate, ast.ASTRef{}, cause
	})
	run("proof", 1024, func() ([]*base.Rule, ast.ASTRef, error) {
		candidate, cause := compiler.NonlinearProof(set, goal, level, model, services)
		return nil, candidate, cause
	})
}
