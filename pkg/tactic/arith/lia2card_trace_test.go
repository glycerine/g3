package arith

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestLIA2CardSourceTraceSitesAndRefusedStream(t *testing.T) {
	base, m := fixDLFixture(t, false)
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	tags.Enable("pbsum")
	equationMust(t, port.BindTrace(tags))
	x := fixDLConst(t, m, true, "card-trace-x")
	z := fixDLConst(t, m, true, "card-trace-unbounded")
	zero := fixDLNumber(t, m, true, "0")
	one := fixDLNumber(t, m, true, "1")
	two := fixDLNumber(t, m, true, "2")
	lower := fixDLApp(t, m, 3, x, zero)
	upper := fixDLApp(t, m, 2, x, one)
	comparison := collectorEq(t, m, fixDLApp(t, m, 6, x, x), one)
	unsupported := fixDLApp(t, m, 3, z, two)
	forms := []ast.ASTHandle{lower, upper, comparison, unsupported}
	goal := fixDLGoal(t, m, false, false, false, forms...)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, port.Apply(goal, &result))
	if tags.ShouldTrace("pb") {
		got := output.String()
		for _, want := range []string{
			"[pb] operator() src/tactic/arith/lia2card_tactic.cpp:185",
			"add bound card-trace-x",
			"[pb] get_sum src/tactic/arith/lia2card_tactic.cpp:342",
			"Can't handle card-trace-unbounded",
			"[pbsum] mk_app_core src/tactic/arith/lia2card_tactic.cpp:92",
			" ==>\n",
			"[pb] operator() src/tactic/arith/lia2card_tactic.cpp:210",
			"(goal",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("trace missing %q:\n%s", want, got)
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("trace emitted with source gate disabled", output.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, port.BindTrace(refused))
	retry := fixDLGoal(t, m, false, false, false, forms...)
	var retryResult tactic.TacticResult
	defer func() { equationMust(t, retryResult.Close()) }()
	equationMust(t, port.Apply(retry, &retryResult))
}

func TestLIA2CardExactTraceRecords(t *testing.T) {
	base, m := fixDLFixture(t, false)
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	x := fixDLConst(t, m, true, "card-exact-x")
	formula := collectorEq(t, m, x, fixDLNumber(t, m, true, "1"))
	goal := fixDLGoal(t, m, false, false, false, formula)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("pb")
	equationMust(t, port.BindTrace(tags))
	equationMust(t, port.traceBound(x))
	equationMust(t, lia2cardTraceUnsupported(m, tags, x))
	equationMust(t, port.traceGoal(goal))
	if !tags.ShouldTrace("pb") {
		if out.Len() != 0 {
			t.Fatal("disabled trace wrote output")
		}
		return
	}
	var goalText bytes.Buffer
	equationMust(t, goal.Display(&goalText))
	record := func(function string, line int, body string) string {
		return fmt.Sprintf("-------- [pb] %s src/tactic/arith/lia2card_tactic.cpp:%d ---------\n%s------------------------------------------------\n", function, line, body)
	}
	want := record("operator()", 185, "add bound card-exact-x\n") +
		record("get_sum", 342, "Can't handle card-exact-x\n") +
		record("operator()", 210, goalText.String())
	if out.String() != want {
		t.Fatalf("source trace records:\n%s\nwant:\n%s", out.String(), want)
	}
}
