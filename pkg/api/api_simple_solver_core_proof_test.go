package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverUnsatCoreAndProofOwnResults(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) {
		astTestMust(t, c.SetBoolOption("proof", true))
		astTestMust(t, c.SetBoolOption("unsat_core", true))
	})
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	label := f.pin(f.c.Constant("api-simple-label", boolean))
	no := f.pin(f.c.Bool(false))
	astTestMust(t, s.AssertTracked(no, label))
	if status, e := s.Check(); e != nil || status != lbool.False {
		t.Fatal("status", status, e)
	}
	core, err := s.UnsatCore()
	astTestMust(t, err)
	if len(core) != 1 {
		t.Fatal("core", len(core))
	}
	proof, err := s.Proof()
	astTestMust(t, err)
	proofValue, err := f.m.RefGet(proof)
	astTestMust(t, err)
	if proofValue.IsNull() {
		t.Fatal("missing proof")
	}
	astTestMust(t, s.Close())
	if got, e := f.m.RefGet(core[0]); e != nil || got != label {
		t.Fatal("core ownership", got, e)
	}
	astTestMust(t, f.m.DelRef(&core[0]))
	astTestMust(t, f.m.DelRef(&proof))
}
