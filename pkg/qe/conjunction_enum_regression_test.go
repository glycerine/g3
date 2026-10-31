package qe

import "testing"

func TestConjunctionEqualityExtractionDistinguishesArithmeticLE(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("conj-regression-x", f.integer)
	y := f.constant("conj-regression-y", f.integer)
	forward := f.own(m.MkArithmeticLE(x, y))
	reverse := f.own(m.MkArithmeticLE(y, x))
	single, err := NewConjunctionEnum(m, forward)
	fmMust(t, err)
	defer func() { fmMust(t, single.Close()) }()
	equalities, err := single.ExtractEqualities()
	fmMust(t, err)
	if len(equalities) != 0 {
		for i := range equalities {
			fmMust(t, m.DelRef(&equalities[i]))
		}
		t.Fatal("a single inequality was treated as an equality")
	}
	both := f.own(m.MkAnd(forward, reverse))
	paired, err := NewConjunctionEnum(m, both)
	fmMust(t, err)
	defer func() { fmMust(t, paired.Close()) }()
	equalities, err = paired.ExtractEqualities()
	fmMust(t, err)
	defer func() {
		for i := range equalities {
			fmMust(t, m.DelRef(&equalities[i]))
		}
	}()
	if len(equalities) != 1 {
		t.Fatal("opposing inequalities did not form one equality")
	}
}
