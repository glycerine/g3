package smtcore

import (
	"errors"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTFailedEnodeCreationLeavesParentsIntact(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "enode-failure", 1)
	a := f.base[0]
	missing, err := f.m.MkConst(astTestSymbol(t, f.m, "missing-enode"), f.sort)
	astTestMust(t, err)
	expr, err := f.m.MkApp(f.ternary, []ast.ASTHandle{a, a, missing})
	astTestMust(t, err)
	astTestPin(t, f.m, expr)
	child := f.s.context.enodes.get(a)
	before := child.Parents()
	n, err := f.s.MkEnode(expr, false, false, true)
	if n != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("missing child: node %p, error %v", n, err)
	}
	if parents := child.Parents(); !slices.Equal(parents, before) {
		t.Fatalf("failed construction added %d parent entries", len(parents)-len(before))
	}
	astTestMust(t, f.s.Internalize(missing, false))
	astTestMust(t, f.s.PushScope())
	n, err = f.s.MkEnode(expr, false, false, true)
	astTestMust(t, err)
	parents := child.Parents()
	if len(parents) != len(before)+2 || parents[len(before)] != n || parents[len(before)+1] != n {
		t.Fatalf("retry did not record both occurrences of the child")
	}
	astTestMust(t, f.s.PopScopes(1))
	if !slices.Equal(child.Parents(), before) {
		t.Fatal("scope pop retained a parent from the retried construction")
	}
}

func TestSMTEnodeLookupCanRetireAnEarlierChild(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "enode-callback", 2)
	expr, err := f.m.MkApp(f.binary, f.base)
	astTestMust(t, err)
	astTestPin(t, f.m, expr)
	if n, err := NewSMTEnode(f.m, expr, nil, SMTEnodeOptions{}); n != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("missing lookup: node %p, error %v", n, err)
	}
	first, err := NewSMTEnode(f.m, f.base[0], nil, SMTEnodeOptions{})
	astTestMust(t, err)
	second := f.s.context.enodes.get(f.base[1])
	before := second.Parents()
	lookup := func(h ast.ASTHandle) *SMTEnode {
		if h == f.base[0] {
			return first
		}
		astTestMust(t, first.Delete(false))
		return second
	}
	n, err := NewSMTEnode(f.m, expr, lookup, SMTEnodeOptions{UpdateChildrenParent: true})
	if n != nil || !errors.Is(err, ast.ErrASTDomain) || !slices.Equal(second.Parents(), before) {
		t.Fatalf("retired operand was accepted or published: node %p, error %v", n, err)
	}
}
