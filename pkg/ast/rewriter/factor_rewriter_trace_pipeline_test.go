package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:166-227,332-338,90-93.

import (
	"bytes"
	"strings"
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
	got := output.String()
	for _, site := range []string{
		"[factor_rewriter] mk_adds src/ast/rewriter/factor_rewriter.cpp:166",
		"[factor_rewriter] mk_adds src/ast/rewriter/factor_rewriter.cpp:198",
		"[factor_rewriter] mk_muls src/ast/rewriter/factor_rewriter.cpp:219",
		"[factor_rewriter] extract_factors src/ast/rewriter/factor_rewriter.cpp:332",
		"[factor_rewriter] mk_le src/ast/rewriter/factor_rewriter.cpp:90",
		"trace-factor-x", "trace-factor-y", "trace-factor-z", " <= ",
	} {
		if !strings.Contains(got, site) {
			t.Fatalf("missing source trace %q in %q", site, got)
		}
	}
	if strings.Count(got, "factor_rewriter.cpp:166") < 4 {
		t.Fatal("missing ordered add visits", got)
	}
}
