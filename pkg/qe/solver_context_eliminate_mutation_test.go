package qe

import "testing"

func TestSolverContextMutationEliminateUsesSourceSwapPop(t *testing.T) {
	f := newSolverMutationFixture(t)
	removed, err := f.context.Contains(1)
	fmMust(t, err)
	moved, err := f.context.Contains(2)
	fmMust(t, err)
	updated := f.base.own(f.base.r.manager.MkEq(f.x, f.z))
	fmMust(t, f.context.EliminateVariable(1, updated, f.z))
	variables, err := f.context.Variables()
	fmMust(t, err)
	if len(variables) != 2 || variables[0] != f.x || variables[1] != f.z {
		t.Fatalf("swap-pop variables=%v", variables)
	}
	contains, err := f.context.Contains(1)
	if err != nil || contains != moved {
		t.Fatalf("moved contains=%p want %p err=%v", contains, moved, err)
	}
	if _, err = removed.Target(); err == nil {
		t.Fatal("removed contains owner remained open")
	}
	formula, present, err := f.context.CurrentFormula()
	if err != nil || !present || formula != updated {
		t.Fatalf("formula=%v present=%v err=%v", formula, present, err)
	}
}
