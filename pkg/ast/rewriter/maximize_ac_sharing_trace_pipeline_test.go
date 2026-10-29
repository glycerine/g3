package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp:49-95.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestMaximizeBVSharingSourceTraceBalancedPipeline(t *testing.T) {
	f := newMaximizeBVFixture(t, false)
	bindBVTraceGlobals(t, f.m)
	c, err := NewMaximizeBVSharingConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	astTestMust(t, c.PushScope())
	decl, err := f.m.GetDecl(maximizeBVApp(t, f, 4, f.vars[0], f.vars[1]))
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("ac_sharing_detail")
	astTestMust(t, c.BindTrace(tags))
	out, proof := maximizeBVOutput(t, f.m)
	status, err := c.ReduceApp(decl, ast.SliceArguments(f.vars[:4]), out, proof)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("balanced trace status", status)
	}
	if !tags.ShouldTrace("ac_sharing_detail") {
		return
	}
	got := output.String()
	for _, want := range []string{
		"maximize_ac_sharing.cpp:49", "before-reuse: num_args: 4",
		"maximize_ac_sharing.cpp:54", "args: (:var 0)\n(:var 1)\n(:var 2)\n(:var 3)\n",
		"maximize_ac_sharing.cpp:76", "tree-loop: num_args: 4", "tree-loop: num_args: 2",
		"maximize_ac_sharing.cpp:95", "result: (bvadd",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing trace %q in %q", want, got)
		}
	}
	if strings.Count(got, "maximize_ac_sharing.cpp:76") != 2 {
		t.Fatal("tree round trace count", got)
	}
}
