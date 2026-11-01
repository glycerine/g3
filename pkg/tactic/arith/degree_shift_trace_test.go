package arith

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestDegreeShiftSourceTraceSitesAndRefusedStream(t *testing.T) {
	base, m := fixDLFixture(t, false)
	shift, err := NewDegreeShiftTactic(m, base.services)
	equationMust(t, err)
	defer func() { equationMust(t, shift.Close()) }()
	x := fixDLConst(t, m, false, "degree-trace-x")
	power := fixDLApp(t, m, 18, x, fixDLNumber(t, m, false, "2"))
	form, err := m.MkEq(power, fixDLNumber(t, m, false, "4"))
	equationMust(t, err)
	fixDLPin(t, m, form)
	goal := fixDLGoal(t, m, false, true, false, form)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("degree_shift")
	equationMust(t, shift.BindTrace(tags))
	equationMust(t, shift.Apply(goal, &result))
	if !tags.ShouldTrace("degree_shift") {
		if output.Len() != 0 {
			t.Fatal("trace emitted with source gate disabled", output.String())
		}
		return
	}
	for _, want := range []string{
		"[degree_shift] collect src/tactic/arith/degree_shift_tactic.cpp:186",
		"candidates:\nPOWER: 2\ndegree-trace-x",
		"[degree_shift] operator() src/tactic/arith/degree_shift_tactic.cpp:289",
		"(goal",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("trace missing %q:\n%s", want, output.String())
		}
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, shift.BindTrace(refused))
	retry := fixDLGoal(t, m, false, false, false, form)
	var retryResult tactic.TacticResult
	defer func() { equationMust(t, retryResult.Close()) }()
	equationMust(t, shift.Apply(retry, &retryResult))
}
