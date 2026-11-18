package smtcore

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Test helpers: math/big views of theory_str's rationals.

// strTestQ is an inline rational(v) for a small v.
func strTestQ(v int64) *numeral.Rational { return smtQIntPtr(int32(v)) }

// testBig is r as math/big.
func (t *SMTStringTheory) testBig(r *numeral.Rational) *big.Int {
	if r == nil {
		return nil
	}
	b, err := smtQBig(t.q(), r)
	if err != nil {
		panic(err)
	}
	return new(big.Int).Set(b.Num())
}

// testRational is a rational of the theory's manager from math/big.
func (t *SMTStringTheory) testRational(b *big.Int) *numeral.Rational {
	r := smtQZero()
	if err := smtQFromBig(t.q(), &r, new(big.Rat).SetInt(b)); err != nil {
		panic(err)
	}
	return &r
}

func (t *SMTStringTheory) testLengthValue(e ast.ASTHandle) (*big.Int, bool, error) {
	r := smtQZero()
	defer smtQDel(t.q(), &r)
	known, err := t.lengthValue(e, &r)
	return t.testBig(&r), known, err
}

func (t *SMTStringTheory) testArithmeticValue(e ast.ASTHandle) (*big.Int, bool, error) {
	r := smtQZero()
	defer smtQDel(t.q(), &r)
	known, err := t.arithmeticValue(e, &r)
	if !r.IsInt() {
		return nil, known, err
	}
	return t.testBig(&r), known, err
}

func (t *SMTStringTheory) testArithmeticBound(e ast.ASTHandle, lower bool) (*big.Int, bool, error) {
	r := smtQZero()
	defer smtQDel(t.q(), &r)
	known, err := t.stringArithmeticBound(e, lower, &r)
	if !known {
		return nil, known, err
	}
	return t.testBig(&r), known, err
}
