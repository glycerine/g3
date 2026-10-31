package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExprStrongContextSimplifierScopedForcedChecks(t *testing.T) {
	m, fixture := smtSolverFixture(t)
	s, err := NewExprStrongContextSimplifier(m, fixture.config)
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	decl, err := m.RefGet(s.name)
	astTestMust(t, err)
	name, err := m.GetASTName(decl)
	astTestMust(t, err)
	if name != symbol.Numerical(0xbeef101) {
		t.Fatal("strong simplifier naming declaration differs from source")
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "strong-context-p"), boolean)
	astTestMust(t, err)
	pRef, err := m.NewRef(p)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&pRef)) }()
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(p))
	forced, err := s.isForced(p, falsity)
	astTestMust(t, err)
	if !forced {
		t.Fatal("context did not force p to be true")
	}
	forced, err = s.isForced(p, truth)
	astTestMust(t, err)
	if forced {
		t.Fatal("satisfiable value was marked impossible")
	}
	astTestMust(t, s.Pop())
	forced, err = s.isForced(p, falsity)
	astTestMust(t, err)
	if forced {
		t.Fatal("popped context still constrained p")
	}
	copy := *s
	if err = copy.Push(); err != ast.ErrASTHandle {
		t.Fatal("copied owner was accepted", err)
	}
	astTestMust(t, s.Close())
	if err = s.Push(); err != arena.ErrClosed {
		t.Fatal("closed simplifier accepted a push", err)
	}
}
