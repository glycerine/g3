package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:85-88,111.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestNLA2BVSourceTraceSitesDisplayAndCount(t *testing.T) {
	r := nla2BVFixture(t, false)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("nla2bv")
	tags.Enable("nla2bv_verbose")
	if !tags.ShouldTrace("nla2bv") {
		t.Skip("source TRACE macro erased")
	}
	equationMust(t, r.BindTrace(tags))
	m := r.manager
	x := fixDLConst(t, m, true, "trace-count-x")
	product, err := m.MkArithmeticMul(x, x)
	equationMust(t, err)
	formula, err := m.MkEq(product, x)
	equationMust(t, err)
	fixDLPin(t, m, formula)
	g := fixDLGoal(t, m, false, false, false, formula)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, r.Apply(g, &result))
	got := output.String()
	for _, want := range []string{
		"src/tactic/arith/nla2bv_tactic.cpp:86", "Muls: 1\n",
		"src/tactic/arith/nla2bv_tactic.cpp:97", "substitute vars\n",
		"src/tactic/arith/nla2bv_tactic.cpp:100", "after reduce\n",
		"src/tactic/arith/nla2bv_tactic.cpp:110", "[nla2bv_verbose]",
		"src/tactic/arith/nla2bv_tactic.cpp:111", "(goal",
	} {
		if !strings.Contains(got, want) {
			t.Fatal("missing source trace fragment", want, got)
		}
	}
	if gotCount := strings.Count(got, "Muls: 1\n"); gotCount != 2 {
		t.Fatalf("source logs multiplication count before and after reduction, got %d records:\n%s", gotCount, got)
	}
}
