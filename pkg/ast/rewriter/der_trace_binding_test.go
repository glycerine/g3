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

type derRefusingWriter struct{}

func (derRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDERTraceBindingDisabledFailureAndOwnerGuards(t *testing.T) {
	f := newFactorFixture(t)
	bindBVTraceGlobals(t, f.m)
	d, err := NewDER(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, d.Close()) })
	v, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	eq, err := f.m.MkEq(v, f.numeral(1))
	astTestMust(t, err)
	q := derQuantifier(t, f.m, false, []ast.ASTHandle{f.sort}, eq)
	var silent bytes.Buffer
	astTestMust(t, d.BindTrace(diagnostic.NewTraceTagsWithWriter(&silent)))
	derReduce(t, d, f.m, q)
	if silent.Len() != 0 {
		t.Fatal("disabled trace", silent.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(derRefusingWriter{})
	refused.Enable("der")
	astTestMust(t, d.BindTrace(refused))
	derReduce(t, d, f.m, q)
	copyOwner := *d
	if err = copyOwner.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copy binding", err)
	}
	astTestMust(t, d.Close())
	if err = d.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed binding", err)
	}
}
