package sat

import "testing"

func TestParallelUnitExchangeLimitTracksFinalSharedSize(t *testing.T) {
	p := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, p.Close()) })
	_, limit, err := p.Exchange([]Literal{MakeLiteral(0, false), MakeLiteral(1, false)}, 7)
	satOK(t, err)
	if limit != 2 {
		t.Fatal(limit)
	}
	out, limit, err := p.Exchange(nil, 1)
	satOK(t, err)
	if len(out) != 1 || out[0] != MakeLiteral(1, false) || limit != 2 {
		t.Fatal(out, limit)
	}
}
