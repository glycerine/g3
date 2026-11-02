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

// Source: dl_bmc_engine.cpp linear::get_model. The source builds an owning
// rule trace and a hyper-resolution proof while walking from the query to a
// fact; every partially built owner must be released when construction fails.
func TestBMCLinearTraceOwnershipAndProofAllocationFrontiers(t *testing.T) {
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
			t.Errorf("linear ownership fixture left %d bytes", used)
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
	fact, middle, goal := declare("linear-frontier-fact"), declare("linear-frontier-middle"), declare("linear-frontier-goal")
	application := func(decl ast.ASTHandle) ast.ASTHandle {
		value, cause := m.MkApp(decl, nil)
		must(cause)
		return value
	}
	add := func(name string, head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, cause := rm.NewRule(head, tails, nil, name)
		must(cause)
		must(set.Add(rule))
		must(rule.Close())
	}
	add("fact", application(fact))
	add("middle", application(middle), application(fact))
	add("goal", application(goal), application(middle))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("linear frontier fixture did not finalize")
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
	status, level, err := compiler.SearchLinear(set, goal, solver, 3)
	must(err)
	if status != lbool.True || level != 2 {
		t.Fatal("linear frontier search", status, level)
	}
	model, err := solver.AcquireModel()
	must(err)
	defer func() { must(model.DecRef()) }()

	// Warm deterministic names and proof services before measuring per-call
	// cleanup, then verify the successful values remain independently owned.
	trace, err := compiler.LinearTrace(set, goal, level, model)
	must(err)
	for i := len(trace) - 1; i >= 0; i-- {
		must(trace[i].Close())
	}
	proof, err := compiler.LinearProof(set, goal, level, model, services)
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

	traceComplete := false
	for stop := int64(0); stop < 256 && !traceComplete; stop++ {
		before := budget.Used()
		budget.FailAfter(stop)
		candidate, cause := compiler.LinearTrace(set, goal, level, model)
		budget.FailAfter(-1)
		for i := len(candidate) - 1; i >= 0; i-- {
			must(candidate[i].Close())
		}
		if cause == nil {
			traceComplete = true
		} else {
			checkFailure(stop, cause)
		}
		if after := budget.Used(); after != before {
			t.Fatalf("trace frontier %d changed live bytes: %d -> %d", stop, before, after)
		}
	}
	if !traceComplete {
		t.Fatal("linear trace ownership campaign did not complete")
	}

	proofComplete, proofFailures := false, 0
	for stop := int64(0); stop < 512 && !proofComplete; stop++ {
		before := budget.Used()
		budget.FailAfter(stop)
		candidate, cause := compiler.LinearProof(set, goal, level, model, services)
		budget.FailAfter(-1)
		if candidate != (ast.ASTRef{}) {
			must(m.DelRef(&candidate))
		}
		if cause == nil {
			proofComplete = true
		} else {
			checkFailure(stop, cause)
			proofFailures++
		}
		if after := budget.Used(); after != before {
			t.Fatalf("proof frontier %d changed live bytes: %d -> %d", stop, before, after)
		}
	}
	if !proofComplete || proofFailures == 0 {
		t.Fatal("linear proof campaign did not cross failure and success", proofFailures, proofComplete)
	}
}
