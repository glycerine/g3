package arith

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestPurifyArithExactEmptyConstraintTrace(t *testing.T) {
	port, m := qfnraPurifier(t, false, true)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("purify_arith")
	equationMust(t, port.BindTrace(tags))
	goal := fixDLGoal(t, m, false, false, false, generalPurifierFailureFormula(t, m, "integer"))
	invocation, err := newQFNRAPurifyInvocation(port, goal)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, invocation.close()) })
	equationMust(t, port.traceGoal(goal, 835))
	equationMust(t, invocation.traceConstraints())
	if !tags.ShouldTrace("purify_arith") {
		if output.Len() != 0 {
			t.Fatal("disabled trace wrote output")
		}
		return
	}
	var text bytes.Buffer
	equationMust(t, goal.Display(&text))
	record := func(line int, body string) string {
		return fmt.Sprintf("-------- [purify_arith] operator() src/tactic/arith/purify_arith_tactic.cpp:%d ---------\n%s------------------------------------------------\n", line, body)
	}
	want := record(835, text.String()) + record(762, "\n")
	if output.String() != want {
		t.Fatalf("trace records:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestPurifyArithParameterDescriptionsIncludeTheoryRewriter(t *testing.T) {
	port, m := qfnraPurifier(t, false, true)
	descriptions, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, descriptions.Close()) })
	equationMust(t, port.CollectParamDescrs(descriptions))
	for _, name := range []string{"complete", "elim_root_objects", "elim_inverses", "elim_and", "max_steps", "som", "sort_store"} {
		found, err := descriptions.ContainsName(name)
		equationMust(t, err)
		if !found {
			t.Fatalf("missing parameter %s", name)
		}
	}
	symbol, err := m.Symbols.Intern("elim_inverses")
	equationMust(t, err)
	description, found, err := descriptions.Description(symbol)
	equationMust(t, err)
	if !found || description != "(default: true) eliminate inverse trigonometric functions (asin, acos, atan)." {
		t.Fatal("inverse parameter description", description)
	}
}

func TestPurifyArithSourceTraceSitesAndRefusedStream(t *testing.T) {
	port, m := qfnraPurifier(t, false, true)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("purify_arith")
	equationMust(t, port.BindTrace(tags))
	quantified := generalPurifierFailureFormula(t, m, "quantifier")
	integer := generalPurifierFailureFormula(t, m, "integer")
	goal := fixDLGoal(t, m, false, true, false, quantified, integer)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, port.Apply(goal, &result))
	if tags.ShouldTrace("purify_arith") {
		for _, want := range []string{
			"operator() src/tactic/arith/purify_arith_tactic.cpp:835",
			"process_quantifier src/tactic/arith/purify_arith_tactic.cpp:700",
			"num_vars: 1\nbody: ", "\nnew_body: ",
			"operator() src/tactic/arith/purify_arith_tactic.cpp:762",
			"operator() src/tactic/arith/purify_arith_tactic.cpp:847",
		} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("trace missing %q:\n%s", want, output.String())
			}
		}
	} else if output.Len() != 0 {
		t.Fatal("disabled trace wrote output", output.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, port.BindTrace(refused))
	retry := fixDLGoal(t, m, false, true, false, quantified, integer)
	var retried tactic.TacticResult
	t.Cleanup(func() { equationMust(t, retried.Close()) })
	equationMust(t, port.Apply(retry, &retried))
}

func TestPurifyArithTraceBindingSurvivesTranslation(t *testing.T) {
	port, m := qfnraPurifier(t, false, true)
	tags := diagnostic.NewTraceTags()
	equationMust(t, port.BindTrace(tags))
	same, err := port.Translate(m)
	equationMust(t, err)
	samePort := same.(*QFNRAPurifyTactic)
	t.Cleanup(func() { equationMust(t, samePort.Close()) })
	if samePort.trace != tags {
		t.Fatal("same-context translation lost trace binding")
	}
	target, targetManager := fixDLFixture(t, false)
	cross, matched, err := TranslateContextLeaf(port, targetManager, target.context, target.globals, target.services)
	equationMust(t, err)
	if !matched {
		t.Fatal("missing context translation")
	}
	crossPort := cross.(*QFNRAPurifyTactic)
	t.Cleanup(func() { equationMust(t, crossPort.Close()) })
	if crossPort.trace != tags || crossPort.context != target.context {
		t.Fatal("cross-context translation lost trace binding or target context")
	}
}
