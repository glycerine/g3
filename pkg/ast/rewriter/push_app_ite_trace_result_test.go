package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:59-82.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestPushAppITESourceResultTraceFollowsTarget(t *testing.T) {
	x := newPushITEFixture(t, false)
	bindBVTraceGlobals(t, x.m)
	term := pushITEMustIte(t, x.m, x.c1, x.x1, x.y1)
	root := pushITEMustApp(t, x.m, x.f, x.x2, term)
	rw, err := NewPushAppITE(x.m, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("push_app_ite")
	astTestMust(t, rw.BindTrace(tags))
	pushITERewrite(t, rw, x.m, root)
	if !tags.ShouldTrace("push_app_ite") {
		return
	}
	trace := output.String()
	target := strings.Index(trace, "push_app_ite.cpp:51")
	result := strings.Index(trace, "push_app_ite.cpp:78")
	if target < 0 || result <= target {
		t.Fatalf("source trace ordering: %q", trace)
	}
	if want := "(ite push-ite-c1 (push-ite-f push-ite-x2 push-ite-x1) (push-ite-f push-ite-x2 push-ite-y1))\n"; !strings.Contains(trace[result:], want) {
		t.Fatalf("missing result %q in %q", want, trace)
	}
}
