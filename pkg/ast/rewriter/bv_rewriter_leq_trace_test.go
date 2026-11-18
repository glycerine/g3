package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:509-526; src/util/trace.h:40-46.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestBVRewriterLEExtraSourceTrace(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	bindBVTraceGlobals(t, m)
	local, err := c.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Del(&local)) }()
	key := "bv_le_extra"
	astTestMust(t, c.SetBoolName(&local, &key, true))
	p, err := NewBVRewriter(m, c, g, &local)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("le_extra")
	astTestMust(t, p.BindTrace(tags))
	xs := bvMergeVars(t, m, 3)
	cat := func(a, b ast.ASTHandle) ast.ASTHandle { return bvPropagationConcat(t, m, a, b) }
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	status, err := p.RewriteLE(false, cat(xs[0], xs[1]), cat(xs[0], xs[2]), out)
	astTestMust(t, err)
	if status != Rewrite2 {
		t.Fatal("concat source branch", status)
	}
	status, err = p.RewriteLE(true, cat(xs[0], xs[1]), cat(xs[0], xs[2]), out)
	astTestMust(t, err)
	if status != Rewrite2 {
		t.Fatal("signed concat source branch", status)
	}
	add := func(a, b ast.ASTHandle) ast.ASTHandle { return bvPropagationOp(t, m, p, 4, a, b) }
	status, err = p.RewriteLE(false, add(bvAddNum(t, m, 2, 8), xs[0]), add(bvAddNum(t, m, 5, 8), xs[0]), out)
	astTestMust(t, err)
	if status != Rewrite2 {
		t.Fatal("overflow source branch", status)
	}
	if !tags.ShouldTrace("le_extra") {
		if output.Len() != 0 {
			t.Fatal("erased source trace", output.String())
		}
		return
	}
	record := func(line, body string) string {
		return "-------- [le_extra] mk_leq_core src/ast/rewriter/bv_rewriter.cpp:" + line + " ---------\n" + body + "------------------------------------------------\n"
	}
	want := record("514", "bv_ule\n(concat (:var 0) (:var 1))\n(concat (:var 0) (:var 2))\n--->\n(bvule (:var 1) (:var 2))\n") +
		record("514", "bv_sle\n(concat (:var 0) (:var 1))\n(concat (:var 0) (:var 2))\n--->\n(bvule (:var 1) (:var 2))\n") +
		record("523", "bv_ule\n(bvadd #x02 (:var 0))\n(bvadd #x05 (:var 0))\n--->\n(bvule #x03 (bvadd #x05 (:var 0)))\n")
	if got := output.String(); got != want {
		t.Fatalf("source LE records: got %q, want %q", got, want)
	}
	tags.Disable("le_extra")
	status, err = p.RewriteLE(false, cat(xs[0], xs[1]), cat(xs[0], xs[2]), out)
	astTestMust(t, err)
	if status != Rewrite2 || output.String() != want {
		t.Fatalf("disabled LE tag: status=%v records=%q", status, output.String())
	}
}
