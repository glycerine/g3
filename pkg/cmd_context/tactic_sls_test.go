package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandContextRegistersQFBVSLSTactic(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_BV"))
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	_, err = c.DeclareFunction("qfbv-sls-command-p", nil, boolean)
	contextMust(t, err)
	p, err := c.MakeApp("qfbv-sls-command-p", nil, nil, boolean)
	contextMust(t, err)
	contextMust(t, c.Assert(p, ""))
	strategy, err := c.NewBuiltinTactic("qfbv-sls")
	contextMust(t, err)
	contextMust(t, strategy.IncRef())
	defer strategy.DecRef()
	status, err := c.CheckSatUsing(strategy, nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
}
