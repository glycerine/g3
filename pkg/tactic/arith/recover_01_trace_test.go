package arith

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestRecover01SourceTraceSitesAndRefusedStream(t *testing.T) {
	base, m := fixDLFixture(t, false)
	boolSort, err := m.MkBoolSort()
	equationMust(t, err)
	name, err := m.Symbols.Intern("recover-trace-p")
	equationMust(t, err)
	p, err := m.MkConst(name, boolSort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	notP, err := m.MkNot(p)
	equationMust(t, err)
	fixDLPin(t, m, notP)
	x := fixDLConst(t, m, true, "recover-trace-x")
	clause := func(lit ast.ASTHandle, value string) ast.ASTHandle {
		eq := collectorEq(t, m, x, fixDLNumber(t, m, true, value))
		form, e := m.MkOrArgs([]ast.ASTHandle{lit, eq})
		equationMust(t, e)
		fixDLPin(t, m, form)
		return form
	}
	goal := fixDLGoal(t, m, false, true, false, clause(p, "0"), clause(notP, "3"))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("recover_01")
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, recover01Apply(goal, base.rewriter, base.services, 10, &result, tags))
	if !tags.ShouldTrace("recover_01") {
		if output.Len() != 0 {
			t.Fatal("trace emitted with source gate disabled", output.String())
		}
		return
	}
	for _, want := range []string{
		"[recover_01] process src/tactic/arith/recover_01_tactic.cpp:288",
		"recover-trace-x --> ",
		"[recover_01] operator() src/tactic/arith/recover_01_tactic.cpp:376",
		"(goal",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("trace missing %q:\n%s", want, output.String())
		}
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	retry := fixDLGoal(t, m, false, false, false, clause(p, "0"), clause(notP, "3"))
	var retryResult tactic.TacticResult
	defer func() { equationMust(t, retryResult.Close()) }()
	equationMust(t, recover01Apply(retry, base.rewriter, base.services, 10, &retryResult, refused))
}
