package numeral

// Source: src/util/mpfx.cpp:251-315,704-743,758-787,842-854.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

// SetFromInteger is the source set_core(mpfx&, mpz_manager&, mpz const&):
// the integer's digits become the integer-part words.
func (m *FixedPointManager) SetFromInteger(dst *FixedPoint, integers *IntegerManager, src *Int) error {
	if err := m.check(dst); err != nil {
		return err
	}
	if integers == nil {
		return ErrFixedPointOwner
	}
	return m.setFromInteger(dst, integers, src)
}

func (m *FixedPointManager) setFromInteger(n *FixedPoint, z *IntegerManager, v *Int) error {
	zero, err := z.IsZero(v)
	if err != nil {
		return err
	}
	if zero {
		m.reset(n)
		return nil
	}
	m.allocateIfNeeded(n)
	negative, digits, err := z.Decompose(v, m.tmpDigits)
	m.tmpDigits = digits
	if err != nil {
		return err
	}
	n.sign = negative
	if uint32(len(digits)) > m.intWords {
		return ErrFixedPointOverflow
	}
	w := n.words
	clear(w[:m.fracWords])
	intPart := w[m.fracWords:]
	copy(intPart, digits)
	clear(intPart[len(digits):])
	return nil
}

// SetFromRational is the source set_core(mpfx&, mpq_manager&, mpq const&).
// For a noninteger input, mpfx.cpp:287 assigns the destination's existing
// sign (n.m_sign = is_neg(n)) to the result before rounding, and the
// numerator's sign is then removed.
func (m *FixedPointManager) SetFromRational(dst *FixedPoint, rationals *RationalManager, src *Rational) (err error) {
	if err := m.check(dst); err != nil {
		return err
	}
	if rationals == nil {
		return ErrFixedPointOwner
	}
	isInt, err := rationals.IsInt(src)
	if err != nil {
		return err
	}
	z := rationals.Z
	if isInt {
		return m.setFromInteger(dst, z, &src.Num)
	}
	n := dst
	m.allocateIfNeeded(n)
	tmp := NewInt(0)
	defer func() {
		if e := z.Del(&tmp); err == nil {
			err = e
		}
	}()
	n.sign = n.isNeg()
	if err = z.Mul2k(&src.Num, 32*m.fracWords, &tmp); err != nil {
		return err
	}
	if err = z.Abs(&tmp); err != nil {
		return err
	}
	roundAway := false
	if n.sign != m.roundUp {
		divides, e := z.Divides(&src.Den, &tmp)
		if e != nil {
			return e
		}
		roundAway = !divides
	}
	if err = z.Div(&tmp, &src.Den, &tmp); err != nil {
		return err
	}
	if roundAway {
		if err = z.Inc(&tmp); err != nil {
			return err
		}
	}
	_, digits, err := z.Decompose(&tmp, m.tmpDigits)
	m.tmpDigits = digits
	if err != nil {
		return err
	}
	if uint32(len(digits)) > m.totalWords {
		return ErrFixedPointOverflow
	}
	copy(n.words, digits)
	clear(n.words[len(digits):])
	return nil
}

// ToInteger is the source to_mpz_core, reached for integer values; a value
// with fractional words reports a domain error instead of truncating.
func (m *FixedPointManager) ToInteger(src *FixedPoint, integers *IntegerManager, dst *Int) error {
	if err := m.check(src); err != nil {
		return err
	}
	if integers == nil {
		return ErrFixedPointOwner
	}
	if !m.isInt(src) {
		return ErrFixedPointDomain
	}
	if err := integers.SetLimbs(dst, m.view(src)[m.fracWords:]); err != nil {
		return err
	}
	if src.isNeg() {
		return integers.Neg(dst)
	}
	return nil
}

// ToRational is the source to_mpq_core: the words over 2^(32*frac).
func (m *FixedPointManager) ToRational(src *FixedPoint, rationals *RationalManager, dst *Rational) (err error) {
	if err := m.check(src); err != nil {
		return err
	}
	if rationals == nil {
		return ErrFixedPointOwner
	}
	z := rationals.Z
	a, b := NewInt(0), NewInt(1)
	defer func() {
		err = JoinNumericErrors(err, z.Del(&a))
		err = JoinNumericErrors(err, z.Del(&b))
	}()
	if err = z.SetLimbs(&a, m.view(src)); err != nil {
		return err
	}
	if err = z.Mul2k(&b, 32*m.fracWords, &b); err != nil {
		return err
	}
	if err = rationals.RatDiv(&a, &b, dst); err != nil {
		return err
	}
	if src.isNeg() {
		return rationals.Neg(dst)
	}
	return nil
}

// String is the source to_string, which is display.
func (m *FixedPointManager) String(a *FixedPoint) (string, error) {
	if err := m.check(a); err != nil {
		return "", err
	}
	var b strings.Builder
	if err := m.display(&b, a); err != nil {
		return "", err
	}
	return b.String(), nil
}

func (m *FixedPointManager) ToRationalString(a *FixedPoint) (string, error) {
	return m.String(a)
}

func (m *FixedPointManager) Display(w io.Writer, a *FixedPoint) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := m.check(a); err != nil {
		return err
	}
	return m.display(w, a)
}

// display is the source display: integers print their integer words; other
// values print the words without trailing zero bits over a power of two.
// The source shifts the numeral's words in place and back; g3 shifts a copy.
func (m *FixedPointManager) display(w io.Writer, n *FixedPoint) error {
	if n.isNeg() {
		if err := writeFixedPointPart(w, "-"); err != nil {
			return err
		}
	}
	ws := m.view(n)
	if m.isInt(n) {
		text, err := m.mpn.String(ws[m.fracWords:])
		if err != nil {
			return err
		}
		return writeFixedPointPart(w, text)
	}
	shift := fixedWordsNTZ(ws)
	shifted := ws
	if shift > 0 {
		shifted = make([]uint32, len(ws))
		fixedWordsShr(ws, shift, shifted)
	}
	text, err := m.mpn.String(shifted)
	if err != nil {
		return err
	}
	if err := writeFixedPointPart(w, text); err != nil {
		return err
	}
	// display denominator as a power of 2
	k := 32*m.fracWords - shift
	if err := writeFixedPointPart(w, "/2"); err != nil {
		return err
	}
	if k > 1 {
		if err := writeFixedPointPart(w, "^"); err != nil {
			return err
		}
		return writeFixedPointPart(w, strconv.FormatUint(uint64(k), 10))
	}
	return nil
}

func writeFixedPointPart(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}
