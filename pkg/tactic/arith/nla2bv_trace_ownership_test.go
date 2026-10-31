package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp TRACE operands;
// src/ast/ast_smt2_pp.cpp temporary format_ref ownership.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestNLA2BVTraceOperandFailuresReachCaller(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		name := "writable"
		if refuse {
			name = "refusing"
		}
		t.Run(name, func(t *testing.T) {
			r := nla2BVFixture(t, false)
			var buffer bytes.Buffer
			var writer io.Writer = &buffer
			if refuse {
				writer = &nla2BVRefusingTrace{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("nla2bv")
			tags.Enable("nla2bv_verbose")
			if !tags.ShouldTrace("nla2bv") {
				t.Skip("source TRACE macro erased")
			}
			equationMust(t, r.BindTrace(tags))
			x := fixDLConst(t, r.manager, true, "trace-operand")
			g := fixDLGoal(t, r.manager, false, false, false, mustNLA2BVEq(t, r.manager, x, fixDLNumber(t, r.manager, true, "2")))
			c := &nla2BVInvocation{t: r}
			for _, site := range []struct {
				name string
				emit func() error
			}{
				{"expression", func() error { return c.traceExpressionLine("add_int_var", 230, "no bounds for ", x) }},
				{"power", func() error { return c.tracePower2Substitution(x, x) }},
				{"goal", func() error { return c.traceGoalDisplay(g, "nla2bv", 97, "substitute vars\n") }},
				{"verbose-goal", func() error { return c.traceGoalDisplay(g, "nla2bv_verbose", 110, "") }},
				{"goal-count", func() error { return c.traceGoalCount(g, 86, true) }},
			} {
				t.Run(site.name, func(t *testing.T) {
					r.manager.Budget().FailAfter(0)
					err := site.emit()
					r.manager.Budget().FailAfter(-1)
					if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal("trace operand failure suppressed", err)
					}
				})
			}
		})
	}
}

func TestNLA2BVTraceInitialFailurePrecedesGoalMutation(t *testing.T) {
	r := nla2BVFixture(t, false)
	tags := diagnostic.NewTraceTagsWithWriter(io.Discard)
	tags.Enable("nla2bv")
	if !tags.ShouldTrace("nla2bv") {
		t.Skip("source TRACE macro erased")
	}
	equationMust(t, r.BindTrace(tags))
	x := fixDLConst(t, r.manager, true, "trace-initial")
	form := mustNLA2BVEq(t, r.manager, x, fixDLNumber(t, r.manager, true, "2"))
	g := fixDLGoal(t, r.manager, false, false, false, form)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	var out tactic.TacticResult
	t.Cleanup(func() { equationMust(t, out.Close()) })
	previous, err := uparams.BindProcessGlobalParamsService(nil)
	equationMust(t, err)
	err = c.run(g, &out)
	_, restore := uparams.BindProcessGlobalParamsService(previous)
	equationMust(t, restore)
	if !errors.Is(err, uparams.ErrGlobalParamsOwner) {
		t.Fatal("initial printer failure suppressed", err)
	}
	if c.filter != nil || out.ModelConverter != nil {
		t.Fatal("execution advanced past failing initial trace")
	}
	got, err := g.Form(0)
	equationMust(t, err)
	depth, err := g.Depth()
	equationMust(t, err)
	if got != form || depth != 0 {
		t.Fatal("initial trace failure changed goal")
	}
}

func TestNLA2BVTraceExpressionReleasesFormatTree(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	x := fixDLConst(t, m, true, "trace-format")
	form := mustNLA2BVEq(t, m, x, fixDLNumber(t, m, true, "2"))
	f, err := m.GetFormatManager()
	equationMust(t, err)
	// Warm the format plugin and its constant roots, then release the whole tree.
	doc, err := m.SMT2PPExprTree(form, nil)
	equationMust(t, err)
	ref, err := f.NewRef(doc)
	equationMust(t, err)
	equationMust(t, f.DelRef(&ref))
	before := f.Stats()
	c := &nla2BVInvocation{t: r}
	for i := 0; i < 3; i++ {
		equationMust(t, c.writeTraceExpr(io.Discard, form))
		after := f.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("trace printer left format tree alive", before, after)
		}
	}
}
