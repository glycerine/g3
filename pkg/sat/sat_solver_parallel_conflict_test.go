package sat

import "testing"

func TestSATSolverParallelExchangeImportsConflictingUnit(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	_, err := s.MkVar(false, true)
	satOK(t, err)
	exchange := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, exchange.Close()) })
	_, _, err = exchange.Exchange([]Literal{MakeLiteral(0, true)}, 0)
	satOK(t, err)
	satOK(t, s.SetParallelExchange(exchange))
	satOK(t, s.propagation.assign(MakeLiteral(0, false), satJustification{}))
	satOK(t, s.conflict.exchangeParallel())
	if !s.propagation.inconsistent {
		t.Fatal("opposite shared unit did not publish conflict")
	}
}
