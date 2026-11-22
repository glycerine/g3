package ast

import (
	"errors"
	"math"
	"testing"
)

func TestASTCanonicalAppHintChecksAllOperands(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	var terms []ASTHandle
	for i := uint32(0); i < 80; i++ {
		h, err := m.MkVar(i, b)
		astTestMust(t, err)
		astTestPin(t, m, h)
		terms = append(terms, h)
	}
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "hint-collision"), []ASTHandle{b, b, b}, b, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	args := []ASTHandle{terms[0], terms[1], terms[2]}
	mk := func() ASTHandle {
		t.Helper()
		h, err := m.MkApp(decl, args)
		astTestMust(t, err)
		return h
	}
	left := mk()
	astTestMust(t, m.IncRef(left))
	d, err := m.internal(decl)
	astTestMust(t, err)
	// Fill from the ordinary table even in validation/diagnostic builds.
	position := func(h ASTHandle) uint32 {
		t.Helper()
		for i, e := range m.table.entries {
			if m.external(e.ref) == h {
				return uint32(i + 1)
			}
		}
		t.Fatal("missing application")
		return 0
	}
	m.table.rememberApp(d, args, position(left))
	slot := m.table.inlineAppHint(d, args)
	if m.external(m.hintedApp(d, args, *slot)) != left {
		t.Fatal("missed canonical application")
	}
	if m.hintedApp(d, args, math.MaxUint32).slot != 0 {
		t.Fatal("used a position beyond the end of the table")
	}
	args[1] = terms[3]
	if m.table.inlineAppHint(d, args) != slot || m.hintedApp(d, args, *slot).slot != 0 {
		t.Fatal("hint collision aliased the middle operand")
	}
	right := mk()
	astTestPin(t, m, right)
	if right == left {
		t.Fatal("distinct arguments share an application")
	}
	wrong := append([]ASTHandle(nil), args...)
	wrong[1].origin++
	*slot = position(right)
	if m.hintedApp(d, wrong, *slot).slot != 0 {
		t.Fatal("ignored operand owner")
	}
	wrong[1] = args[1]
	wrong[1].generation++
	if m.hintedApp(d, wrong, *slot).slot != 0 || m.hintedApp(d, args[:2], *slot).slot != 0 {
		t.Fatal("ignored operand generation or arity")
	}
	wrongDecl := d
	wrongDecl.generation++
	if m.hintedApp(wrongDecl, args, *slot).slot != 0 {
		t.Fatal("ignored declaration generation")
	}
	if _, err := m.MkApp(decl, wrong); !errors.Is(err, ErrASTHandle) {
		t.Fatal("stale operand", err)
	}
	args[1] = terms[1]
	*slot = position(left)
	astTestMust(t, m.DecRef(left))
	if m.hintedApp(d, args, *slot).slot != 0 {
		t.Fatal("used a deleted table entry")
	}
	args[1] = terms[3]
	astTestMust(t, m.CompactMemory())
	if mk() != right {
		t.Fatal("compaction lost the resident application")
	}
	// The ordinary duplicate path consumes one placement generation. Exercise
	// the same transition with a populated hint and an exhausted free head.
	slot = m.table.inlineAppHint(d, args)
	*slot = position(right)
	if mk() != right {
		t.Fatal("priming duplicate")
	}
	free := m.freeNode
	g := m.nodeAt(free).generation
	if mk() != right || m.freeNode != free || m.nodeAt(free).generation != g+1 {
		t.Fatal("ordinary duplicate lifetime changed")
	}
	m.nodeAt(free).generation = math.MaxUint32
	if mk() != right || m.freeNode == free {
		t.Fatal("used an exhausted placement")
	}
}
