package ast

import "testing"

// Each operand occurrence contributes a reference. Deletion enqueues the
// declaration before the operands and processes the last dead operand first.
func TestASTInlineApplicationDiamondLifetime(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "diamond"), []ASTHandle{b, b}, b, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	first, err := m.MkVar(40, b)
	astTestMust(t, err)
	second, err := m.MkVar(41, b)
	astTestMust(t, err)
	left, err := m.MkApp(decl, []ASTHandle{first, first})
	astTestMust(t, err)
	right, err := m.MkApp(decl, []ASTHandle{first, second})
	astTestMust(t, err)
	root, err := m.MkApp(decl, []ASTHandle{left, right})
	astTestMust(t, err)
	astTestMust(t, m.IncRef(root))
	if got := m.nodeAt(first.slot).rc; got != 3 {
		t.Fatalf("shared leaf has %d references, want 3", got)
	}
	if a := m.appAt(root.slot); a.ground || a.quantifiers || a.labels || a.depth != 3 {
		t.Fatalf("unexpected root flags or depth: %+v", a)
	}
	astTestMust(t, m.DecRef(root))
	// Deletion order is root, right, second, left, first. The arena
	// free list gives these placements back in reverse deletion order.
	slot := m.freeNode
	for _, want := range []ASTHandle{first, left, second, right, root} {
		if slot != want.slot || m.nodeAt(slot).live {
			t.Fatalf("free slot %d, want dead placement %d", slot, want.slot)
		}
		slot = m.nodeAt(slot).kindIndex
	}
	if got := m.nodeAt(decl.slot).rc; got != 1 {
		t.Fatalf("declaration has %d references after cascade, want 1", got)
	}
	reused, err := m.MkVar(42, b)
	astTestMust(t, err)
	astTestPin(t, m, reused)
	if reused.slot != first.slot || reused.generation == first.generation {
		t.Fatalf("reused placement %v, want new generation of %v", reused, first)
	}
}
