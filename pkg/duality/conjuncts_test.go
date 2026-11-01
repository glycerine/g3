package duality

import "testing"

func TestCollectConjunctsSourcePolarityConstantsAndMultiplicity(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := ctx.BoolConst("conj-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("conj-q")
	must(err)
	closeExpr(t, q)
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	negP, err := p.Not()
	must(err)
	closeExpr(t, negP)
	pair, err := p.And(q)
	must(err)
	closeExpr(t, pair)
	repeated, err := pair.And(p)
	must(err)
	closeExpr(t, repeated)
	withTrue, err := repeated.And(truth)
	must(err)
	closeExpr(t, withTrue)
	disj, err := negP.Or(q)
	must(err)
	closeExpr(t, disj)
	negQ, err := q.Not()
	must(err)
	closeExpr(t, negQ)
	negDisj, err := disj.Not()
	must(err)
	closeExpr(t, negDisj)
	implication, err := p.Implies(q)
	must(err)
	closeExpr(t, implication)
	before := m.Stats()
	for _, c := range []struct {
		term *Expr
		pos  bool
		want []*Expr
	}{
		{withTrue, true, []*Expr{p, q, p}},
		{negDisj, true, []*Expr{p, negQ}},
		{disj, false, []*Expr{p, negQ}},
		{truth, true, nil}, {falsity, false, nil},
		{falsity, true, []*Expr{falsity}},
		{implication, true, []*Expr{implication}},
	} {
		got, err := CollectConjuncts(c.term, c.pos)
		must(err)
		if len(got) != len(c.want) {
			t.Fatal("conjunct count", len(got), len(c.want))
		}
		for n, term := range got {
			h, err := term.Raw()
			must(err)
			want, err := c.want[n].Raw()
			must(err)
			if h != want {
				t.Fatal("conjunct order/polarity", n)
			}
			must(term.Close())
		}
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("conjunct collection leaked", before, after)
	}
}
