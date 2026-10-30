package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverParallelExchangePublishesAndImportsRootUnits(t *testing.T) {
	left, _, _ := satSolverCheckFixture(t)
	right, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, err := left.MkVar(false, true)
		satOK(t, err)
		_, err = right.MkVar(false, true)
		satOK(t, err)
	}
	exchange := NewParallelUnitExchange()
	t.Cleanup(func() { satOK(t, exchange.Close()) })
	satOK(t, left.SetParallelExchange(exchange))
	satOK(t, right.SetParallelExchange(exchange))
	satOK(t, left.propagation.assign(MakeLiteral(0, false), satJustification{}))
	satOK(t, left.conflict.exchangeParallel())
	satOK(t, right.conflict.exchangeParallel())
	value, err := right.sourceValueVar(0)
	satOK(t, err)
	if value != lbool.True {
		t.Fatal(value)
	}
	satOK(t, right.propagation.assign(MakeLiteral(1, true), satJustification{}))
	satOK(t, right.conflict.exchangeParallel())
	satOK(t, left.conflict.exchangeParallel())
	value, err = left.sourceValueVar(1)
	satOK(t, err)
	if value != lbool.False {
		t.Fatal(value)
	}
}
