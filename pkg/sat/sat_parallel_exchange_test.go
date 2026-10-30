package sat

import "testing"

func TestParallelUnitExchangeSourceOrderAndDeduplication(t *testing.T) {
	p := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, p.Close()) })
	a := MakeLiteral(0, false)
	b := MakeLiteral(1, true)
	out, limit, err := p.Exchange([]Literal{a, a, b}, 0)
	satOK(t, err)
	if len(out) != 0 || limit != 2 {
		t.Fatal(out, limit)
	}
	c := MakeLiteral(2, false)
	out, limit, err = p.Exchange([]Literal{b, c}, 0)
	satOK(t, err)
	if len(out) != 2 || out[0] != a || out[1] != b || limit != 3 {
		t.Fatal(out, limit)
	}
}
