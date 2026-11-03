package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:36-56.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestPushAppITESourceTargetTrace(t *testing.T) {
	x := newPushITEFixture(t, false)
	bindBVTraceGlobals(t, x.m)
	term := pushITEMustIte(t, x.m, x.c1, x.x1, x.y1)
	root := pushITEMustApp(t, x.m, x.f, term, x.x2)
	rw, err := NewPushAppITE(x.m, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("push_app_ite")
	astTestMust(t, rw.BindTrace(tags))
	got, _ := pushITERewrite(t, rw, x.m, root)
	if got == root {
		t.Fatal("target was not rewritten")
	}
	if !tags.ShouldTrace("push_app_ite") {
		return
	}
	trace := output.String()
	const target = "-------- [push_app_ite] is_target src/ast/rewriter/push_app_ite.cpp:51 ---------\n" +
		"found target for push app ite:\n" +
		"push-ite-f (ite push-ite-c1 push-ite-x1 push-ite-y1) push-ite-x2\n" +
		"------------------------------------------------\n"
	if !strings.HasPrefix(trace, target) || strings.Count(trace, "push_app_ite.cpp:51") != 1 {
		t.Fatalf("source target trace: got %q, want prefix %q", trace, target)
	}
}
