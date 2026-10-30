package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISolverFactoryLogicSelectedSolver(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", func(c *SMTContext) {
		astTestMust(t, c.SetBoolOption("model", true))
	})
	s, err := f.c.NewSolverForLogic("QF_LIA")
	astTestMust(t, err)
	x := f.arithmeticVariable("api-factory-logic-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	astTestMust(t, s.Assert(f.pin(f.m.MkEq(x, four))))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.True {
		t.Fatal("status", status, checkErr)
	}
	md, err := s.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(x)
	astTestMust(t, err)
	if value, valueErr := md.ConstantInterpretation(declaration); valueErr != nil || value.IsNull() {
		t.Fatal("model interpretation", value, valueErr)
	}
	astTestMust(t, md.Close())
	astTestMust(t, s.Close())
}

func TestAPISolverFactoryRejectsUnknownLogic(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if solver, err := f.c.NewSolverForLogic("NOT_A_Z3_LOGIC"); err == nil || solver != nil {
		t.Fatal("unknown logic accepted", solver, err)
	}
}
