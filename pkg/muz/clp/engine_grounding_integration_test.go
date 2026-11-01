package clp_test

import (
	"github.com/glycerine/g3/pkg/muz/clp"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	usolver "github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type recordingCLPSolver struct {
	usolver.Solver
	results []lbool.LBool
}

func (s *recordingCLPSolver) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	result, err := s.Solver.CheckSat(assumptions)
	if err == nil {
		s.results = append(s.results, result)
	}
	return result, err
}

func TestCLPRuleVariablesShareGroundingAcrossHeadAndBody(t *testing.T) {
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
	p, q, r := decl("ground-p"), decl("ground-q"), decl("ground-r")
	goalName, err := m.Symbols.Intern("ground-goal")
	must(err)
	goalDecl, err := m.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rm.RegisterPredicate(goalDecl))
	goal, err := m.MkApp(goalDecl, nil)
	must(err)
	app := func(d ast.ASTHandle, arg ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, []ast.ASTHandle{arg})
		must(err)
		return h
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	addRule := func(head ast.ASTHandle, tails []ast.ASTHandle) {
		r, err := rm.NewRule(head, tails, nil, "")
		must(err)
		must(set.Add(r))
		must(r.Close())
	}
	addRule(app(p, zero), nil)
	addRule(app(p, one), nil)
	addRule(app(r, one), nil)
	x, err := m.MkVar(0, integer)
	must(err)
	positive, err := m.MkArithmeticGT(x, zero)
	must(err)
	addRule(app(q, x), []ast.ASTHandle{app(p, x), positive})
	addRule(goal, []ast.ASTHandle{app(p, x), app(r, x)})
	ok, err := set.Finalize()
	must(err)
	if !ok {
		t.Fatal("rule set did not finalize")
	}
	solver, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(solver.Close()) }()
	recording := &recordingCLPSolver{Solver: solver}
	engine, err := clp.NewEngine(m, set, recording, 20)
	must(err)
	defer func() { must(engine.Close()) }()
	for _, tc := range []struct {
		value ast.ASTHandle
		want  lbool.LBool
	}{{zero, lbool.False}, {one, lbool.True}} {
		query := app(q, tc.value)
		ref, err := m.NewRef(query)
		must(err)
		got, err := engine.Query(query)
		must(err)
		must(m.DelRef(&ref))
		if got != tc.want {
			t.Fatalf("query %v = %v, want %v", tc.value, got, tc.want)
		}
	}
	// Query grounding must tolerate free-variable index gaps; variables are
	// replaced by fresh constants without changing the omitted indices.
	sparse, err := m.MkVar(3, integer)
	must(err)
	sparseQuery := app(q, sparse)
	sparseRef, err := m.NewRef(sparseQuery)
	must(err)
	got, err := engine.Query(sparseQuery)
	must(err)
	must(m.DelRef(&sparseRef))
	if got != lbool.True {
		t.Fatal("sparse-variable query", got)
	}
	recording.results = nil
	got, err = engine.Query(goal)
	must(err)
	if got != lbool.True {
		t.Fatal("backtracking query", got)
	}
	want := []lbool.LBool{lbool.True, lbool.True, lbool.False, lbool.True, lbool.True}
	if len(recording.results) != len(want) {
		t.Fatalf("solver checks %v, want %v", recording.results, want)
	}
	for i := range want {
		if recording.results[i] != want[i] {
			t.Fatalf("solver checks %v, want %v", recording.results, want)
		}
	}
}
