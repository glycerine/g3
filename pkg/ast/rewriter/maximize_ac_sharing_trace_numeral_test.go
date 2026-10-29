package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:39-49,91-95.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestMaximizeBVSharingSourceTraceNumeralExcludedThenRestored(t *testing.T) {
	f := newMaximizeBVFixture(t, false)
	bindBVTraceGlobals(t, f.m)
	c, err := NewMaximizeBVSharingConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	astTestMust(t, c.PushScope())
	decl, err := f.m.GetDecl(maximizeBVApp(t, f, 4, f.vars[0], f.vars[1]))
	astTestMust(t, err)
	numeral := bvAddNum(t, f.m, 3, 8)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("ac_sharing_detail")
	astTestMust(t, c.BindTrace(tags))
	out, proof := maximizeBVOutput(t, f.m)
	status, err := c.ReduceApp(decl, ast.SliceArguments([]ast.ASTHandle{numeral, f.vars[0], f.vars[1]}), out, proof)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("numeral trace status", status)
	}
	if !tags.ShouldTrace("ac_sharing_detail") {
		return
	}
	got := output.String()
	entryEnd := strings.Index(got, "maximize_ac_sharing.cpp:76")
	if entryEnd < 0 {
		t.Fatal("missing tree trace", got)
	}
	if !strings.Contains(got[:entryEnd], "before-reuse: num_args: 2") || strings.Contains(got[:entryEnd], "#x03") {
		t.Fatal("numeral was not excluded from entry arguments", got)
	}
	if !strings.Contains(got[entryEnd:], "result: (bvadd #x03") {
		t.Fatal("numeral not restored in result", got)
	}
}
