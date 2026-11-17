package api

// Z3_mk_solver builds no solver (api_solver.cpp: the Z3_solver_ref holds a
// factory; init_solver builds the solver on first use). g3 built it at
// once, so the solver's own ASTs took the ids of the caller's next terms
// and every later id moved: goivy's raft check (raft_no_assume_test.ivy,
// committed_iff_in_commit_log) then searched differently from z3 and did
// not finish. runSolverLazyMode mirrors test_vectors/solver_lazy/lazy.c;
// TestAPIASTEventOrder compares its AST events with z3's.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const (
	solverLazyDecls     = "(declare-fun f (Int) Int) "
	solverLazyGoalDecls = "(declare-fun f (Int) Int) (declare-fun a () Int) "
	solverLazyBase      = "(assert (forall ((x Int)) (! (= (f x) (+ x 1)) :pattern ((f x)))))"
	solverLazyGoal      = "(assert (= (f a) a))"
)

func solverLazyResult(r lbool.LBool) int {
	switch r {
	case lbool.True:
		return 1
	case lbool.False:
		return -1
	}
	return 0
}

// runSolverLazyMode is lazy.c <mode>; it returns the program's output.
func runSolverLazyMode(t *testing.T, mode int) string {
	f := newAPIProgramFixture(t)
	parse := func(body string) ast.ASTHandle {
		decls := solverLazyDecls
		if body == solverLazyGoal {
			decls = solverLazyGoalDecls
		}
		ref, err := f.c.ParseSMT2String(decls+body, SMT2Bindings{})
		astTestMust(t, err)
		*f.trail = append(*f.trail, ref)
		h, err := f.m.RefGet(ref)
		astTestMust(t, err)
		return h
	}
	check := func(s *APISimpleSolver) int {
		r, err := s.Check()
		astTestMust(t, err)
		return solverLazyResult(r)
	}
	var out strings.Builder
	switch mode {
	case 0:
		s, err := f.c.NewSolver()
		astTestMust(t, err)
		b := parse(solverLazyBase)
		p, err := f.c.NewParameterSet()
		astTestMust(t, err)
		astTestMust(t, p.SetBool("smt.macro_finder", true))
		astTestMust(t, s.SetParameters(p))
		astTestMust(t, p.Close())
		astTestMust(t, s.Assert(b))
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(parse(solverLazyGoal)))
		fmt.Fprintf(&out, "%d\n", check(s))
		astTestMust(t, s.Pop(1))
		fmt.Fprintf(&out, "%d\n", check(s))
		astTestMust(t, s.Close())
	case 1:
		s, err := f.c.NewSolver()
		astTestMust(t, err)
		d, err := s.ParameterDescriptions()
		astTestMust(t, err)
		astTestMust(t, d.Close())
		astTestMust(t, s.Assert(parse(solverLazyBase)))
		fmt.Fprintf(&out, "%d\n", check(s))
		astTestMust(t, s.Reset())
		astTestMust(t, s.Assert(parse(solverLazyGoal)))
		fmt.Fprintf(&out, "%d\n", check(s))
		astTestMust(t, s.Close())
	case 3:
		const text = "(declare-fun f (Int) Int) (declare-fun a () Int) (assert (forall ((x Int)) (= (f x) (+ x 1))))"
		for range 2 {
			ref, err := f.c.ParseSMT2String(text, SMT2Bindings{})
			astTestMust(t, err)
			*f.trail = append(*f.trail, ref)
		}
		out.WriteString("ok\n")
	case 4:
		s, err := f.c.NewSolverForLogic("UFLIA")
		astTestMust(t, err)
		astTestMust(t, s.Assert(parse(solverLazyBase)))
		fmt.Fprintf(&out, "%d\n", check(s))
		astTestMust(t, s.Close())
	default:
		s, err := f.c.NewSolverForLogic("UFLIA")
		astTestMust(t, err)
		u, err := f.c.NewSimpleSolver()
		astTestMust(t, err)
		astTestMust(t, s.Assert(parse(solverLazyBase)))
		astTestMust(t, u.Assert(parse(solverLazyGoal)))
		a, err := s.ScopeLevel()
		astTestMust(t, err)
		b, err := u.ScopeLevel()
		astTestMust(t, err)
		fmt.Fprintf(&out, "%d %d\n", a, b)
		astTestMust(t, u.Close())
		astTestMust(t, s.Close())
	}
	return out.String()
}

func TestAPISolverLazyModesAnswer(t *testing.T) {
	for mode, want := range []string{"-1\n1\n", "1\n1\n", "0 0\n", "ok\n", "1\n"} {
		if got := runSolverLazyMode(t, mode); got != want {
			t.Fatalf("mode %d: %q, want %q (lazy.c against build/libz3)", mode, got, want)
		}
	}
}

// A solver handle makes no AST until its first use: the caller's next
// term gets the id it would get with no solver at all.
func TestAPISolverIsBuiltOnFirstUse(t *testing.T) {
	firstID := func(withSolver bool) uint32 {
		f := newAPIProgramFixture(t)
		if withSolver {
			s, err := f.c.NewSolver()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
		}
		ref, err := f.c.ParseSMT2String(solverLazyDecls+solverLazyBase, SMT2Bindings{})
		astTestMust(t, err)
		*f.trail = append(*f.trail, ref)
		h, err := f.m.RefGet(ref)
		astTestMust(t, err)
		id, err := f.m.GetASTID(h)
		astTestMust(t, err)
		return id
	}
	if plain, after := firstID(false), firstID(true); plain != after {
		t.Fatalf("Z3_mk_solver made ASTs: the parsed term is #%d after it, #%d without", after, plain)
	}
}
