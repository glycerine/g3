package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_idiv/process_mod.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// qfnraPurifyBuilder roots intermediate ASTs until all constraints own them.
type qfnraPurifyBuilder struct {
	temps qfnraPurifyTemps
	err   error
}

func (b *qfnraPurifyBuilder) hold(build func() (ast.ASTHandle, error)) ast.ASTHandle {
	if b.err != nil {
		return ast.ASTHandle{}
	}
	value, err := build()
	if err != nil {
		b.err = err
		return ast.ASTHandle{}
	}
	value, b.err = b.temps.hold(value)
	return value
}

func (i *qfnraPurifyInvocation) processIntegerDivision(x, y ast.ASTHandle, quotient *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	b := qfnraPurifyBuilder{temps: qfnraPurifyTemps{manager: m}}
	defer func() { err = errors.Join(err, b.temps.close()) }()
	mod := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyMod, nil, []ast.ASTHandle{x, y})
	})
	if b.err != nil {
		return b.err
	}
	remainder, err := i.newEntry(mod)
	if err != nil {
		return err
	}
	q, err := m.RefGet(quotient.fresh)
	if err != nil {
		return err
	}
	r, err := m.RefGet(remainder.fresh)
	if err != nil {
		return err
	}
	// GCC evaluates call arguments right to left, so each OR/EQ builds its
	// last argument first.
	zero := b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticIntegerNumeralText("0") })
	// OR(EQ(y, 0), EQ(x, mk_add(mk_mul(k1, y), k2)))
	product := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(q, y) })
	sum := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticAdd(product, r) })
	equation := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(x, sum) })
	yZero := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(y, zero) })
	first := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(yZero, equation) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(first, quotient, remainder); err != nil {
		return err
	}
	// OR(EQ(y, 0), mk_le(0, k2))
	positiveR := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(zero, r) })
	second := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(yZero, positiveR) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(second, remainder); err != nil {
		return err
	}
	// OR(mk_le(y, 0), mk_lt(k2, y))
	rBelowY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(r, y) })
	nonpositiveY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(y, zero) })
	third := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(nonpositiveY, rBelowY) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(third, remainder); err != nil {
		return err
	}
	// OR(mk_ge(y, 0), mk_lt(k2, mk_mul(mk_numeral(-1), y)))
	minusOne := b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticIntegerNumeralText("-1") })
	negY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(minusOne, y) })
	rBelowNegY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(r, negY) })
	nonnegativeY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGE(y, zero) })
	fourth := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(nonnegativeY, rBelowNegY) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(fourth, remainder); err != nil {
		return err
	}
	if !i.owner.complete {
		return nil
	}
	text, _, numeral, err := m.ArithmeticNumeralText(y)
	if err != nil {
		return err
	}
	if numeral && text != "0" {
		return nil
	}
	// OR(NOT(EQ(y, 0)), EQ(k1, mk_idiv(x, 0))): the right disjunct first.
	divZero := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyIDiv, nil, []ast.ASTHandle{x, zero})
	})
	qEq := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(q, divZero) })
	notZero := b.hold(func() (ast.ASTHandle, error) { return m.MkNot(yZero) })
	qGuard := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(notZero, qEq) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(qGuard, quotient); err != nil {
		return err
	}
	modZero := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyMod, nil, []ast.ASTHandle{x, zero})
	})
	rEq := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(r, modZero) })
	rGuard := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(notZero, rEq) })
	if b.err != nil {
		return b.err
	}
	return i.addConstraint(rGuard, remainder)
}
