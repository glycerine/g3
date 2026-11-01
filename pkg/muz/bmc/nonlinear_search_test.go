package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
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
	scope, err := ast.NewScopedProof(m)
	must(err)
	for _, rule := range trace {
		formula, err := rule.ToFormula()
		must(err)
		value, err := m.RefGet(formula)
		must(err)
		proof, err := m.MkProofAsserted(value)
		must(err)
		must(rule.SetProof(proof))
		must(m.DelRef(&formula))
	}
	must(scope.Close())
	proofRef, err := compiler.NonlinearProof(set, goal, level, md, config)
	must(err)
	proof, err := m.RefGet(proofRef)
	must(err)
	fact, err := m.GetProofFact(proof)
	must(err)
	if fact != app(goal, zero) {
		t.Fatal("variable nonlinear proof conclusion", fact, app(goal, zero))
	}
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(proof)
	must(err)
	defer func() {
		for _, condition := range side {
			must(m.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("variable nonlinear proof is invalid")
	}
	for _, condition := range side {
		must(smt.Push())
		negated, e := m.MkNot(condition)
		must(e)
		must(smt.AssertFormula(negated))
		checked, e := smt.CheckSat(nil)
		must(e)
		must(smt.Pop(1))
		if checked != lbool.False {
			t.Fatal("invalid variable nonlinear proof side condition", checked)
		}
	}
	must(m.DelRef(&proofRef))
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
