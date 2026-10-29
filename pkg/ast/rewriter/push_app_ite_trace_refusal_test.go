package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp:36-55 conditional trace guards.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestPushAppITERefusedTargetsDoNotTrace(t *testing.T) {
	x := newPushITEFixture(t, false)
	bindBVTraceGlobals(t, x.m)
	first := pushITEMustIte(t, x.m, x.c1, x.x1, x.y1)
	second := pushITEMustIte(t, x.m, x.c2, x.x2, x.y2)
	boolean := pushITEMustIte(t, x.m, x.c1, x.c1, x.c2)
	rw, err := NewPushAppITE(x.m, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("push_app_ite")
	astTestMust(t, rw.BindTrace(tags))
	// A conservative multiple-ITE refusal returns before CTRACE; a Boolean ITE
	// never sets found_ite. Exercise both through the retained configuration.
	two := pushITEMustApp(t, x.m, x.f, first, second)
	got, _ := pushITERewrite(t, rw, x.m, two)
	if got != two {
		t.Fatal("multiple ITEs were rewritten")
	}
	boolDecl, err := x.m.MkFuncDecl(astTestSymbol(t, x.m, "push-ite-refuse-bool"), []ast.ASTHandle{x.boolSort}, x.boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	boolRoot := pushITEMustApp(t, x.m, boolDecl, boolean)
	got, _ = pushITERewrite(t, rw, x.m, boolRoot)
	if got != boolRoot {
		t.Fatal("Boolean ITE was rewritten")
	}
	if tags.ShouldTrace("push_app_ite") && output.Len() != 0 {
		t.Fatalf("refused target traced: %q", output.String())
	}
}
