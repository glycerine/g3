package qe

import "testing"

func TestSolverContextMutationAddVariablePreservesAppendOrder(t *testing.T) {
	f := newSolverMutationFixture(t)
	w := f.base.constant("solver-mutation-w", f.base.integer)
	fmMust(t, f.context.AddVariable(w))
	variables, err := f.context.Variables()
	fmMust(t, err)
	if len(variables) != 4 || variables[3] != w {
		t.Fatalf("variables=%v", variables)
	}
	variables[0] = w
	first, err := f.context.Variable(0)
	if err != nil || first != f.x {
		t.Fatalf("detached slice mutated context: %v %v", first, err)
	}
	contains, err := f.context.Contains(3)
	fmMust(t, err)
	target, err := contains.Target()
	if err != nil || target != w {
		t.Fatalf("added target=%v err=%v", target, err)
	}
}
