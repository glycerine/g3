package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:54-76 reuse restart.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestMaximizeBVSharingSourceTraceReuseIndicesAndRestart(t *testing.T) {
	f := newMaximizeBVFixture(t, false)
	bindBVTraceGlobals(t, f.m)
	c, err := NewMaximizeBVSharingConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	astTestMust(t, c.PushScope())
	decl, err := f.m.GetDecl(maximizeBVApp(t, f, 4, f.vars[0], f.vars[1]))
	astTestMust(t, err)
	out, proof := maximizeBVOutput(t, f.m)
	_, err = c.ReduceApp(decl, ast.SliceArguments(f.vars[:4]), out, proof)
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("ac_sharing_detail")
	astTestMust(t, c.BindTrace(tags))
	status, err := c.ReduceApp(decl, ast.SliceArguments([]ast.ASTHandle{f.vars[0], f.vars[1], f.vars[4]}), out, proof)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("reuse trace status", status)
	}
	if !tags.ShouldTrace("ac_sharing_detail") {
		return
	}
	got := output.String()
	if !strings.Contains(got, "reusing args: 0 1\n") || !strings.Contains(got, "tree-loop: num_args: 2\n") {
		t.Fatal("reuse indices/restart trace", got)
	}
	if strings.Count(got, "maximize_ac_sharing.cpp:60") != 1 {
		t.Fatal("reuse trace count", got)
	}
}
