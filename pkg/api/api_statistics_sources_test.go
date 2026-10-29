package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIStatisticsSolverAndFixedpointSnapshots(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("api-statistics-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	equal := f.pin(f.m.MkEq(x, four))
	astTestMust(t, f.c.Assert(equal))
	if status, err := f.c.CheckSat(); err != nil || status != lbool.True {
		t.Fatal("check", status, err)
	}
	solver, err := f.c.StatisticsObject()
	astTestMust(t, err)
	text, err := solver.String()
	astTestMust(t, err)
	if size, e := solver.Size(); e != nil || size == 0 || !strings.Contains(text, ":rlimit-count") {
		t.Fatal("solver statistics source additions", size, text, e)
	}
	astTestMust(t, solver.Close())

	fixedpoint, err := f.c.FixedpointStatisticsObject()
	astTestMust(t, err)
	if _, err = fixedpoint.String(); err != nil {
		t.Fatal("fixedpoint statistics display", err)
	}
	astTestMust(t, fixedpoint.Close())
}
