package cmd_context

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestFixedpointQueryTimeoutScopeUsesLocalParametersAndRestoresCancellation(t *testing.T) {
	c := contextFixture(t)
	_, err := c.Manager()
	contextMust(t, err)
	params, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&params)) }()
	name := "timeout"
	contextMust(t, c.config.Params.SetUIntName(&params, &name, 1))
	if timeout, e := c.fixedpointTimeout(&params); e != nil || timeout != 1 {
		t.Fatal("local fixedpoint timeout", timeout, e)
	}
	scope, err := c.beginFixedpointTimeout(&params)
	contextMust(t, err)
	deadline := time.Now().Add(time.Second)
	for !c.manager.ResourceLimit().CancelFlag() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !c.manager.ResourceLimit().CancelFlag() {
		contextMust(t, scope.Close())
		t.Fatal("fixedpoint timer did not cancel")
	}
	contextMust(t, scope.Close())
	contextMust(t, scope.Close())
	if c.manager.ResourceLimit().CancelFlag() {
		t.Fatal("fixedpoint timer leaked cancellation")
	}
	contextMust(t, c.config.Params.SetUIntName(&params, &name, 0))
	if timeout, e := c.fixedpointTimeout(&params); e != nil || timeout != 0 {
		t.Fatal("disabled local fixedpoint timeout", timeout, e)
	}
	limit := c.manager.ResourceLimit()
	limit.IncCancel()
	scope, err = c.beginFixedpointTimeout(&params)
	contextMust(t, err)
	scope.handler.HandleEvent(control.EventCallerTimeout)
	contextMust(t, scope.Close())
	if limit.CancelCount() != 1 {
		t.Fatal("fixedpoint timer cleared an independent cancellation", limit.CancelCount())
	}
	limit.DecCancel()
}
