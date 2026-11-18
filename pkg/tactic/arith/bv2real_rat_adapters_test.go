package arith

// Test-only adapters: the bv2real tests state their inputs and expectations
// as *big.Rat oracles; production code holds numeral rationals.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// rational imports a test value into a project owner of this call.
func (c *bv2RealCall) rational(q *big.Rat) *unumeral.OwnedRational {
	v := c.number()
	if v != nil {
		c.err = c.u.manager.NumeralService().SetString(v, q.RatString())
	}
	if c.err != nil {
		return nil
	}
	return v
}

func (c *bv2RealCall) rationalValue(v *unumeral.OwnedRational) *big.Rat {
	if c.err != nil {
		return nil
	}
	text, err := c.u.manager.NumeralService().String(v)
	if err != nil {
		c.err = err
		return nil
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		c.err = ast.ErrASTDomain
	}
	return q
}

func (c *bv2RealCall) signedNumeral(q *big.Rat) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	value := c.rational(q)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.signedNumeralOwned(value)
}

func (c *bv2RealCall) encoding(a, b ast.ASTHandle, d, r *big.Rat) ast.ASTHandle {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	divisor, root := c.rational(d), c.rational(r)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.encodingOwned(a, b, divisor, root)
}

func (c *bv2RealCall) reduced(a, b ast.ASTHandle, d, r *big.Rat) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	divisor, root := c.rational(d), c.rational(r)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.reducedOwned(a, b, divisor, root)
}

func (c *bv2RealCall) divisible(h ast.ASTHandle, overflow *big.Rat) (ast.ASTHandle, bool) {
	if c.err != nil {
		return h, false
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	input := c.rational(overflow)
	if c.err != nil {
		return h, false
	}
	return c.divisibleOwned(h, input)
}

func (c *bv2RealCall) bvNumeral(h ast.ASTHandle) (*big.Rat, bool) {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	q := c.number()
	if q == nil {
		return nil, false
	}
	var width uint32
	yes, err := c.u.manager.BitVectorNumeralValue(h, q, &width)
	if err != nil {
		c.err = err
		return nil, false
	}
	if !yes {
		return nil, false
	}
	return c.rationalValue(q), c.err == nil
}

// numeralRat is mk_numeral(q, width) of a test value.
func (c *bv2RealCall) numeralRat(q *big.Rat, width uint32) ast.ASTHandle {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	n := c.rational(q)
	if n == nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.u.manager.BitVectorNumeral(n, width))
}

func (c *bv2RealCall) alignDivisorRat(a, b ast.ASTHandle, d *big.Rat) (ast.ASTHandle, ast.ASTHandle, bool) {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	value := c.rational(d)
	if c.err != nil {
		return a, b, false
	}
	a, b, ok := c.alignDivisor(a, b, value)
	if ok {
		if out := c.rationalValue(value); out != nil {
			d.Set(out)
		}
	}
	return a, b, ok
}

func (c *bv2RealCall) alignDivisorsRat(a, b, x, y ast.ASTHandle, d1, d2 *big.Rat) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	v1, v2 := c.rational(d1), c.rational(d2)
	if c.err != nil {
		return a, b, x, y
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, v1, v2)
	if out := c.rationalValue(v1); out != nil {
		d1.Set(out)
	}
	if out := c.rationalValue(v2); out != nil {
		d2.Set(out)
	}
	return a, b, x, y
}

func bv2RealOwned(u *BV2RealUtil, q *big.Rat) (unumeral.OwnedRational, error) {
	s := u.manager.NumeralService()
	v, err := s.NewValue()
	if err != nil {
		return v, err
	}
	if q != nil {
		if err = s.SetString(&v, q.RatString()); err != nil {
			return v, errors.Join(err, s.Del(&v))
		}
	}
	return v, nil
}

