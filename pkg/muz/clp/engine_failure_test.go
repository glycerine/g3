package clp_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestCLPSolverFailureBalancesScope(t *testing.T) {
	f := newFixture(t)
	stop := errors.New("stop")
	f.solver.fail = stop
	e, x := clp.NewEngine(f.m, f.set, f.solver, 20)
	if x != nil {
		t.Fatal(x)
	}
	defer e.Close()
	_, x = e.Query(f.app["Q"])
	if !errors.Is(x, stop) {
		t.Fatal(x)
	}
	if f.solver.pushes != 1 {
		t.Fatal(f.solver.pushes)
	}
}

func TestCLPQueryAllocationFrontiers(t *testing.T) {
	for _, variable := range []bool{false, true} {
		t.Run(fmt.Sprintf("variable=%v", variable), func(t *testing.T) {
			testCLPQueryAllocationFrontiers(t, variable)
		})
	}
}

func testCLPQueryAllocationFrontiers(t *testing.T, variable bool) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Errorf("CLP query left %d bytes", budget.Used())
				}
			})
			f := newFixture(t)
			budget = f.m.Budget()
			rules, query := f.set, f.app["Q"]
			if variable {
				boolean, err := f.m.MkBoolSort()
				if err != nil {
					t.Fatal(err)
				}
				name, err := f.m.Symbols.Intern("clp-failure-unary")
				if err != nil {
					t.Fatal(err)
				}
				decl, err := f.m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				if err != nil {
					t.Fatal(err)
				}
				if err = f.rm.RegisterPredicate(decl); err != nil {
					t.Fatal(err)
				}
				truth, err := f.m.MkTrue()
				if err != nil {
					t.Fatal(err)
				}
				head, err := f.m.MkApp(decl, []ast.ASTHandle{truth})
				if err != nil {
					t.Fatal(err)
				}
				rule, err := f.rm.NewRule(head, nil, nil, "fact")
				if err != nil {
					t.Fatal(err)
				}
				rules, err = base.NewRuleSet(f.rm)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = rules.Close() })
				if err = rules.Add(rule); err != nil {
					t.Fatal(err)
				}
				if err = rule.Close(); err != nil {
					t.Fatal(err)
				}
				if ok, err := rules.Finalize(); err != nil || !ok {
					t.Fatal("finalize", ok, err)
				}
				x, err := f.m.MkVar(0, boolean)
				if err != nil {
					t.Fatal(err)
				}
				query, err = f.m.MkApp(decl, []ast.ASTHandle{x})
				if err != nil {
					t.Fatal(err)
				}
				ref, err := f.m.NewRef(query)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = f.m.DelRef(&ref) })
			}
			engine, err := clp.NewEngine(f.m, rules, f.solver, 20)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				budget.FailAfter(-1)
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			}()
			budget.FailAfter(stop)
			_, err = engine.Query(query)
			budget.FailAfter(-1)
			if f.solver.pushes != f.solver.pops {
				t.Fatal("query left a solver scope open", f.solver.pushes, f.solver.pops)
			}
			if err == nil {
				success = true
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				if err := construction.Cleanup(); err != nil {
					t.Fatal(err)
				}
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal(err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("CLP query allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("did not reach both failure and success", failures, success)
	}
}

func TestCLPGroundingClosesPartialSubstitutionOnAllocationFailure(t *testing.T) {
	f := newFixture(t)
	engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	budget := f.m.Budget()
	before := budget.Used()
	defer budget.FailAfter(-1)
	// The substitution owner is allocated, then its first cache allocation
	// fails, leaving a partial owner that the caller must close.
	budget.FailAfter(1)
	_, err = engine.Query(f.app["Q"])
	budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected grounding allocation failure", err)
	}
	if after := budget.Used(); after != before {
		t.Fatalf("failed grounding changed owner bytes: before %d, after %d", before, after)
	}
	status, err := engine.Query(f.app["Q"])
	if err != nil || status != lbool.True {
		t.Fatal("grounding did not recover", status, err)
	}
}

func TestCLPSolverUnknownRaisesSourceErrorAndRestoresScope(t *testing.T) {
	f := newFixture(t)
	f.solver.checks = []lbool.LBool{lbool.Undef}
	engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	_, err = engine.Query(f.app["Q"])
	if !errors.Is(err, clp.ErrSolverUnknown) || err.Error() != "undef" {
		t.Fatal("solver unknown", err)
	}
	if f.solver.pushes != 1 || f.solver.pops != 1 {
		t.Fatal("unbalanced solver scope", f.solver.pushes, f.solver.pops)
	}
	result, err := engine.Query(f.app["Q"])
	if err != nil || result != lbool.True {
		t.Fatal("retry", result, err)
	}
}
