package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:59-82.

import (
	"bytes"
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
		if output.Len() != 0 {
			t.Fatal("erased trace wrote output", output.String())
		}
		return
	}
	// expr_ref streaming uses mk_ismt2_pp: the default width of 80 wraps
	// this ite and indents its branches by five columns.
	const want = "-------- [push_app_ite] is_target src/ast/rewriter/push_app_ite.cpp:51 ---------\n" +
		"found target for push app ite:\n" +
		"push-ite-f push-ite-x2 (ite push-ite-c1 push-ite-x1 push-ite-y1)\n" +
		"------------------------------------------------\n" +
		"-------- [push_app_ite] reduce_app src/ast/rewriter/push_app_ite.cpp:78 ---------\n" +
		"(ite push-ite-c1\n" +
		"     (push-ite-f push-ite-x2 push-ite-x1)\n" +
		"     (push-ite-f push-ite-x2 push-ite-y1))\n" +
		"------------------------------------------------\n"
	if got := output.String(); got != want {
		t.Fatalf("source push-app-ite trace:\ngot:\n%swant:\n%s", got, want)
	}
}
