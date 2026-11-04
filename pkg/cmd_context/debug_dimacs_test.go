package cmd_context

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCommandContextDisplayDIMACSRestoresGlobalOption(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_FD"))
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	_, err = c.DeclareFunction("dimacs-p", nil, boolean)
	contextMust(t, err)
	p, err := c.MakeApp("dimacs-p", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(p, ""))
	var out bytes.Buffer
	contextMust(t, c.SetRegularOutput(&out))
	contextMust(t, c.DisplayDIMACS())
	if !strings.Contains(out.String(), "p cnf ") {
		t.Fatal("DIMACS command did not print the generated formula", out.String())
	}
	value, err := c.config.Globals.SourceGetValueName("sat.dimacs.display")
	contextMust(t, err)
	if value != "false" {
		t.Fatal("DIMACS display option was not reset", value)
	}
}