func bv2RealRat(u *BV2RealUtil, v *unumeral.OwnedRational) (*big.Rat, error) {
	text, err := u.manager.NumeralService().String(v)
	if err != nil {
		return nil, err
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return q, nil
}

func bv2RealWithPair(u *BV2RealUtil, d, r *big.Rat, body func(d, r *unumeral.OwnedRational) error) (err error) {
	if d == nil || r == nil {
		return body(nil, nil)
	}
	s := u.manager.NumeralService()
	dv, err := bv2RealOwned(u, d)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Del(&dv)) }()
	rv, err := bv2RealOwned(u, r)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Del(&rv)) }()
	if err = body(&dv, &rv); err != nil {
		return err
	}
	dq, e := bv2RealRat(u, &dv)
	if e != nil {
		return e
	}
	rq, e := bv2RealRat(u, &rv)
	if e != nil {
		return e
	}
	d.Set(dq)
	r.Set(rq)
	return nil
}

func mkBV2RealCRat(u *BV2RealUtil, a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) error {
	return bv2RealWithPair(u, d, r, func(dv, rv *unumeral.OwnedRational) error { return u.MkBV2RealC(a, b, dv, rv, out) })
}

func mkBV2RealRat(u *BV2RealUtil, a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) (ok bool, err error) {
	err = bv2RealWithPair(u, d, r, func(dv, rv *unumeral.OwnedRational) error {
		var e error
		ok, e = u.MkBV2Real(a, b, dv, rv, out)
		return e
	})
	return ok, err
}

func mkReducedRat(u *BV2RealUtil, a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) error {
	return bv2RealWithPair(u, d, r, func(dv, rv *unumeral.OwnedRational) error { return u.MkReduced(a, b, dv, rv, out) })
}

func matchRat(u *BV2RealUtil, h ast.ASTHandle, aout, bout ast.ASTRef, d, r *big.Rat) (found bool, err error) {
	err = bv2RealWithPair(u, d, r, func(dv, rv *unumeral.OwnedRational) error {
		var e error
		found, e = u.Match(h, aout, bout, dv, rv)
		return e
	})
	return found, err
}

func defaultsRat(u *BV2RealUtil) (root, divisor, maxDivisor big.Rat, err error) {
	s := u.manager.NumeralService()
	var vs [3]unumeral.OwnedRational
	for i := range vs {
		if vs[i], err = s.NewValue(); err != nil {
			return
		}
		defer func(v *unumeral.OwnedRational) { err = errors.Join(err, s.Del(v)) }(&vs[i])
	}
	if err = u.Defaults(&vs[0], &vs[1], &vs[2]); err != nil {
		return
	}
	for i, out := range []*big.Rat{&root, &divisor, &maxDivisor} {
		q, e := bv2RealRat(u, &vs[i])
		if e != nil {
			err = e
			return
		}
		out.Set(q)
	}
	return
}

// boundRat reads a BoundManager bound as a *big.Rat test oracle value.
func boundRat(b *BoundManager, term ast.ASTHandle, lower bool) (value big.Rat, strict, found bool, err error) {
	q := b.q()
	r := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&r)) }()
	if lower {
		strict, found, err = b.Lower(term, &r)
	} else {
		strict, found, err = b.Upper(term, &r)
	}
	if err != nil || !found {
		return value, strict, found, err
	}
	err = q.BigRatOf(&r, &value)
	return value, strict, found, err
}

// ratOf converts a rational of m's NumeralService().Manager to a test
// oracle value.
func ratOf(m *ast.ASTManager, r *unumeral.Rational) *big.Rat {
	out := new(big.Rat)
	if err := m.NumeralService().Manager.BigRatOf(r, out); err != nil {
		panic(err)
	}
	return out
}

// smallRational is the small rational n/d (no storage, no destruction).
func smallRational(n, d int32) unumeral.Rational {
	return unumeral.Rational{Num: unumeral.NewInt(n), Den: unumeral.NewInt(d)}
}

// ratText is rational::to_string of a rational of m's NumeralService().Manager.
func ratText(m *ast.ASTManager, r *unumeral.Rational) string {
	text, err := m.NumeralService().Manager.String(r)
	if err != nil {
		panic(err)
	}
	return text
}

// degreeShiftCandidatesHeld releases the candidate degrees at cleanup.
func degreeShiftCandidatesHeld(t testing.TB, m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]*unumeral.Rational, error) {
	got, err := degreeShiftCandidates(m, roots)
	t.Cleanup(func() {
		if e := degreeShiftReleaseDegrees(m, got); e != nil {
			t.Error(e)
		}
	})
	return got, err
}
