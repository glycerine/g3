package api

import (
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
	if _, err = f.opt.AssertSoft(p, softRat(7, 2), "group"); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.ObjectiveExpressions()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	// AUDIT.md cmd_context_02-F10: arith_util::mk_add of one argument is
	// that argument, so a one-term group is the ite penalty itself.
	term, err := f.m.RefGet(refs[0])
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
