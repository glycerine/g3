package api

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIContextPolynomialOwnerIndependentAndSharedByCopies(t *testing.T) {
	c := newAPIInterruptFixture(t)
	m, err := c.Manager()
	astTestMust(t, err)
	_, err = c.RealSort()
	astTestMust(t, err)
	algebraic, err := m.ArithmeticAlgebraicManager()
	astTestMust(t, err)
	arithPolynomial, err := algebraic.PolynomialManager()
	astTestMust(t, err)
	p, err := c.polynomialManager()
	astTestMust(t, err)
	if p == arithPolynomial {
		t.Fatal("API context borrowed arithmetic plugin's polynomial owner")
	}
	core, err := p.ArithmeticCore()
	astTestMust(t, err)
	integers, err := core.IntegerManager()
	astTestMust(t, err)
	arithCore, err := arithPolynomial.ArithmeticCore()
	astTestMust(t, err)
	arithIntegers, err := arithCore.IntegerManager()
	astTestMust(t, err)
	if integers == arithIntegers || integers == m.NumeralService().Manager.Integers() {
		t.Fatal("context pmanager did not construct its own integer owner")
	}
	if integers.RuntimeOwner() != m.NumeralService().RuntimeOwner() {
		t.Fatal("private integers must borrow the context runtime")
	}
	copyContext := *c
	again, err := copyContext.polynomialManager()
	astTestMust(t, err)
	if again != p {
		t.Fatal("copied context did not share its persistent polynomial owner")
	}
	other := newAPIInterruptFixture(t)
	otherPM, err := other.polynomialManager()
	astTestMust(t, err)
	if otherPM == p {
		t.Fatal("independent contexts share their polynomial owner")
	}
	astTestMust(t, copyContext.Close())
	if !integers.Closed() || !c.polyLimit.Closed() {
		t.Fatal("context close did not release polynomial member owners")
	}
	if _, err = core.IntegerManager(); err == nil {
		t.Fatal("polynomial core survived context destruction")
	}
	if _, err = c.polynomialManager(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("polynomial access after shared context close", err)
	}
}

func TestAPIContextInterruptCancelsBothLimitsAfterHandler(t *testing.T) {
	c := newAPIInterruptFixture(t)
	m, err := c.Manager()
	astTestMust(t, err)
	limit, err := c.polynomialLimit()
	astTestMust(t, err)
	calls := 0
	scope, err := newSMTContextInterruptScope(c, control.EventHandlerFunc(func(caller control.EventCaller) {
		if caller != control.EventCallerAPIInterrupt || limit.CancelFlag() || m.ResourceLimit().CancelFlag() {
			t.Fatal("context canceled member limits before invoking the active handler")
		}
		calls++
	}))
	astTestMust(t, err)
	astTestMust(t, c.Interrupt())
	astTestMust(t, scope.Close())
	if calls != 1 || !limit.CancelFlag() || !m.ResourceLimit().CancelFlag() {
		t.Fatal("interrupt did not cancel both context member limits", calls)
	}
	limit.ResetCancel()
	m.ResourceLimit().ResetCancel()
	astTestMust(t, c.Interrupt())
	if !limit.CancelFlag() || !m.ResourceLimit().CancelFlag() {
		t.Fatal("context member cancellation depends on an installed handler")
	}
}

func TestAPIPolynomialScopeRoutesOnlyPolynomialLimit(t *testing.T) {
	c := newAPIInterruptFixture(t)
	m, err := c.Manager()
	astTestMust(t, err)
	limit, err := c.polynomialLimit()
	astTestMust(t, err)
	scope, err := newAPIPolynomialInterruptScope(c)
	astTestMust(t, err)
	scope.handler.HandleEvent(control.EventCallerTimeout)
	astTestMust(t, scope.Close())
	if !limit.CancelFlag() || m.ResourceLimit().CancelFlag() {
		t.Fatal("polynomial handler canceled the wrong context limit")
	}
	limit.ResetCancel()
	algebraicScope, err := newAPIOperationInterruptScope(c)
	astTestMust(t, err)
	algebraicScope.handler.HandleEvent(control.EventCallerTimeout)
	astTestMust(t, algebraicScope.Close())
	if limit.CancelFlag() || !m.ResourceLimit().CancelFlag() {
		t.Fatal("algebraic handler must use the AST limit")
	}
}

