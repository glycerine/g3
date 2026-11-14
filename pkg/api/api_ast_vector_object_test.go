package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIASTVectorSourceOperationsAndOwnership(t *testing.T) {
	m, c := apiResultsFixture(t, false)
	v, err := newAPIASTVectorObject(c)
	astTestMust(t, err)
	astTestMust(t, c.saveObject(&v.object))
	p := apiResultsAtom(t, m, "api-vector-p")
	q := apiResultsAtom(t, m, "api-vector-q")
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{p, q, boolSort} {
		astTestMust(t, m.IncRef(h))
	}
	base := func(h ast.ASTHandle) uint32 {
		t.Helper()
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		return n
	}
	pBase, qBase, sortBase := base(p), base(q), base(boolSort)

	astTestMust(t, v.push(p))
	astTestMust(t, v.resize(3))
	if n, e := v.size(); e != nil || n != 3 {
		t.Fatal("resize", n, e)
	}
	if h, e := v.get(1); e != nil || !h.IsNull() {
		t.Fatal("expanded ast_ref slot", h, e)
	}
	if _, e := v.get(3); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("out-of-bounds get", e)
	}
	astTestMust(t, v.set(0, q))
	astTestMust(t, v.set(1, boolSort)) // ast_ref_vector stores every AST kind.
	if base(p) != pBase || base(q) != qBase+1 || base(boolSort) != sortBase+1 {
		t.Fatal("set did not retain incoming before releasing old")
	}
	astTestMust(t, v.resize(1))
	if base(boolSort) != sortBase {
		t.Fatal("shrink retained removed sort")
	}
	text, err := v.string()
	astTestMust(t, err)
	if !strings.HasPrefix(text, "(ast-vector\n  ") || !strings.HasSuffix(text, ")") {
		t.Fatal("source vector display", text)
	}
	astTestMust(t, c.Close())
	if base(q) != qBase {
		t.Fatal("vector destructor retained element")
	}
	for _, h := range []ast.ASTHandle{p, q, boolSort} {
		astTestMust(t, m.DecRef(h))
	}
}

func TestAPIASTVectorTranslateOwnsIndependentTargetRoots(t *testing.T) {
	sourceManager := astTestManager(t)
	targetManager := astTestManager(t)
	source, err := newAPIContextResults(sourceManager, false)
	astTestMust(t, err)
	target, err := newAPIContextResults(targetManager, false)
	astTestMust(t, err)
	v, err := newAPIASTVectorObject(source)
	astTestMust(t, err)
	astTestMust(t, source.saveObject(&v.object))
	p := apiResultsAtom(t, sourceManager, "api-vector-translate")
	astTestMust(t, v.push(p))

	translated, err := v.translate(target)
	astTestMust(t, err)
	if n, e := translated.size(); e != nil || n != 1 {
		t.Fatal("translated size", n, e)
	}
	copy, err := translated.get(0)
	astTestMust(t, err)
	if kind, e := targetManager.GetKind(copy); e != nil || kind != ast.ASTApp {
		t.Fatal("translated AST", kind, e)
	}
	astTestMust(t, source.Close())
	if _, e := targetManager.GetKind(copy); e != nil {
		t.Fatal("target vector depended on source context", e)
	}
	astTestMust(t, target.Close())
	if targetManager.IsLiveHandle(copy) {
		t.Fatal("target vector failed to release translated root")
	}
}
