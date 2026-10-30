package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverModelDisplayAndStatistics(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", func(c *SMTContext) { astTestMust(t, c.SetBoolOption("model", true)) })
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	x := f.arithmeticVariable("api-simple-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	equality := f.pin(f.m.MkEq(x, four))
	astTestMust(t, s.Assert(equality))
	if status, e := s.Check(); e != nil || status != lbool.True {
		t.Fatal("status", status, e)
	}
	md, err := s.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(x)
	astTestMust(t, err)
	if value, e := md.ConstantInterpretation(declaration); e != nil || value.IsNull() {
		t.Fatal("model interpretation", value, e)
	}
	text, err := s.String()
	astTestMust(t, err)
	if !strings.Contains(text, "api-simple-x") {
		t.Fatal("solver display", text)
	}
	stats, err := s.Statistics()
	astTestMust(t, err)
	if size, e := stats.Size(); e != nil || size == 0 {
		t.Fatal("solver statistics", size, e)
	}
	astTestMust(t, stats.Close())
	astTestMust(t, md.Close())
	astTestMust(t, s.Close())
}
