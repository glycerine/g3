package smtcore

import (
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// An interpreter backtrack point holds this slice while other declarations
// and applications may be registered. Suppressed applications are never
// candidates, even if a declaration changes from one application to many.
func TestSMTDeclarationCandidatesSurviveMixedScopes(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "decl-candidates", 4)
	terms := make([]ast.ASTHandle, len(f.base))
	for i, h := range f.base {
		x, e := f.m.MkApp(f.unary, []ast.ASTHandle{h})
		astTestMust(t, e)
		astTestPin(t, f.m, x)
		terms[i] = x
	}
	it := newSMTMAMInterpreter(f.s, nil, false)
	astTestMust(t, f.s.PushScope())
	a, e := f.s.MkEnode(terms[0], false, false, true)
	astTestMust(t, e)
	view := it.enodesOf(f.unary)
	if !slices.Equal(view, []*SMTEnode{a}) {
		t.Fatal("missing singleton candidate")
	}
	astTestMust(t, f.s.PushScope())
	_, e = f.s.MkEnode(terms[1], true, false, true)
	astTestMust(t, e)
	if got := it.enodesOf(f.unary); !slices.Equal(got, view) {
		t.Fatal("suppressed application entered the candidate list")
	}
	astTestMust(t, f.s.PopScopes(1))
	astTestMust(t, f.s.PopScopes(1))
	if view[0] != nil {
		t.Fatal("popped candidate remained in the borrowed interpreter range")
	}
	astTestMust(t, f.s.PushScope())
	_, e = f.s.MkEnode(terms[2], true, false, true)
	astTestMust(t, e)
	if view[0] != nil || len(it.enodesOf(f.unary)) != 0 {
		t.Fatal("a suppressed node occupied a borrowed candidate slot")
	}
	b, e := f.s.MkEnode(terms[3], false, false, true)
	astTestMust(t, e)
	if view[0] != b || !slices.Equal(it.enodesOf(f.unary), []*SMTEnode{b}) {
		t.Fatal("could not refill the matching range independently")
	}
	astTestMust(t, f.s.PopScopes(1))
	if view[0] != nil {
		t.Fatal("candidate slot retained second node")
	}
}

// The second internalizer can read applications created by the first, even
// when neither has requested a declaration index before the handoff.
func TestSMTDeclarationIndexAfterCheckerRebind(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "decl-rebind", 3)
	terms := make([]ast.ASTHandle, len(f.base))
	for i, arg := range f.base {
		term, err := f.m.MkApp(f.unary, []ast.ASTHandle{arg})
		astTestMust(t, err)
		astTestPin(t, f.m, term)
		terms[i] = term
	}
	ctx := f.s.context
	first, err := f.s.MkEnode(terms[0], false, false, true)
	astTestMust(t, err)
	other, err := NewSMTInternalizerState(f.m, ctx, nil)
	astTestMust(t, err)
	hidden, err := other.MkEnode(terms[1], true, false, true)
	astTestMust(t, err)
	third, err := f.s.MkEnode(terms[2], false, false, true)
	astTestMust(t, err)
	if !slices.Equal(ctx.declarationEnodes(f.unary), []*SMTEnode{first, hidden, third}) {
		t.Fatal("first declaration lookup lost registration order across owners")
	}
	it := newSMTMAMInterpreter(other, nil, false)
	if !slices.Equal(it.enodesOf(f.unary), []*SMTEnode{first, third}) {
		t.Fatal("first matching lookup lost a candidate from the previous owner")
	}
	astTestMust(t, other.Close())
	if !slices.Equal(ctx.declarationEnodes(f.unary), []*SMTEnode{first, third}) {
		t.Fatal("closing the second owner lost a surviving application")
	}
	astTestMust(t, f.s.Close())
}
