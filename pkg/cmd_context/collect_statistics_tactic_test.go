package cmd_context

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestCollectStatisticsBuiltinTactic(t *testing.T) {
	c := contextFixture(t)
	if !IsBuiltinTactic("collect-statistics") {
		t.Fatal("tactic missing from registry")
	}
	var output bytes.Buffer
	selected, err := c.NewBuiltinTacticWithOutput("collect-statistics", &output)
	contextMust(t, err)
	contextMust(t, selected.IncRef())
	t.Cleanup(func() { contextMust(t, selected.DecRef()) })
	m, err := c.Manager()
	contextMust(t, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, goal.Close()) })
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	_, err = c.DeclareFunction("statistics-test", nil, boolean)
	contextMust(t, err)
	atom, err := c.MakeApp("statistics-test", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, goal.Assert(atom))
	result := &tactic.TacticResult{}
	contextMust(t, selected.Apply(goal, result))
	contextMust(t, result.Close())
	if !strings.Contains(output.String(), ":uninterpreted-constants    1") {
		t.Fatalf("statistics output: %q", output.String())
	}
}
