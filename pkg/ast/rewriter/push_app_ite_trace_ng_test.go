package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:85-92.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestNGPushAppITETracesBaseTargetBeforeGroundRefusal(t *testing.T) {
	x := newPushITEFixture(t, false)
	bindBVTraceGlobals(t, x.m)
	term := pushITEMustIte(t, x.m, x.c1, x.x1, x.y1)
	root := pushITEMustApp(t, x.m, x.f, term, x.x2)
	rw, err := NewNGPushAppITE(x.m, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("push_app_ite")
	astTestMust(t, rw.BindTrace(tags))
	got, _ := pushITERewrite(t, rw, x.m, root)
	if got != root {
		t.Fatal("ground application was rewritten")
	}
	if !tags.ShouldTrace("push_app_ite") {
		return
	}
	trace := output.String()
	if strings.Count(trace, "push_app_ite.cpp:51") != 1 || strings.Contains(trace, "push_app_ite.cpp:78") {
		t.Fatalf("base-target/derived-refusal ordering: %q", trace)
	}
}
