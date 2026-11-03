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

// Source: dl_bmc_engine.cpp qlinear::check. Each bit-width attempt owns a
// solver scope, its compiled formulas, indexed-term roots, and (on success) a
// model that must outlive the pop. Failed attempts must pop without publishing
// a model and leave the caller's solver assertions intact.
func TestBMCQuantifiedLinearSearchAllocationScopeCleanup(t *testing.T) {
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
			t.Errorf("quantified search fixture left %d bytes", used)
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
		decl, cause := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(rm.RegisterPredicate(decl))
		return decl
	}
	fact, goal := declare("qlinear-scope-fact"), declare("qlinear-scope-goal")
	application := func(decl ast.ASTHandle) ast.ASTHandle {
		value, cause := m.MkApp(decl, nil)
		must(cause)
		return value
	}
	add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, cause := rm.NewRule(head, tails, nil, "")
		must(cause)
		must(set.Add(rule))
		must(rule.Close())
	}
	add(application(fact))
	add(application(goal), application(fact))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("quantified search fixture did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	guardName, err := m.Symbols.Intern("qlinear-scope-guard")
	must(err)
	guard, err := m.MkConst(guardName, boolean)
	must(err)
	guardRef, err := m.NewRef(guard)
	must(err)
	defer func() { must(m.DelRef(&guardRef)) }()
	newSolver := func() (*smtcore.SMTSolver, func()) {
		t.Helper()
		raw, cause := context.NewSimpleSolver()
		must(cause)
		solver, ok := raw.(*smtcore.SMTSolver)
		if !ok {
			t.Fatal("expected native SMT solver")
		}
		return solver, func() { must(raw.Close()) }
	}

	// Warm indexed names, solver internals, and the winning model path.
	solver, closeSolver := newSolver()
	must(solver.AssertFormula(guard))
	status, width, model, err := compiler.SearchQuantifiedLinear(set, goal, solver)
	must(err)
	if status != lbool.True || width != 4 || model == nil {
		t.Fatal("quantified search warmup", status, width, model)
	}
	must(model.DecRef())
	closeSolver()

	complete, failures := false, 0
	allowances := []int64{0, 1, 4, 16, 64, 256, 1024, 4096, 16384, 65536, 262144, 1048576}
	for _, stop := range allowances {
		if complete {
			break
		}
		before := budget.Used()
		solver, closeSolver = newSolver()
		must(solver.AssertFormula(guard))
		budget.FailAfter(stop)
		status, width, candidate, cause := compiler.SearchQuantifiedLinear(set, goal, solver)
		budget.FailAfter(-1)
		if candidate != nil {
			must(candidate.DecRef())
		}
		if cause == nil {
			if status != lbool.True || width != 4 || candidate == nil {
				t.Fatal("quantified search success publication", status, width, candidate)
			}
			complete = true
		} else {
			if candidate != nil {
				t.Fatal("failed quantified search published a model")
			}
			var construction memory.ConstructionFailure
			if errors.As(cause, &construction) {
				must(construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatalf("quantified search frontier %d: %v", stop, cause)
			}
			failures++
		}
		depth, scopeErr := solver.ScopeLevel()
		must(scopeErr)
		if depth != 0 {
			t.Fatalf("quantified search frontier %d left solver scope %d", stop, depth)
		}

		if cause == nil {
			// The assertion below the source qlinear push survives the successful
			// attempt; its negation therefore remains inconsistent.
			negated, checkErr := m.MkNot(guard)
			must(checkErr)
			checked, checkErr := solver.CheckSat([]ast.ASTHandle{negated})
			must(checkErr)
			if checked != lbool.False {
				t.Fatalf("quantified search frontier %d damaged caller scope: %v", stop, checked)
			}
		}
		closeSolver()
		if after := budget.Used(); after != before {
			t.Fatalf("quantified search frontier %d changed live bytes: %d -> %d", stop, before, after)
		}
	}
	if !complete || failures == 0 {
		t.Fatal("quantified search campaign did not cross failure and success", failures, complete)
	}
}
