package qe

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestFMNumeralRecognitionUsesOnlySourceIdentity(t *testing.T) {
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error { return failure })
	m := f.r.manager
	x := f.variable(0, f.real)
	wide := f.own(m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
	product := f.own(m.MkArithmeticMul(wide, x))
	bound := f.own(m.MkArithmeticLE(product, wide))
	before, count := m.Budget().Used(), f.runtime.State().GlobalCount
	m.Budget().FailAfter(0)
	variable, monomial, monomialErr := f.r.monomial(product)
	inequality, inequalityErr := f.r.inequality(bound)
	m.Budget().FailAfter(-1)
	fmMust(t, monomialErr)
	fmMust(t, inequalityErr)
	if !monomial || variable != x || !inequality {
		t.Fatal("source identity-only numeral recognition changed", variable, monomial, inequality)
	}
	if m.Budget().Used() != before || f.runtime.State().GlobalCount != count {
		t.Fatal("numeral recognition allocated or formatted its stored value")
	}
	// The no-value source overload tests only the family and operator. Raw
	// declaration metadata is inspected later, when add_constraint copies it.
	parameter, err := m.NewParameterInt(4)
	fmMust(t, err)
	info, err := m.NewFuncDeclInfo(f.r.arithmetic, 0, []ast.ASTParameter{parameter, parameter}, ast.ASTFuncFlags{}, false)
	fmMust(t, err)
	label, err := m.Symbols.Intern("fm-identity-only-numeral")
	fmMust(t, err)
	declaration := f.own(m.MkFuncDecl(label, nil, f.real, info))
	fmMust(t, m.DelParameter(&parameter))
	raw := f.own(m.MkApp(declaration, nil))
	product = f.own(m.MkArithmeticMul(raw, x))
	bound = f.own(m.MkArithmeticLE(product, raw))
	variable, monomial, err = f.r.monomial(product)
	fmMust(t, err)
	inequality, err = f.r.inequality(bound)
	fmMust(t, err)
	if variable != x || !monomial || !inequality {
		t.Fatal("identity recognition inspected numeral metadata")
	}
	if result, cause := f.store().parse(bound); result != nil || !errors.Is(cause, ast.ErrASTDomain) {
		t.Fatal("constraint ingestion did not validate the source numeral value", result, cause)
	}
}

func TestFMNumeralReadFailurePreservesScopeAndRetry(t *testing.T) {
	for _, coefficient := range []bool{false, true} {
		name := "right-hand-side"
		if coefficient {
			name = "coefficient"
		}
		t.Run(name, func(t *testing.T) {
			var store *fmStore
			var stopped *fmScope
			active := false
			f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
				if active {
					if len(store.pending) != 1 {
						t.Error("native copy lost its owning parse scope")
						return failure
					}
					stopped = store.pending[0]
					reader := stopped.numeralRead
					if err := stopped.close(); !errors.Is(err, arena.ErrInvalidSlot) && !errors.Is(err, numeral.ErrRationalLexicalPending) {
						t.Errorf("active native copy cleanup did not refuse: %v", err)
					}
					wantNumbers := 1
					if coefficient {
						wantNumbers = 2
					}
					if reader == nil || stopped.numeralRead != reader || reader.bytes == 0 || len(stopped.numbers) != wantNumbers || reader.receiver != stopped.numbers[wantNumbers-1] {
						t.Error("native refusal lost the direct source destination or copy owner")
					}
				}
				return failure
			})
			m := f.r.manager
			x := f.variable(0, f.real)
			text := strings.Repeat("9", 600)
			wide := f.own(m.ArithmeticRealNumeralText(text))
			lhs, rhs := x, wide
			if coefficient {
				lhs = f.own(m.MkArithmeticMul(wide, x))
				rhs = f.own(m.ArithmeticRealNumeralText("1"))
			}
			form := f.own(m.MkArithmeticLE(lhs, rhs))
			store = f.store()
			fmMust(t, f.runtime.SetMaxSize(1))
			active = true
			constraint, cause := store.parse(form)
			active = false
			fmMust(t, f.runtime.SetMaxSize(0))
			var failure memory.Failure
			if constraint != nil || !errors.As(cause, &failure) || stopped == nil {
				t.Fatal("parse missed the actual stored-numeral native copy failure", constraint, cause)
			}
			fmMust(t, store.drain())
			fmMust(t, store.drain())
			if stopped.numeralRead != nil || len(stopped.numbers) != 0 || len(store.pending) != 0 {
				t.Fatal("completed parse cleanup left its stopped copy owner")
			}
			constraint, err := store.parse(form)
			fmMust(t, err)
			if coefficient {
				fmNumbers(t, store, constraint, "1", text)
			} else {
				fmNumbers(t, store, constraint, text, "1")
			}
		})
	}
}

func TestFMNumeralReadReservationAndNonmatch(t *testing.T) {
	f := newFMFixture(t)
	store := f.store()
	m := f.r.manager
	h := f.number("3")
	scope, err := store.begin()
	fmMust(t, err)
	value, err := scope.number()
	fmMust(t, err)
	fmMust(t, m.NumeralService().SetInt64(value, 7))
	m.Budget().FailAfter(0)
	cause := store.numeral(scope, h, value)
	m.Budget().FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || scope.numeralRead != nil {
		t.Fatal("failed reader reservation lost its source destination", cause)
	}
	variable := f.variable(0, f.integer)
	if cause = store.numeral(scope, variable, value); !errors.Is(cause, ast.ErrASTDomain) {
		t.Fatal("non-numeral changed the source constraint error", cause)
	}
	text, err := m.NumeralService().String(value)
	fmMust(t, err)
	if text != "7" || scope.numeralRead.receiver != nil {
		t.Fatal("nonmatch changed the borrowed destination", text)
	}
	fmMust(t, store.numeral(scope, h, value))
	text, err = m.NumeralService().String(value)
	fmMust(t, err)
	if text != "3" {
		t.Fatal("reader reuse did not copy source value", text)
	}
	fmMust(t, store.drain())
	fmMust(t, store.drain())
}
