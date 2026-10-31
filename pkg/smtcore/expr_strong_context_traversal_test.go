package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
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

func TestExprStrongContextSimplifierPreservesEquivalence(t *testing.T) {
	m, solver := smtSolverFixture(t)
	s, err := NewExprStrongContextSimplifier(m, solver.config)
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			astTestMust(t, m.DelRef(&refs[i]))
		}
	}()
	own := func(h ast.ASTHandle, e error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, e)
		ref, e := m.NewRef(h)
		astTestMust(t, e)
		refs = append(refs, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	p := own(m.MkConst(astTestSymbol(t, m, "strong-equiv-p"), boolean))
	q := own(m.MkConst(astTestSymbol(t, m, "strong-equiv-q"), boolean))
	notP := own(m.MkNot(p))
	shared := own(m.MkOr(p, q))
	left := own(m.MkAnd(shared, p))
	right := own(m.MkAnd(shared, notP))
	formula := own(m.MkOr(left, right))
	for _, assumed := range []bool{false, true} {
		astTestMust(t, s.Push())
		astTestMust(t, solver.Push())
		if assumed {
			astTestMust(t, s.Assert(p))
			astTestMust(t, solver.AssertFormula(p))
		}
		ref, e := s.Simplify(formula)
		astTestMust(t, e)
		simplified, e := m.RefGet(ref)
		astTestMust(t, e)
		equality := own(m.MkIff(formula, simplified))
		difference := own(m.MkNot(equality))
		astTestMust(t, solver.AssertFormula(difference))
		status, e := solver.CheckSat(nil)
		astTestMust(t, e)
		if status != lbool.False {
			t.Fatalf("assumed=%t: simplified formula is not equivalent: %v", assumed, status)
		}
		astTestMust(t, m.DelRef(&ref))
		astTestMust(t, solver.Pop(1))
		astTestMust(t, s.Pop())
	}
	integer := own(m.ArithmeticIntegerNumeralText("17"))
	ref, err := s.Simplify(integer)
	astTestMust(t, err)
	got, err := m.RefGet(ref)
	astTestMust(t, err)
	if got != integer {
		t.Fatal("non-Boolean expression changed")
	}
	astTestMust(t, m.DelRef(&ref))
	v := own(m.MkVar(0, boolean))
	quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v}))
	ref, err = s.Simplify(quantified)
	astTestMust(t, err)
	got, err = m.RefGet(ref)
	astTestMust(t, err)
	if got != quantified {
		t.Fatal("quantified expression changed")
	}
	astTestMust(t, m.DelRef(&ref))
}

func TestExprStrongContextSimplifierAllocationFailureCanRetry(t *testing.T) {
	refusals := 0
	for _, stop := range []int64{0, 1, 2, 4, 8, 16, 32, 64, 128} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, fixture := smtSolverFixture(t)
			s, err := NewExprStrongContextSimplifier(m, fixture.config)
			astTestMust(t, err)
			defer func() { astTestMust(t, s.Close()) }()
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			p, err := m.MkConst(astTestSymbol(t, m, "strong-retry-p"), boolean)
			astTestMust(t, err)
			ref, err := m.NewRef(p)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&ref)) }()
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(p))
			m.Budget().FailAfter(stop)
			failed, err := s.Simplify(p)
			m.Budget().FailAfter(-1)
			if failed != (ast.ASTRef{}) {
				astTestMust(t, m.DelRef(&failed))
			}
			if err != nil {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatalf("unexpected failure: %v", err)
				}
				refusals++
			}
			result, err := s.Simplify(p)
			astTestMust(t, err)
			got, err := m.RefGet(result)
			astTestMust(t, err)
			if !m.IsTrue(got) {
				t.Fatal("simplifier lost its asserted context after retry")
			}
			astTestMust(t, m.DelRef(&result))
			astTestMust(t, s.Pop())
			falsity, err := m.MkFalse()
			astTestMust(t, err)
			forced, err := s.isForced(p, falsity)
			astTestMust(t, err)
			if forced {
				t.Fatal("failure left a solver scope asserted")
			}
		})
	}
	if refusals == 0 {
		t.Fatal("injection did not reach an allocation")
	}
}
