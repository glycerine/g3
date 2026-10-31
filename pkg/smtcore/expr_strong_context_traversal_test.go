package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprStrongContextSimplifierModelBased(t *testing.T) {
	m, fixture := smtSolverFixture(t)
	s, err := NewExprStrongContextSimplifier(m, fixture.config)
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "strong-model-p"), boolean)
	astTestMust(t, err)
	pRef, err := m.NewRef(p)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&pRef)) }()
	q, err := m.MkConst(astTestSymbol(t, m, "strong-model-q"), boolean)
	astTestMust(t, err)
	qRef, err := m.NewRef(q)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&qRef)) }()
	conjunction, err := m.MkAnd(p, q)
	astTestMust(t, err)
	conjunctionRef, err := m.NewRef(conjunction)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&conjunctionRef)) }()
	simplify := func(expression ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		ref, e := s.Simplify(expression)
		astTestMust(t, e)
		h, e := m.RefGet(ref)
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
		return h
	}
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(p))
	if got := simplify(p); !m.IsTrue(got) {
		t.Fatalf("p under p = %v, want true", got)
	}
	got := simplify(conjunction)
	count, err := m.GetNumArgs(got)
	astTestMust(t, err)
	if count != 2 {
		t.Fatalf("simplified conjunction has %d args", count)
	}
	first, err := m.GetArg(got, 0)
	astTestMust(t, err)
	second, err := m.GetArg(got, 1)
	astTestMust(t, err)
	if !m.IsTrue(first) || second != q {
		t.Fatalf("unexpected simplification: %v %v", first, second)
	}
	astTestMust(t, s.Pop())
	if got := simplify(p); got != p {
		t.Fatalf("p without assumptions = %v", got)
	}
	notP, err := m.MkNot(p)
	astTestMust(t, err)
	notPRef, err := m.NewRef(notP)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&notPRef)) }()
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(notP))
	if got := simplify(p); !m.IsFalse(got) {
		t.Fatalf("p under not p = %v, want false", got)
	}
	astTestMust(t, s.Pop())
}
