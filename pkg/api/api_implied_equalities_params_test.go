package api

// z3 4.7.1 bug (smt_solver::updt_params): the produce_models parameter that
// Z3_get_implied_equalities passes to updt_params makes the 4.7.1
// smt_params re-read every setting from that parameter set alone, so a
// solver-level smt.mbqi=false is silently turned back on (and a quantifier
// internalized under mbqi=false then crashes the model checker). Upstream
// z3 (4.15.2) applies the merged solver parameters instead, keeping
// mbqi=false: there the check below answers unknown, while 4.7.1 answers
// sat. g3 keeps the upstream behavior.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestImpliedEqualitiesKeepsSolverParams(t *testing.T) {
	f, s := newImpliedEqualityFixture(t)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("mbqi", false))
	astTestMust(t, s.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	q := f.pin(f.c.Constant("q", f.pin(f.c.BoolSort())))
	checkImpliedEqualityIDs(t, s, []ast.ASTHandle{q}, lbool.True, []uint32{0})
	astTestMust(t, s.LoadSMT2String("(declare-fun f (Int) Int)(declare-const c Int)"+
		"(assert (forall ((x Int)) (! (> (f x) 0) :pattern ((f x)))))(assert (> (f c) 1))"))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatalf("check after implied equalities: %v; upstream z3 keeps smt.mbqi=false and answers unknown", status)
	}
}
