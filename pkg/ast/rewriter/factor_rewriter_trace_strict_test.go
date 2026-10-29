package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp:104-120.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFactorRewriterSourceStrictSuccessTracePrintsResultOnly(t *testing.T) {
	f := newFactorFixture(t)
	bindBVTraceGlobals(t, f.m)
	x, y, zero := f.variable("strict-x"), f.variable("strict-y"), f.numeral(0)
	product := f.app(factorOpMul, x, y)
	_, decl := f.relation(factorOpLT, product, zero)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("factor_rewriter")
	astTestMust(t, rw.BindTrace(tags))
	out := astTestPin(t, f.m, zero)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{product, zero}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("strict factor status", status)
	}
	if !tags.ShouldTrace("factor_rewriter") {
		return
	}
	got := output.String()
	header := "[factor_rewriter] mk_lt src/ast/rewriter/factor_rewriter.cpp:119"
	if !strings.Contains(got, header) || strings.Contains(got, "factor_rewriter.cpp:104") {
		t.Fatal("strict success/refusal trace selection", got)
	}
	start := strings.Index(got, header)
	if start < 0 || !strings.Contains(got[start:], "strict-x") || !strings.Contains(got[start:], "strict-y") {
		t.Fatal("strict result trace body", got)
	}
}
