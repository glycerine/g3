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

func TestParallelUnitExchangeLiteralPolarityAndDetachedOutput(t *testing.T) {
	p := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, p.Close()) })
	positive := MakeLiteral(7, false)
	negative := MakeLiteral(7, true)
	out, limit, err := p.Exchange([]Literal{positive, negative, positive}, 42)
	satOK(t, err)
	if len(out) != 0 || limit != 2 {
		t.Fatal("initial exchange", out, limit)
	}
	out, limit, err = p.Exchange(nil, 0)
	satOK(t, err)
	if len(out) != 2 || out[0] != positive || out[1] != negative || limit != 2 {
		t.Fatal("polarity or order", out, limit)
	}
	out[0] = negative
	snapshot, err := p.Units()
	satOK(t, err)
	if snapshot[0] != positive {
		t.Fatal("caller modified shared unit storage")
	}
}
