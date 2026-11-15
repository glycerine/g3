package qe

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// qeTraceCapture routes the fixture's qe TRACE records into a buffer; it
// skips the test in builds without -tags g3trace.
func qeTraceCapture(t *testing.T, f *fmFixture) *bytes.Buffer {
	t.Helper()
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("qe")
	t.Cleanup(tags.Finalize)
	if !tags.ShouldTrace("qe") {
		t.Skip("TRACE records need -tags g3trace")
	}
	f.r.config.Trace = tags
	return &trace
}

func qeTraceInOrder(t *testing.T, trace string, want ...string) {
	t.Helper()
	at := 0
	for _, text := range want {
		k := strings.Index(trace[at:], text)
		if k < 0 {
			t.Fatalf("trace lacks %q after offset %d:\n%s", text, at, trace)
		}
		at += k + len(text)
	}
}

// AUDIT.md qe_05-F6: expr_quant_elim::operator() traces elim input and
// elim result.
func TestExprQuantElimTracesElimInputAndResult(t *testing.T) {
	f := qsatKernelFixture(t)
	trace := qeTraceCapture(t, f)
	m := f.r.manager
	a := f.constant("trace-a", f.integer)
	x := f.own(m.MkVar(0, f.integer))
	body := f.own(m.MkArithmeticGT(x, a))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	e := newExprQuantElim(m, f.r.config)
	defer func() { fmMust(t, e.Close()) }()
	result, err := e.run(f.own(m.MkTrue()), q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	qeTraceInOrder(t, trace.String(),
		"[qe] operator() src/qe/qe.cpp:2234", "elim input\n",
		"[qe] eliminate_block src/qe/qe.cpp:2145",
		"[qe] eliminate_block src/qe/qe.cpp:2161", "Input:\n", "Result:\n",
		"[qe] operator() src/qe/qe.cpp:2243", "elim result\ntrue\n")
}

// AUDIT.md qe_05-F6: sat_tactic::operator() traces input: and reduced:,
// and the QE simplify rewriter traces reduce_quantifier.
func TestQESATTacticTracesInputAndReduced(t *testing.T) {
	f := qsatKernelFixture(t)
	trace := qeTraceCapture(t, f)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.boolean))
	body := f.own(m.MkOr(v, f.own(m.MkNot(v))))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	port, err := NewQESATTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	fmMust(t, port.Apply(goal, f.result()))
	qeTraceInOrder(t, trace.String(),
		"[qe] operator() src/qe/qe_sat_tactic.cpp:246", "input: ",
		"[qe] reduce_quantifier src/qe/qe.cpp:2600", "simplifying",
		"[qe] reduce_quantifier src/qe/qe.cpp:2603", "variables extracted",
		"[qe] reduce_quantifier src/qe/qe.cpp:2616", "abstracted",
		"[qe] operator() src/qe/qe_sat_tactic.cpp:250", "reduced: ")
}
