package smtcore

import (
	"errors"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTEnodePopResumesAfterMissingParent(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "pop-parent", 2)
	a, b := f.base[0], f.base[1]
	x, err := f.m.MkApp(f.ternary, []ast.ASTHandle{a, b, a})
	astTestMust(t, err)
	astTestPin(t, f.m, x)
	left, right := f.s.context.enodes.get(a), f.s.context.enodes.get(b)
	originalLeft, originalRight := left.Parents(), right.Parents()
	astTestMust(t, f.s.PushScope())
	n, err := f.s.MkEnode(x, false, false, true)
	astTestMust(t, err)
	// A missing second occurrence must not consume any earlier occurrence.
	left.sourceParents = left.sourceParents[:len(left.sourceParents)-1]
	beforeLeft, beforeRight := left.Parents(), right.Parents()
	if err := f.s.PopScopes(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("missing duplicate parent: %v", err)
	}
	if !slices.Equal(left.Parents(), beforeLeft) || !slices.Equal(right.Parents(), beforeRight) {
		t.Fatal("failed pop changed the other parent entries")
	}
	if f.s.context.enodes.get(x) != n {
		t.Fatal("failed pop unregistered the still-live enode")
	}
	inTable, err := f.s.congruence.ContainsPointer(n)
	if err != nil || !inTable {
		t.Fatalf("failed pop removed the congruence entry: %v, %v", inTable, err)
	}
	left.sourceParents = append(left.sourceParents, n)
	astTestMust(t, f.s.PopScopes(1))
	if f.s.context.enodes.get(x) != nil || !slices.Equal(left.Parents(), originalLeft) || !slices.Equal(right.Parents(), originalRight) {
		t.Fatal("retried pop did not restore the original graph")
	}
	astTestMust(t, f.s.CheckInvariant())
}

func TestSMTEnodeDeleteWithRetiredOperand(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "retired-operand", 2)
	first, err := NewSMTEnode(f.m, f.base[0], nil, SMTEnodeOptions{})
	astTestMust(t, err)
	second, err := NewSMTEnode(f.m, f.base[1], nil, SMTEnodeOptions{})
	astTestMust(t, err)
	x, err := f.m.MkApp(f.binary, f.base)
	astTestMust(t, err)
	astTestPin(t, f.m, x)
	n, err := NewSMTEnode(f.m, x, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{f.base[0]: first, f.base[1]: second}), SMTEnodeOptions{UpdateChildrenParent: true})
	astTestMust(t, err)
	astTestMust(t, second.Delete(false))
	if err := n.Delete(true); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("expected retired operand error, got %v", err)
	}
	if before := first.Parents(); len(before) != 1 || before[0] != n {
		t.Fatalf("refused deletion changed the first parent's links")
	}
	astTestMust(t, n.Delete(false))
	astTestMust(t, first.Delete(false))
}

func TestSMTWiderEnodePopRestoresInteriorParentsOnCongruenceError(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "interior-parents", 3)
	domain := make([]ast.ASTHandle, 11)
	args := make([]ast.ASTHandle, len(domain))
	for i := range args {
		domain[i] = f.sort
		args[i] = f.base[i%len(f.base)]
	}
	decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "wide-parents"), domain, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	x, err := f.m.MkApp(decl, args)
	astTestMust(t, err)
	astTestPin(t, f.m, x)
	astTestMust(t, f.s.PushScope())
	n, err := f.s.MkEnode(x, false, false, true)
	astTestMust(t, err)
	// Publicly constructed, borrowed parents can follow an owned node.
	// Every removal from these three roots is now in the interior.
	borrowed := make([]*SMTEnode, len(f.base))
	before := make([][]*SMTEnode, len(f.base))
	for i, h := range f.base {
		term, e := f.m.MkApp(f.unary, []ast.ASTHandle{h})
		astTestMust(t, e)
		astTestPin(t, f.m, term)
		borrowed[i], e = NewSMTEnode(f.m, term, f.s.context.enodes.get, SMTEnodeOptions{UpdateChildrenParent: true})
		astTestMust(t, e)
		before[i] = f.s.context.enodes.get(h).Parents()
	}
	f.s.congruence.closed = true
	err = f.s.PopScopes(1)
	f.s.congruence.closed = false
	if !errors.Is(err, ast.ErrASTHandle) || f.s.context.enodes.get(x) != n {
		t.Fatalf("missing congruence error or enode: %v", err)
	}
	for i, h := range f.base {
		if !slices.Equal(before[i], f.s.context.enodes.get(h).Parents()) {
			t.Fatalf("refused pop reordered root %d", i)
		}
	}
	astTestMust(t, f.s.PopScopes(1))
	for i, h := range f.base {
		if parents := f.s.context.enodes.get(h).Parents(); len(parents) != 1 || parents[0] != borrowed[i] {
			t.Fatalf("retried pop lost borrowed parent %d", i)
		}
		astTestMust(t, borrowed[i].Delete(true))
	}
	astTestMust(t, f.s.CheckInvariant())
}