func TestAPIPolynomialAndAlgebraicConvertersUseContextOwner(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	inputs, err := f.c.newPolynomialInputs(f.m, f.p, f.q)
	astTestMust(t, err)
	if inputs.manager != f.c.polynomial.manager {
		t.Fatal("subresultant converter bypassed context polynomial owner")
	}
	astTestMust(t, inputs.close())
	x := algebraicPolynomialVariable(t, f.apiAlgebraicFixture, 0)
	p := algebraicPolynomialSquareMinus(t, f.apiAlgebraicFixture, x, f.two)
	converted, err := f.c.newAlgebraicPolynomial(f.m, p)
	astTestMust(t, err)
	if converted.manager != f.c.polynomial.manager {
		t.Fatal("algebraic converter bypassed context polynomial owner")
	}
	astTestMust(t, converted.close())
	// Conversion into the private integer domain and back must preserve large
	// coefficients, not just small immediates shared by the fixture.
	large := f.pin(f.c.Numeral("1234567890123456789012345678901234567890", f.pin(f.c.RealSort())))
	expression := f.pin(f.c.ArithmeticAdd(large, x))
	value, err := f.c.AlgebraicEval(expression, []ast.ASTHandle{f.one})
	astTestMust(t, err)
	if value != 1 {
		t.Fatal("large private-domain coefficient sign", value)
	}
}

func TestAPIContextPolynomialConstructorRefusalDoesNotPublishOwner(t *testing.T) {
	c := newAPIInterruptFixture(t)
	m, err := c.Manager()
	astTestMust(t, err)
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	_, err = c.polynomialManager()
	m.Budget().FailAfter(-1)
	if err == nil || c.polynomial != nil || m.Budget().Used() != before {
		t.Fatal("failed construction published or leaked polynomial owner", err)
	}
	_, err = c.polynomialManager()
	astTestMust(t, err)
}

func TestAPIContextPolynomialConstructorAllocationPrefixesClose(t *testing.T) {
	success := false
	for stop := int64(0); stop < 128 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := newAPIInterruptFixture(t)
			m, err := c.Manager()
			astTestMust(t, err)
			m.Budget().FailAfter(stop)
			_, err = c.polynomialManager()
			m.Budget().FailAfter(-1)
			success = err == nil
			// Failed source members may own physical numerical construction
			// prefixes; context Close must drain those before its runtime.
			astTestMust(t, c.Close())
			if !c.closed || c.polynomial != nil || !c.polyLimit.Closed() {
				t.Fatal("context did not finish polynomial member teardown", err)
			}
		})
	}
	if !success {
		t.Fatal("polynomial constructor did not complete within allocation horizon")
	}
}

func TestAPIPolynomialSubresultantsUsesIndependentCancellation(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	limit, err := f.c.polynomialLimit()
	astTestMust(t, err)
	limit.Cancel()
	result, err := f.c.PolynomialSubresultants(f.p, f.q, f.x)
	if result != nil || !errors.Is(err, polynomial.ErrUPolynomialCanceled) {
		t.Fatal("subresultants bypassed the context polynomial limit", result, err)
	}
	if f.m.ResourceLimit().CancelFlag() {
		t.Fatal("polynomial-only cancellation poisoned AST services")
	}
	limit.ResetCancel()
	result, err = f.c.PolynomialSubresultants(f.p, f.q, f.x)
	astTestMust(t, err)
	astTestMust(t, result.Close())
}

func TestAPIPolynomialPrivateOwnerExactResultants(t *testing.T) {
	for _, constant := range []string{"2", "1234567890123456789012345678901234567890"} {
		t.Run(constant, func(t *testing.T) {
			f := newAPIPolynomialFixture(t)
			a := f.pin(f.c.Numeral(constant, f.pin(f.c.RealSort())))
			xSquared := f.pin(f.c.ArithmeticMul(f.x, f.x))
			p := f.pin(f.c.ArithmeticSub(xSquared, a))
			result, err := f.c.PolynomialSubresultants(p, f.q, f.x)
			astTestMust(t, err)
			defer func() { astTestMust(t, result.Close()) }()
			values, err := result.Values()
			astTestMust(t, err)
			if len(values) != 1 {
				t.Fatal("degree-two/degree-one principal subresultants", len(values))
			}
			// This source chain omits the degree-one leading coefficient.
			// Its sole entry is Res(x²-a, x-1) = 1-a.
			aValue, _ := new(big.Int).SetString(constant, 10)
			want := []string{new(big.Int).Sub(big.NewInt(1), aValue).String()}
			for i, value := range values {
				actual, err := f.c.NumeralString(value)
				astTestMust(t, err)
				if actual != want[i] {
					t.Fatalf("coefficient %d = %s, want %s", i, actual, want[i])
				}
			}
		})
	}
}
