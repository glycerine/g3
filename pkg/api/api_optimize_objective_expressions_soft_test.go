package api

import (
	"math/big"
	"testing"
)

func TestAPIOptimizeObjectiveExpressionsBuildSoftPenaltySum(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	boolean := f.pin(f.m.MkBoolSort())
	name, err := f.m.Symbols.Intern("api-opt-objective-soft")
	if err != nil {
		t.Fatal(err)
	}
	p := f.pin(f.m.MkConst(name, boolean))
	if _, err = f.opt.AssertSoft(p, big.NewRat(7, 2), "group"); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.ObjectiveExpressions()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	sum, err := f.m.RefGet(refs[0])
	if err != nil {
		t.Fatal(err)
	}
	if kind, e := f.m.GetDeclKind(sum); e != nil || kind != 6 {
		t.Fatal(kind, e)
	}
	term, err := f.m.GetArg(sum, 0)
	if err != nil {
		t.Fatal(err)
	}
	if kind, e := f.m.GetDeclKind(term); e != nil || kind != 4 {
		t.Fatal(kind, e)
	}
	if err = f.m.DelRef(&refs[0]); err != nil {
		t.Fatal(err)
	}
}
