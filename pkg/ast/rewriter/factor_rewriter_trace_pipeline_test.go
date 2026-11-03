package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:166-227,332-338,90-93.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFactorRewriterSourceTracePipeline(t *testing.T) {
	f := newFactorFixture(t)
	bindBVTraceGlobals(t, f.m)
	x, y, z, zero := f.variable("trace-factor-x"), f.variable("trace-factor-y"), f.variable("trace-factor-z"), f.numeral(0)
	left, right := f.app(factorOpMul, x, y), f.app(factorOpMul, x, z)
	_, decl := f.relation(factorOpLE, left, right)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("factor_rewriter")
	astTestMust(t, rw.BindTrace(tags))
	out := astTestPin(t, f.m, zero)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{left, right}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("factored trace fixture", status)
	}
	if !tags.ShouldTrace("factor_rewriter") {
		if output.Len() != 0 {
			t.Fatal("erased trace wrote output", output.String())
		}
		return
	}
	// mk_adds visits each product once; only mk_muls expands its factors.
	// Check the full ordered pipeline, including source spaces and the
	// default SMT2 pretty-printer's let bindings and line breaks.
	const want = "-------- [factor_rewriter] mk_adds src/ast/rewriter/factor_rewriter.cpp:166 ---------\n" +
		"0 (* trace-factor-x trace-factor-y)\n" +
		"------------------------------------------------\n" +
		"-------- [factor_rewriter] mk_adds src/ast/rewriter/factor_rewriter.cpp:166 ---------\n" +
		"1 (* trace-factor-x trace-factor-z)\n" +
		"------------------------------------------------\n" +
		"-------- [factor_rewriter] mk_adds src/ast/rewriter/factor_rewriter.cpp:198 ---------\n" +
		"+(* trace-factor-x trace-factor-y) -(* trace-factor-x trace-factor-z) \n" +
		"------------------------------------------------\n" +
		"-------- [factor_rewriter] mk_muls src/ast/rewriter/factor_rewriter.cpp:219 ---------\n" +
		"trace-factor-x trace-factor-y \ntrace-factor-x trace-factor-z \n\n" +
		"------------------------------------------------\n" +
		"-------- [factor_rewriter] extract_factors src/ast/rewriter/factor_rewriter.cpp:332 ---------\n" +
		"trace-factor-x (+ trace-factor-y (- trace-factor-z)) \n" +
		"------------------------------------------------\n" +
		"-------- [factor_rewriter] mk_le src/ast/rewriter/factor_rewriter.cpp:90 ---------\n" +
		"(* trace-factor-x trace-factor-y) <= (* trace-factor-x trace-factor-z)\n" +
		"(let ((a!1 (and (< (+ trace-factor-y (- trace-factor-z)) 0)\n" +
		"                (< 0 trace-factor-x)))\n" +
		"      (a!2 (and (< 0 (+ trace-factor-y (- trace-factor-z)))\n" +
		"                (< trace-factor-x 0))))\n" +
		"  (or (= 0 trace-factor-x) (= 0 (+ trace-factor-y (- trace-factor-z))) a!1 a!2))\n" +
		"------------------------------------------------\n"
	if got := output.String(); got != want {
		t.Fatalf("source factor trace:\ngot:\n%swant:\n%s", got, want)
	}
}
