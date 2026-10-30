package qe

import "testing"

func TestSolverContextMutationCachesContainsByVariable(t *testing.T) {
	f := newSolverMutationFixture(t)
	first, err := f.context.Contains(1)
	fmMust(t, err)
	again, err := f.context.Contains(1)
	fmMust(t, err)
	if first != again {
		t.Fatal("contains cache did not preserve owner")
	}
	target, err := first.Target()
	if err != nil || target != f.y {
		t.Fatalf("target=%v err=%v", target, err)
	}
	contains, err := first.Check(f.form)
	if err != nil || !contains {
		t.Fatalf("formula contains=%v err=%v", contains, err)
	}
}
