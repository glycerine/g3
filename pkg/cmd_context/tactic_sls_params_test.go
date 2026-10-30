package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandContextQFBVSLSAcceptsRuntimeParameters(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_BV"))
	strategy, err := c.NewBuiltinTactic("qfbv-sls")
	contextMust(t, err)
	contextMust(t, strategy.IncRef())
	defer strategy.DecRef()
	params, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer c.config.Params.Del(&params)
	seed := "random_seed"
	contextMust(t, c.config.Params.SetUIntName(&params, &seed, 23))
	status, err := c.CheckSatUsing(strategy, &params)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
}
