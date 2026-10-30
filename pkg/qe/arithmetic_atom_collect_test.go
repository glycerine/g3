package qe

import "testing"

func TestCollectArithmeticAtomsNNFPolarityAndOrder(t *testing.T) {
	f := newFMFixture(t)
	a := f.constant("collect-a", f.boolean)
	b := f.constant("collect-b", f.boolean)
	form := f.own(f.r.manager.MkAnd(a, f.own(f.r.manager.MkNot(b))))
	s, e := CollectArithmeticAtomsNNF(f.r.manager, form)
	fmMust(t, e)
	defer s.Close()
	p, e := s.Positive()
	fmMust(t, e)
	n, e := s.Negative()
	fmMust(t, e)
	if len(p) != 1 || p[0] != a || len(n) != 1 || n[0] != b {
		t.Fatal(p, n)
	}
}
