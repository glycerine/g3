package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTInternalizeRequiresLiveLocalExpression(t *testing.T) {
	m := astTestManager(t)
	_, _, solver := smtInternalizerFixture(t, m, nil)
	other := astTestManager(t)
	sort, err := other.MkBoolSort()
	astTestMust(t, err)
	var foreign ast.ASTHandle
	for i := uint32(0); i < 512; i++ {
		foreign, err = other.MkVar(i, sort)
		astTestMust(t, err)
	}
	localSort, err := m.MkBoolSort()
	astTestMust(t, err)
	local, err := m.MkVar(9, localSort)
	astTestMust(t, err)
	ref, err := m.NewRef(local)
	astTestMust(t, err)
	for _, gate := range []bool{true, false} {
		for _, bad := range []ast.ASTHandle{{}, foreign} {
			if got := solver.Internalize(bad, gate); !errors.Is(got, ast.ErrASTHandle) {
				t.Fatalf("invalid handle returned %v", got)
			}
		}
		if got := solver.Internalize(local, gate); !errors.Is(got, ast.ErrASTDomain) {
			t.Fatalf("free variable returned %v", got)
		}
		if got := solver.Internalize(localSort, gate); !errors.Is(got, ast.ErrASTDomain) {
			t.Fatalf("sort returned %v", got)
		}
	}
	astTestMust(t, m.DelRef(&ref))
	if got := solver.Internalize(local, true); !errors.Is(got, ast.ErrASTHandle) {
		t.Fatalf("retired expression returned %v", got)
	}
	for _, bad := range []ast.ASTHandle{{}, foreign, local} {
		if _, err := NewSMTEnode(m, bad, nil, SMTEnodeOptions{}); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatalf("enode accepted an invalid owner: %v", err)
		}
	}
	valid := smtRelevancyBool(t, m, "after-invalid-expression")
	astTestMust(t, solver.Internalize(valid, true))
	if _, found := solver.BoolVar(valid); !found {
		t.Fatal("live local atom missing")
	}
}
