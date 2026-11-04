package clp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: clp_context.cpp imp::search retains l_undef across subsequent
// unsatisfiable rules, but a later successful derivation returns l_true.
func TestCLPSearchSourceBranchStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		checks []lbool.LBool
		want   lbool.LBool
	}{
		{"both unsatisfiable", []lbool.LBool{lbool.False, lbool.False}, lbool.False},
		{"frontier before unsatisfiable", []lbool.LBool{lbool.True, lbool.False}, lbool.Undef},
		{"unsatisfiable before frontier", []lbool.LBool{lbool.False, lbool.True}, lbool.Undef},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			set := clpOrderedRules(t, f, []ast.ASTHandle{f.app["Q"]}, []ast.ASTHandle{f.app["P"]})
			f.solver.checks = append([]lbool.LBool(nil), tc.checks...)
			engine, err := clp.NewEngine(f.m, set, f.solver, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Close()
			got, err := engine.Query(f.app["Q"])
			if err != nil || got != tc.want {
				t.Fatalf("query = %v, %v; want %v", got, err, tc.want)
			}
			if len(f.solver.checks) != 0 || f.solver.pushes != 2 || f.solver.pops != 2 || f.solver.level != 0 {
				t.Fatalf("branch traversal: checks remaining %v, pushes %d, pops %d, level %d", f.solver.checks, f.solver.pushes, f.solver.pops, f.solver.level)
			}
		})
	}
}

func TestCLPSearchSourceSuccessOverridesEarlierFrontier(t *testing.T) {
	f := newFixture(t)
	set := clpOrderedRules(t, f, []ast.ASTHandle{f.app["Q"]}, []ast.ASTHandle{f.app["P"]})
	engine, err := clp.NewEngine(f.m, set, f.solver, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	got, err := engine.Query(f.app["Q"])
	if err != nil || got != lbool.True {
		t.Fatalf("query = %v, %v; want true", got, err)
	}
	// The first Q branch reaches two depth-frontier rules; the second Q
	// branch then reaches the P fact and succeeds.
	if f.solver.pushes != 5 || f.solver.pops != 5 || f.solver.level != 0 {
		t.Fatalf("branch traversal: pushes %d, pops %d, level %d", f.solver.pushes, f.solver.pops, f.solver.level)
	}
}
