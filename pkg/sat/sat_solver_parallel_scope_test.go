package sat

import "testing"

func TestSATSolverParallelExchangeRespectsScopeAndBindVariableFrontier(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	_, err := s.MkVar(false, true)
	satOK(t, err)
	exchange := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, exchange.Close()) })
	satOK(t, s.SetParallelExchange(exchange))
	_, err = s.MkVar(false, true)
	satOK(t, err)
	satOK(t, s.propagation.assign(MakeLiteral(1, false), satJustification{}))
	satOK(t, s.conflict.exchangeParallel())
	if n, err := exchange.Len(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	satOK(t, s.Lifecycle.push())
	satOK(t, s.propagation.assign(MakeLiteral(0, false), satJustification{}))
	satOK(t, s.conflict.exchangeParallel())
	if n, err := exchange.Len(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
