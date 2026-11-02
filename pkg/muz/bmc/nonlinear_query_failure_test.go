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
	"github.com/glycerine/g3/pkg/util/memory"
)

// Source: dl_bmc_engine.cpp nonlinear::compile_query and apply_vars. Query
// applications and universally bound rule-choice applications publish only
// after all temporary declaration/argument owners can be released.
func TestBMCNonlinearQueryAndAppliedVariablesAllocationFrontiers(t *testing.T) {
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
			t.Errorf("nonlinear query fixture left %d bytes", used)
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
	name, err := m.Symbols.Intern("nonlinear-query-frontier")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rm.RegisterPredicate(predicate))
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{yes, no})
	must(err)
	rule, err := rm.NewRule(head, nil, nil, "frontier-fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("nonlinear query fixture did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()

	// Warm deterministic level/argument symbols before exact accounting.
	query, err := compiler.CompileQuery(predicate, 3)
	must(err)
	must(m.DelRef(&query))
	formulas, err := compiler.Compile(set, 3)
	must(err)
	for i := range formulas {
		must(m.DelRef(&formulas[i]))
	}
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

	queryComplete := false
	for stop := int64(0); stop < 256 && !queryComplete; stop++ {
		before := budget.Used()
		budget.FailAfter(stop)
		candidate, cause := compiler.CompileQuery(predicate, 3)
		budget.FailAfter(-1)
		if cause != nil && candidate != (ast.ASTRef{}) {
			t.Fatal("failed nonlinear query published a reference", stop)
		}
		if candidate != (ast.ASTRef{}) {
			must(m.DelRef(&candidate))
		}
		if cause == nil {
			queryComplete = true
		} else {
			checkFailure(stop, cause)
		}
		if after := budget.Used(); after != before {
			t.Fatalf("query frontier %d changed live bytes: %d -> %d", stop, before, after)
		}
	}
	if !queryComplete {
		t.Fatal("query publication campaign did not complete")
	}

	compileComplete, compileFailures := false, 0
	for stop := int64(0); stop < 1024 && !compileComplete; stop++ {
		before := budget.Used()
		budget.FailAfter(stop)
		candidate, cause := compiler.Compile(set, 3)
		budget.FailAfter(-1)
		if cause != nil && len(candidate) != 0 {
			t.Fatal("failed nonlinear compilation published formulas", stop, len(candidate))
		}
		for i := range candidate {
			must(m.DelRef(&candidate[i]))
		}
		if cause == nil {
			compileComplete = true
		} else {
			checkFailure(stop, cause)
			compileFailures++
		}
		if after := budget.Used(); after != before {
			t.Fatalf("compile frontier %d changed live bytes: %d -> %d", stop, before, after)
		}
	}
	if !compileComplete || compileFailures == 0 {
		t.Fatal("compile campaign did not cross failure and success", compileFailures, compileComplete)
	}
}
