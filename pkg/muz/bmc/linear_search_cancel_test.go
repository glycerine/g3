package bmc_test

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::linear::check, which invokes
// bmc::checkpoint before compiling and asserting each unfolding level.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCLinearSearchCancellationPrecedesLevelCompilation(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	name, err := m.Symbols.Intern("linear-cancel-goal")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	head, err := m.MkApp(declaration, nil)
	must(err)
	rule, err := rules.NewRule(head, nil, nil, "linear-cancel-fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("linear cancellation fixture did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	raw, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(raw.Close()) }()
	solver, ok := raw.(*smtcore.SMTSolver)
	if !ok {
		t.Fatal("simple solver is not native SMT")
	}

	m.Limit.IncCancel()
	status, level, canceled := compiler.SearchLinear(set, declaration, solver, 1)
	m.Limit.DecCancel()
	if canceled == nil || status != lbool.Undef || level != 0 {
		t.Fatal("linear search ignored pre-level cancellation", status, level, canceled)
	}
	status, level, err = compiler.SearchLinear(set, declaration, solver, 1)
	must(err)
	if status != lbool.True || level != 0 {
		t.Fatal("canceled linear search mutated solver", status, level)
	}
}
