package rewriter

// Source: src/util/trace.h source-disabled erasure and sticky stream behavior.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type factorRefusingWriter struct{}

func (factorRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFactorRewriterTraceBindingDisabledFailureAndOwnerGuards(t *testing.T) {
	f := newFactorFixture(t)
	bindBVTraceGlobals(t, f.m)
	x, y, zero := f.variable("bind-x"), f.variable("bind-y"), f.numeral(0)
	product := f.app(factorOpMul, x, y)
	_, decl := f.relation(-1, product, zero)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	out := astTestPin(t, f.m, zero)
	var silent bytes.Buffer
	astTestMust(t, rw.BindTrace(diagnostic.NewTraceTagsWithWriter(&silent)))
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{product, zero}), out)
	astTestMust(t, err)
	if status != RewriteDone || silent.Len() != 0 {
		t.Fatal("disabled trace", status, silent.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(factorRefusingWriter{})
	refused.Enable("factor_rewriter")
	astTestMust(t, rw.BindTrace(refused))
	status, err = rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{product, zero}), out)
	if err != nil || status != RewriteDone {
		t.Fatal("source sticky stream changed rewrite", status, err)
	}
	copyOwner := *rw
	if err = copyOwner.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner trace binding", err)
	}
	astTestMust(t, rw.Close())
	if err = rw.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed owner trace binding", err)
	}
}
