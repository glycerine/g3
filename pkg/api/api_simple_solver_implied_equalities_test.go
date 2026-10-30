package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverImpliedEqualitiesRestoreScope(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	terms := make([]ast.ASTHandle, 3)
	for i, name := range []string{"api-simple-implied-a", "api-simple-implied-b", "api-simple-implied-c"} {
		terms[i] = f.pin(f.c.Constant(name, boolean))
	}
	equality := f.pin(f.c.Equal(terms[0], terms[1]))
	astTestMust(t, s.Assert(equality))
	before, err := s.ScopeLevel()
	astTestMust(t, err)
	status, classes, err := s.ImpliedEqualities(terms)
	astTestMust(t, err)
	if status != lbool.True || classes[0] != classes[1] || classes[0] == classes[2] {
		t.Fatal("classes", status, classes)
	}
	after, err := s.ScopeLevel()
	astTestMust(t, err)
	if after != before {
		t.Fatal("scope changed", before, after)
	}
	astTestMust(t, s.Close())
}
