package arith

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestEq2BVSourceTraceAndRefusedStream(t *testing.T) {
	base, m := fixDLFixture(t, false)
	port, err := NewEq2BVTactic(m, base.context, base.globals)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	x := fixDLConst(t, m, true, "eq2bv-trace-x")
	num := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
	lower := fixDLApp(t, m, 3, x, num("0"))
	upper := fixDLApp(t, m, 2, x, num("3"))
	form := collectorEq(t, m, x, num("1"))
	goal := fixDLGoal(t, m, false, true, false, lower, upper, form)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	equationMust(t, port.BindTrace(tags))
	equationMust(t, port.Apply(goal, &result))
	if !tags.ShouldTrace("pb") {
		if output.Len() != 0 {
			t.Fatal("trace emitted with source gate disabled", output.String())
		}
		return
	}
	if got := output.String(); !strings.Contains(got, "[pb] operator() src/tactic/arith/eq2bv_tactic.cpp:210") || !strings.Contains(got, "(goal") {
		t.Fatalf("missing source goal trace:\n%s", got)
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.Enable("pb")
	equationMust(t, port.BindTrace(refused))
	retry := fixDLGoal(t, m, false, false, false, lower, upper, form)
	var retryResult tactic.TacticResult
	defer func() { equationMust(t, retryResult.Close()) }()
	equationMust(t, port.Apply(retry, &retryResult))
}
