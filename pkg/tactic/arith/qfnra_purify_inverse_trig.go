package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_asin/acos/atan.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *qfnraPurifyInvocation) processInverseTrig(op int32, x, fresh ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	b := qfnraPurifyBuilder{temps: qfnraPurifyTemps{manager: m}}
	defer func() { err = errors.Join(err, b.temps.close()) }()
	real := func(value string) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticRealNumeralText(value) })
	}
	app := func(kind int32, args ...ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkFamilyApp(i.owner.arithmetic, kind, nil, args) })
	}
	eq := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkEq(a, c) })
	}
	and := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkAnd(a, c) })
	}
	or := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkOr(a, c) })
	}
	not := func(a ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkNot(a) })
	}
	ge := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGE(a, c) })
	}
	le := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(a, c) })
	}
	gt := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGT(a, c) })
	}
	lt := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(a, c) })
	}
	mul := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(a, c) })
	}
	// GCC evaluates call arguments right to left, so each nested
	// constructor below builds its last argument first.
	if op == qfnraPurifyATan {
		// pi2 = mk_mul(mk_numeral(1/2), mk_pi()); mpi2 likewise.
		pi := app(forwardTrigPi)
		halfPi := mul(real("1/2"), pi)
		pi = app(forwardTrigPi)
		minusHalfPi := mul(real("-1/2"), pi)
		// AND(EQ(x, mk_tan(k)), AND(mk_gt(k, mpi2), mk_lt(k, pi2)))
		below := lt(fresh, halfPi)
		above := gt(fresh, minusHalfPi)
		bounds := and(above, below)
		tan := app(21, fresh)
		constraint := and(eq(x, tan), bounds)
		if b.err != nil {
			return b.err
		}
		return i.addConstraint(constraint, entry)
	}
	one, minusOne := real("1"), real("-1")
	var forwardKind int32
	var low, high ast.ASTHandle
	var undefinedKind int32
	if op == qfnraPurifyASin {
		forwardKind = forwardTrigSin
		pi := app(forwardTrigPi)
		high = mul(real("1/2"), pi)
		pi = app(forwardTrigPi)
		low = mul(real("-1/2"), pi)
		undefinedKind = 34
	} else {
		forwardKind = forwardTrigCos
		high = app(forwardTrigPi)
		low = real("0")
		undefinedKind = 35
	}
	// OR(OR(NOT(mk_ge(x, mone)), NOT(mk_le(x, one))),
	//    AND(EQ(x, mk_sin(k)), AND(mk_ge(k, low), mk_le(k, high))))
	inBounds := func() ast.ASTHandle {
		upper := le(fresh, high)
		lower := ge(fresh, low)
		return and(lower, upper)
	}()
	forward := app(forwardKind, fresh)
	inside := and(eq(x, forward), inBounds)
	upperX := le(x, one)
	notUpper := not(upperX)
	lowerX := ge(x, minusOne)
	outside := or(not(lowerX), notUpper)
	constraint := or(outside, inside)
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(constraint, entry); err != nil {
		return err
	}
	if !i.owner.complete {
		return nil
	}
	// OR(mk_ge(x, mone), EQ(k, mk_u_asin(x))), then OR(mk_le(x, one), ...).
	undefined := app(undefinedKind, x)
	undefinedEq := eq(fresh, undefined)
	lowerGuard := or(ge(x, minusOne), undefinedEq)
	undefined = app(undefinedKind, x)
	undefinedEq = eq(fresh, undefined)
	upperGuard := or(le(x, one), undefinedEq)
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(lowerGuard, entry); err != nil {
		return err
	}
	return i.addConstraint(upperGuard, entry)
}
