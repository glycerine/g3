package rewriter

// Source: src/util/trace.h erased tags and sticky writer behavior.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestMaximizeBVSharingTraceBindingDisabledFailureAndGuards(t *testing.T) {
	f := newMaximizeBVFixture(t, false)
	bindBVTraceGlobals(t, f.m)
	c, err := NewMaximizeBVSharingConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	astTestMust(t, c.PushScope())
	decl, err := f.m.GetDecl(maximizeBVApp(t, f, 4, f.vars[0], f.vars[1]))
	astTestMust(t, err)
	out, proof := maximizeBVOutput(t, f.m)
	var silent bytes.Buffer
	astTestMust(t, c.BindTrace(diagnostic.NewTraceTagsWithWriter(&silent)))
	status, err := c.ReduceApp(decl, ast.SliceArguments(f.vars[:3]), out, proof)
	astTestMust(t, err)
	if status != RewriteDone || silent.Len() != 0 {
		t.Fatal("disabled trace", status, silent.String())
	}
	refused := diagnostic.NewTraceTagsWithWriter(factorRefusingWriter{})
	refused.Enable("ac_sharing_detail")
	astTestMust(t, c.BindTrace(refused))
	status, err = c.ReduceApp(decl, ast.SliceArguments(f.vars[3:6]), out, proof)
	if err != nil || status != RewriteDone {
		t.Fatal("sticky stream changed rewrite", status, err)
	}
	copyOwner := *c
	if err = copyOwner.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copy trace binding", err)
	}
	astTestMust(t, c.Close())
	if err = c.BindTrace(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed trace binding", err)
	}
}
