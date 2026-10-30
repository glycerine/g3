package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSATGoalTranslatorSourceUnsupportedSMT2Diagnostic(t *testing.T) {
	m := astTestManager(t)
	goalTraceBindParams(t, m)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	h := goalTestApp(t, m, ast.ASTOpXor, p, q)
	s, _, _ := satSolverCheckFixture(t)
	tr, err := NewGoalTranslator(m, s)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	err = tr.Compile([]ast.ASTHandle{h})
	if !errors.Is(err, ErrSATGoalUnsupported) {
		t.Fatalf("unsupported source operator class: %v", err)
	}
	var tacticError tactic.TacticException
	var baseError z3exception.Z3Exception
	if !errors.As(err, &tacticError) || !errors.As(err, &baseError) {
		t.Fatal("source tactic/base exception classes were lost", err)
	}
	want := "operator (xor p q) not supported, apply simplifier before invoking translator"
	if err.Error() != want {
		t.Fatalf("unsupported source operator diagnostic: got %q, want %q", err, want)
	}
}

func TestSATGoalTranslatorStandaloneUnsupportedSMT2Diagnostic(t *testing.T) {
	for _, test := range []struct {
		name, want string
		op         int32
	}{
		{"xor", "operator (xor p q) not supported, apply simplifier before invoking translator", ast.ASTOpXor},
		{"implies", "operator (=> p q) not supported, apply simplifier before invoking translator", ast.ASTOpImplies},
		{"distinct", "operator (distinct p q) not supported, apply simplifier before invoking translator", ast.ASTOpDistinct},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			p := goalTestAtom(t, m, "p")
			q := goalTestAtom(t, m, "q")
			formula := goalTestApp(t, m, test.op, p, q)
			solver, _, _ := satSolverCheckFixture(t)
			translator, err := NewGoalTranslator(m, solver)
			satOK(t, err)
			t.Cleanup(func() { satOK(t, translator.Close()) })
			err = translator.Compile([]ast.ASTHandle{formula})
			if !errors.Is(err, ErrSATGoalUnsupported) {
				t.Fatalf("unsupported source operator class: %v", err)
			}
			if err.Error() != test.want {
				t.Fatalf("standalone unsupported diagnostic: got %q, want %q", err, test.want)
			}
		})
	}
}
