package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISolverFactoryTacticBackedSolverRetainsTactic(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) {
		astTestMust(t, c.SetBoolOption("model", true))
	})
	tacticValue, err := f.c.Tactic("smt")
	astTestMust(t, err)
	s, err := f.c.NewSolverFromTactic(tacticValue)
	astTestMust(t, err)
	astTestMust(t, tacticValue.Close())
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-factory-tactic-p", boolean))
	astTestMust(t, s.Assert(p))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.True {
		t.Fatal("status", status, checkErr)
	}
	md, err := s.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(p)
	astTestMust(t, err)
	if value, valueErr := md.ConstantInterpretation(declaration); valueErr != nil || value.IsNull() {
		t.Fatal("model interpretation", value, valueErr)
	}
	astTestMust(t, md.Close())
	astTestMust(t, s.Close())
}
