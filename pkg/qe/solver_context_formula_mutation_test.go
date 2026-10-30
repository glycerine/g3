package qe

import "testing"

func TestSolverContextMutationFormulaReplacement(t *testing.T) {
	f := newSolverMutationFixture(t)
	if _, present, err := f.context.CurrentFormula(); err != nil || present {
		t.Fatalf("initial formula present=%v err=%v", present, err)
	}
	first := f.base.own(f.base.r.manager.MkEq(f.x, f.y))
	second := f.base.own(f.base.r.manager.MkEq(f.x, f.z))
	fmMust(t, f.context.EliminateVariable(2, first, f.x))
	fmMust(t, f.context.EliminateVariable(1, second, f.x))
	formula, present, err := f.context.CurrentFormula()
	if err != nil || !present || formula != second {
		t.Fatalf("replacement=%v present=%v err=%v", formula, present, err)
	}
}
