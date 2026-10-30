package cmd_context

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestWriteConsequencesPublishesNativeStatusAndBlankVectorLine(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	falsity, err := m.MkFalse()
	contextMust(t, err)
	contextMust(t, c.Assert(falsity, ""))
	var out bytes.Buffer
	contextMust(t, c.WriteConsequences(&out, nil, nil))
	if out.String() != "unsat\n\n" {
		t.Fatal(out.String())
	}
	if c.status != lbool.False || !c.checked {
		t.Fatal("result ownership", c.status, c.checked)
	}
}
