package api

import (
	"math/big"
	"testing"
)

func TestAPIOptimizationObjectivesBuildMaxSMTPenaltySum(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	boolean := f.pin(f.m.MkBoolSort())
	leftName, err := f.m.Symbols.Intern("soft-query-left")
	astTestMust(t, err)
	rightName, err := f.m.Symbols.Intern("soft-query-right")
	astTestMust(t, err)
	left := f.pin(f.m.MkConst(leftName, boolean))
	right := f.pin(f.m.MkConst(rightName, boolean))
	astTestMust(t, f.c.AssertSoft(left, big.NewRat(2, 1), "group"))
	astTestMust(t, f.c.AssertSoft(right, big.NewRat(3, 1), "group"))
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	if len(objectives) != 1 {
		t.Fatal("soft group count", len(objectives))
	}
	sum, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if kind, kindErr := f.m.GetDeclKind(sum); kindErr != nil || kind != 6 {
		t.Fatal("soft sum", kind, kindErr)
	}
	if count, countErr := f.m.GetNumArgs(sum); countErr != nil || count != 2 {
		t.Fatal("soft terms", count, countErr)
	}
	for i := uint32(0); i < 2; i++ {
		term, termErr := f.m.GetArg(sum, i)
		astTestMust(t, termErr)
		if kind, kindErr := f.m.GetDeclKind(term); kindErr != nil || kind != 4 {
			t.Fatal("soft penalty ite", i, kind, kindErr)
		}
	}
	astTestMust(t, f.m.DelRef(&objectives[0]))
}
