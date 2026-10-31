package bmc_test

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

func TestBMCNonlinearSearchFindsFirstReachableLevel(t *testing.T) {
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
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	decl := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(rm.RegisterPredicate(d))
		return d
	}
	a, b, goal := decl("search-a"), decl("search-b"), decl("search-goal")
	app := func(d, v ast.ASTHandle) ast.ASTHandle { h, err := m.MkApp(d, []ast.ASTHandle{v}); must(err); return h }
	add := func(head ast.ASTHandle, tails []ast.ASTHandle) {
		r, err := rm.NewRule(head, tails, nil, "")
		must(err)
		must(set.Add(r))
		must(r.Close())
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	add(app(a, zero), nil)
	x, err := m.MkVar(0, integer)
	must(err)
	add(app(b, x), []ast.ASTHandle{app(a, x)})
	add(app(goal, x), []ast.ASTHandle{app(b, x), app(a, x)})
	ok, err := set.Finalize()
	must(err)
	if !ok {
		t.Fatal("rule set did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	raw, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(raw.Close()) }()
	smt, ok := raw.(*smtcore.SMTSolver)
	if !ok {
		t.Fatal("simple solver is not the native SMT solver")
	}
	status, level, err := compiler.SearchNonlinear(set, goal, smt)
	must(err)
	if status != lbool.True || level != 2 {
		t.Fatal("first reachable level", status, level)
	}
	md, err := smt.AcquireModel()
	must(err)
	if md == nil {
		t.Fatal("winning model missing")
	}
	config, err := context.ModelEvaluatorConfig()
	must(err)
	trace, err := compiler.NonlinearTrace(set, goal, level, md, config)
	must(err)
	if len(trace) != 4 {
		t.Fatal("branching rule trace", len(trace))
	}
	for i := len(trace) - 1; i >= 0; i-- {
		must(trace[i].Close())
	}
	must(md.DecRef())
	m.Limit.IncCancel()
	_, _, err = compiler.SearchNonlinear(set, goal, smt)
	m.Limit.DecCancel()
	if err == nil {
		t.Fatal("canceled search succeeded")
	}
}
