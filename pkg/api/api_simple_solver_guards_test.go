package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPISimpleSolverRejectsForeignFormulaAndPopUnderflow(t *testing.T) {
	left := newSMTAPIFixture(t, "QF_UF", nil)
	right := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := left.c.NewSimpleSolver()
	astTestMust(t, err)
	if _, err = s.Model(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("model before check", err)
	}
	if _, err = s.Proof(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("proof before check", err)
	}
	foreign := right.pin(right.c.Bool(true))
	if err = s.Assert(foreign); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("foreign assertion", err)
	}
	if err = s.Pop(1); err == nil {
		t.Fatal("scope underflow accepted")
	}
	astTestMust(t, s.Close())
}
