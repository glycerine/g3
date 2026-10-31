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

func TestBMCNonlinearTraceVisitsModelSelectedRules(t *testing.T) {
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
	config, err := context.ModelEvaluatorConfig()
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
		d, err := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(rm.RegisterPredicate(d))
		return d
	}
	a, b, goal := decl("trace-a"), decl("trace-b"), decl("trace-goal")
	app := func(d ast.ASTHandle) ast.ASTHandle { h, err := m.MkApp(d, nil); must(err); return h }
	add := func(name string, head ast.ASTHandle, tails []ast.ASTHandle) {
		r, err := rm.NewRule(head, tails, nil, name)
		must(err)
		must(set.Add(r))
		must(r.Close())
	}
	add("fact-a", app(a), nil)
	add("fact-b", app(b), nil)
	add("join", app(goal), []ast.ASTHandle{app(a), app(b)})
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
		t.Fatal("simple solver type")
	}
	status, level, err := compiler.SearchNonlinear(set, goal, smt)
	must(err)
	if status != lbool.True || level != 1 {
		t.Fatal("search", status, level)
	}
	md, err := smt.AcquireModel()
	must(err)
	if md == nil {
		t.Fatal("missing model")
	}
	defer func() { must(md.DecRef()) }()
	trace, err := compiler.NonlinearTrace(set, goal, level, md, config)
	must(err)
	defer func() {
		for i := len(trace) - 1; i >= 0; i-- {
			must(trace[i].Close())
		}
	}()
	want := []string{"join", "fact-a", "fact-b"}
	if len(trace) != len(want) {
		t.Fatal("trace length", len(trace))
	}
	for i, r := range trace {
		name, err := r.Name()
		must(err)
		if name != want[i] {
			t.Fatal("trace rule", i, name, want[i])
		}
	}
}
