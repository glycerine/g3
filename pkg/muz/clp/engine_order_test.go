package clp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func clpOrderedRules(t *testing.T, f *fixture, tails ...[]ast.ASTHandle) *base.RuleSet {
	t.Helper()
	set, err := base.NewRuleSet(f.rm)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, tail := range tails {
		rule, err := f.rm.NewRule(f.app["Q"], tail, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if err = set.Add(rule); err != nil {
			t.Fatal(err)
		}
		if err = rule.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fact, err := f.rm.NewRule(f.app["P"], nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(fact); err != nil {
		t.Fatal(err)
	}
	if err = fact.Close(); err != nil {
		t.Fatal(err)
	}
	if ok, err := set.Finalize(); err != nil || !ok {
		t.Fatal("finalize", ok, err)
	}
	return set
}

func TestCLPSortsFactsBeforeRecursiveRules(t *testing.T) {
	f := newFixture(t)
	set := clpOrderedRules(t, f, []ast.ASTHandle{f.app["P"]}, nil)
	f.solver.checks = []lbool.LBool{lbool.False, lbool.True}
	engine, err := clp.NewEngine(f.m, set, f.solver, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result, err := engine.Query(f.app["Q"])
	if err != nil || result != lbool.Undef {
		t.Fatal("ordered depth frontier", result, err)
	}
	if f.solver.pushes != 2 || f.solver.pops != 2 {
		t.Fatal("scopes", f.solver.pushes, f.solver.pops)
	}
}

func TestCLPStableRuleOrderAndBacktracking(t *testing.T) {
	f := newFixture(t)
	set := clpOrderedRules(t, f, []ast.ASTHandle{f.app["Q"]}, []ast.ASTHandle{f.app["P"]})
	engine, err := clp.NewEngine(f.m, set, f.solver, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result, err := engine.Query(f.app["Q"])
	if err != nil || result != lbool.True {
		t.Fatal("backtracking", result, err)
	}
	if f.solver.pushes != 6 || f.solver.pops != 6 {
		t.Fatal("stable traversal scopes", f.solver.pushes, f.solver.pops)
	}
}
