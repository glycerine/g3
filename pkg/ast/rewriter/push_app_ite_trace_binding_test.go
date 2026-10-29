package rewriter

// Source: src/util/trace.h erased tags and sticky writer behavior.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type pushITERefusingWriter struct{}

func (pushITERefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPushAppITETraceBindingDisabledFailureAndOwnerGuards(t *testing.T) {
	x := newPushITEFixture(t, false)
	bindBVTraceGlobals(t, x.m)
	term := pushITEMustIte(t, x.m, x.c1, x.x1, x.y1)
	root := pushITEMustApp(t, x.m, x.f, term, x.x2)
	rw, err := NewPushAppITE(x.m, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	var silent bytes.Buffer
	astTestMust(t, rw.BindTrace(diagnostic.NewTraceTagsWithWriter(&silent)))
	got, _ := pushITERewrite(t, rw, x.m, root)
	if got == root || silent.Len() != 0 {
		t.Fatal("disabled trace", got, silent.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(pushITERefusingWriter{})
	refused.Enable("push_app_ite")
	astTestMust(t, rw.BindTrace(refused))
	got, _ = pushITERewrite(t, rw, x.m, root)
	if got == root {
		t.Fatal("sticky stream failure changed rewrite")
	}
	copyOwner := *rw
	if err = copyOwner.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copy trace binding", err)
	}
	astTestMust(t, rw.Close())
	if err = rw.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed trace binding", err)
	}
}
