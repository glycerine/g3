package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestStrongContextAssignmentPropagation(t *testing.T) {
	m, fixture := smtSolverFixture(t)
	s, err := NewExprStrongContextSimplifier(m, fixture.config)
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "strong-assignment-p"), boolean)
	astTestMust(t, err)
	q, err := m.MkConst(astTestSymbol(t, m, "strong-assignment-q"), boolean)
	astTestMust(t, err)
	notQ, err := m.MkNot(q)
	astTestMust(t, err)
	formula, err := m.MkAnd(p, notQ)
	astTestMust(t, err)
	root, err := m.NewRef(formula)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&root)) }()
	snapshot, status, err := s.captureAssignments(formula)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("assignment fixture is not satisfiable", status)
	}
	defer func() { astTestMust(t, snapshot.Close()) }()
	if snapshot.values[p] != lbool.True || snapshot.values[q] != lbool.False || snapshot.values[notQ] != lbool.True || snapshot.values[formula] != lbool.True {
		t.Fatal("Boolean assignments were not propagated", snapshot.values)
	}
	unknown, err := m.MkConst(astTestSymbol(t, m, "strong-assignment-unknown"), boolean)
	astTestMust(t, err)
	combined, err := m.MkAnd(p, unknown)
	astTestMust(t, err)
	combinedRef, err := m.NewRef(combined)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&combinedRef)) }()
	manual := &strongContextAssignments{manager: m, values: map[ast.ASTHandle]lbool.LBool{p: lbool.True}}
	astTestMust(t, manual.propagate(combined))
	if manual.values[combined] != lbool.Undef {
		t.Fatal("unknown conjunct was not preserved")
	}
}
