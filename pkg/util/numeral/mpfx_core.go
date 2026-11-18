package numeral

// Source: src/util/mpfx.h and src/util/mpfx.cpp:28-83,90-100,133-249,
// 316-520. Concrete fixed-point numeral reached by subpaving's mpfx context.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type fixedPointException struct {
	message string
	parent  error
}

func (e *fixedPointException) Error() string                    { return e.message }
func (e *fixedPointException) Message() string                  { return e.message }
func (e *fixedPointException) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e *fixedPointException) HasErrorCode() bool               { return false }
func (e *fixedPointException) Unwrap() error                    { return e.parent }

var (
	ErrFixedPointOwner          = errors.New("mpfx: invalid numeral owner")
	ErrFixedPointClosed         = errors.New("mpfx: manager or numeral closed")
	ErrFixedPointDomain   error = &fixedPointException{message: "multi-precision fixed point (mpfx) exception"}
	ErrFixedPointOverflow error = &fixedPointException{message: "multi-precision fixed point (mpfx) overflow", parent: ErrFixedPointDomain}
	ErrFixedPointDivZero  error = &fixedPointException{message: "multi-precision fixed point (mpfx) division by zero", parent: ErrFixedPointDomain}
)

// FixedPointManager is the source mpfx_manager: every numeral has the same
// number of 32-bit integer and fractional words, least significant first.
// Rounding initially points toward minus infinity.
type FixedPointManager struct {
	self                *FixedPointManager
	intWords, fracWords uint32
	totalWords          uint32
	roundUp             bool
	// buffer0..2 are the source's 2*m_total_sz scratch vectors.
	buffer0, buffer1, buffer2 []uint32
	tmpDigits                 []uint32
	zeroWords                 []uint32
	one                       FixedPoint
	mpn                       *NaturalManager
	trace                     *diagnostic.TraceTags
	live                      uint32
	closed                    bool
}

// FixedPoint is an owned concrete mpfx numeral. words is nil while the
// numeral holds the source's unallocated zero slot (m_sig_idx == 0);
// otherwise it holds m_total_sz words. An allocated slot whose words are all
// zero is not the source's zero: is_zero tests only the slot.
type FixedPoint struct {
	self    *FixedPoint
	manager *FixedPointManager
	sign    bool
	words   []uint32
	closed  bool
}

func NewFixedPointManager(intWords, fracWords uint32) (*FixedPointManager, error) {
	if intWords == 0 || fracWords == 0 || uint64(intWords)+uint64(fracWords) > 1<<20 {
		return nil, ErrFixedPointOwner
	}
	budget := arena.NewBudget(^uint64(0))
	mpn, err := NewNaturalManager(arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	if err != nil {
		return nil, err
	}
	total := intWords + fracWords
	m := &FixedPointManager{intWords: intWords, fracWords: fracWords, totalWords: total, mpn: mpn,
		buffer0: make([]uint32, 2*total), buffer1: make([]uint32, 2*total), buffer2: make([]uint32, 2*total), zeroWords: make([]uint32, total)}
	m.self = m
	m.one = FixedPoint{manager: m}
	m.one.self = &m.one
	m.setUint32(&m.one, 1)
	return m, nil
}

func (m *FixedPointManager) check(v *FixedPoint) error {
	if m == nil || m.self != m || v == nil || v.self != v || v.manager != m {
		return ErrFixedPointOwner
	}
	if m.closed || v.closed {
		return ErrFixedPointClosed
	}
	return nil
}

func (m *FixedPointManager) pair(a, b *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	return m.check(b)
}

func (m *FixedPointManager) New() (*FixedPoint, error) {
	if m == nil || m.self != m {
		return nil, ErrFixedPointOwner
	}
	if m.closed {
		return nil, ErrFixedPointClosed
	}
	v := &FixedPoint{manager: m}
	v.self = v
	m.live++
	return v, nil
}

func (v *FixedPoint) Close() error {
	if v == nil || v.self != v || v.manager == nil || v.manager.self != v.manager {
		return ErrFixedPointOwner
	}
	if v.closed {
		return nil
	}
	v.manager.reset(v)
	v.closed = true
	v.manager.live--
	return nil
}

func (m *FixedPointManager) Close() error {
	if m == nil || m.self != m {
		return ErrFixedPointOwner
	}
	if m.live != 0 {
		return ErrFixedPointOwner
	}
	if !m.closed {
		m.reset(&m.one)
		m.mpn.Close()
	}
	m.closed = true
	return nil
}

func (m *FixedPointManager) SetRounding(up bool) error {
	if m == nil || m.self != m {
		return ErrFixedPointOwner
	}
	if m.closed {
		return ErrFixedPointClosed
	}
	m.roundUp = up
	return nil
}

// isZero is the source is_zero: the numeral holds the zero slot.
func (n *FixedPoint) isZero() bool { return n.words == nil }

// isPos and isNeg are the source is_pos and is_neg.
func (n *FixedPoint) isPos() bool { return !n.sign && n.words != nil }
func (n *FixedPoint) isNeg() bool { return n.sign }

// view is words(n): the zero slot reads as m_total_sz zero words.
func (m *FixedPointManager) view(n *FixedPoint) []uint32 {
	if n.words == nil {
		return m.zeroWords
	}
	return n.words
}

func (m *FixedPointManager) allocateIfNeeded(n *FixedPoint) {
	if n.words == nil {
		n.words = make([]uint32, m.totalWords)
	}
}

func (m *FixedPointManager) reset(n *FixedPoint) {
	n.words = nil
	n.sign = false
}

// sz is the source sz(ws): the number of words up to the highest nonzero
// one. The source requires a nonzero argument; zero words would read below
// the array (UB), so g3 returns 0 for them and callers treat that case.
func fixedWordsSize(ws []uint32) uint32 {
	r := uint32(len(ws))
	for r > 0 {
		r--
		if ws[r] != 0 {
			return r + 1
		}
	}
	return 0
}

func (m *FixedPointManager) Reset(dst *FixedPoint) error {
	if err := m.check(dst); err != nil {
		return err
	}
	m.reset(dst)
	return nil
}

func (m *FixedPointManager) setUint32(n *FixedPoint, v uint32) {
	if v == 0 {
		m.reset(n)
		return
	}
	m.allocateIfNeeded(n)
	n.sign = false
	clear(n.words)
	n.words[m.fracWords] = v
}

// setUint64 is the source set(mpfx&, uint64_t).
func (m *FixedPointManager) setUint64(n *FixedPoint, v uint64) error {
	if m.intWords == 1 && v > 0xffffffff {
		return ErrFixedPointOverflow
	}
	if v == 0 {
		m.reset(n)
		return nil
	}
	m.allocateIfNeeded(n)
	n.sign = false
	clear(n.words)
	n.words[m.fracWords] = uint32(v)
	if m.intWords > 1 {
		n.words[m.fracWords+1] = uint32(v >> 32)
	}
	return nil
}

// setInt64 is the source set(mpfx&, int64_t). The source negates v as a
// signed value, which is UB for INT64_MIN; the unsigned negation gives the
// intended magnitude 2^63.
func (m *FixedPointManager) setInt64(n *FixedPoint, v int64) error {
	if m.intWords == 1 && (v < -0xffffffff || v > 0xffffffff) {
		return ErrFixedPointOverflow
	}
	if v == 0 {
		m.reset(n)
		return nil
	}
	if v < 0 {
		if err := m.setUint64(n, -uint64(v)); err != nil {
			return err
		}
		n.sign = true
		return nil
	}
	return m.setUint64(n, uint64(v))
}

func (m *FixedPointManager) SetInt64(dst *FixedPoint, n int64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	return m.setInt64(dst, n)
}

func (m *FixedPointManager) SetUint64(dst *FixedPoint, n uint64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	return m.setUint64(dst, n)
}

// set is the source set(mpfx&, mpfx const&).
func (m *FixedPointManager) set(n, v *FixedPoint) {
	if v.isZero() {
		m.reset(n)
		return
	}
	if n == v {
		return
	}
	m.allocateIfNeeded(n)
	n.sign = v.sign
	copy(n.words, v.words)
}

func (m *FixedPointManager) Copy(dst, src *FixedPoint) error {
	if err := m.pair(dst, src); err != nil {
		return err
	}
	m.set(dst, src)
	return nil
}

// SetRatio is the source set(mpfx&, int64_t num, uint64_t den): both
// operands become mpfx numerals before the division.
func (m *FixedPointManager) SetRatio(dst *FixedPoint, num int64, den uint64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	a := FixedPoint{manager: m}
	a.self = &a
	b := FixedPoint{manager: m}
	b.self = &b
	if err := m.setInt64(&a, num); err != nil {
		return err
	}
	if err := m.setUint64(&b, den); err != nil {
		return err
	}
	if b.isZero() {
		return ErrFixedPointDivZero
	}
	m.traceFixedIntegerDivision(num, den, dst, false)
	if err := m.div(&a, &b, dst); err != nil {
		return err
	}
	m.traceFixedIntegerDivision(num, den, dst, true)
	return nil
}

func (m *FixedPointManager) Compare(a, b *FixedPoint) (int, error) {
	if err := m.pair(a, b); err != nil {
		return 0, err
	}
	if m.eq(a, b) {
		return 0, nil
	}
	if m.lt(a, b) {
		return -1, nil
	}
	return 1, nil
}

// eq is the source eq.
func (m *FixedPointManager) eq(a, b *FixedPoint) bool {
	if a.isZero() && b.isZero() {
		return true
	}
	if a.isZero() || b.isZero() {
		return false
	}
	if a.sign != b.sign {
		return false
	}
	for i := range a.words {
		if a.words[i] != b.words[i] {
			return false
		}
	}
	return true
}

// lt is the source lt without its STRACE.
func (m *FixedPointManager) lt(a, b *FixedPoint) bool {
	if a.isZero() {
		return !b.isZero() && !b.isNeg()
	}
	if b.isZero() {
		return a.isNeg()
	}
	if a.isNeg() {
		return b.isPos() || fixedWordsLess(b.words, a.words)
	}
	return b.isPos() && fixedWordsLess(a.words, b.words)
}

func (m *FixedPointManager) IsZero(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.isZero(), nil
}

// isInt is the source is_int: the fractional words are zero.
func (m *FixedPointManager) isInt(n *FixedPoint) bool {
	if n.words == nil {
		return true
	}
	for _, w := range n.words[:m.fracWords] {
		if w != 0 {
			return false
		}
	}
	return true
}

func (m *FixedPointManager) IsInt(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return m.isInt(a), nil
}

// Neg is the source neg: the zero slot keeps its sign.
func (m *FixedPointManager) Neg(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	if !a.isZero() {
		a.sign = !a.sign
	}
	return nil
}

func (m *FixedPointManager) Abs(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	a.sign = false
	return nil
}

// addSub is the source add_sub without its outer STRACE.
func (m *FixedPointManager) addSub(isSub bool, a, b, c *FixedPoint) error {
	if a.isZero() {
		m.set(c, b)
		if isSub && !c.isZero() {
			c.sign = !c.sign
		}
		return nil
	}
	if b.isZero() {
		m.set(c, a)
		return nil
	}
	m.traceFixedAddSubInner(a, b, c, isSub, true)
	sgnA, sgnB := a.sign, b.sign
	wa, wb := a.words, b.words
	if isSub {
		sgnB = !sgnB
	}
	m.allocateIfNeeded(c)
	wc := c.words
	if sgnA == sgnB {
		c.sign = sgnA
		if !fixedWordsAdd(wa, wb, wc) {
			return ErrFixedPointOverflow
		}
	} else if fixedWordsLess(wa, wb) {
		c.sign = sgnB
		m.mpn.Sub(wb, wa, wc)
	} else {
		c.sign = sgnA
		m.mpn.Sub(wa, wb, wc)
		if fixedWordsZero(wc) {
			m.reset(c)
		}
	}
	m.traceFixedAddSubInner(a, b, c, isSub, false)
	return nil
}

func (m *FixedPointManager) Add(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	return m.add(a, b, out)
}

func (m *FixedPointManager) add(a, b, out *FixedPoint) error {
	m.traceFixedAddSubOuter(a, b, out, false, true)
	if err := m.addSub(false, a, b, out); err != nil {
		return err
	}
	m.traceFixedAddSubOuter(a, b, out, false, false)
	return nil
}

func (m *FixedPointManager) Sub(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	return m.sub(a, b, out)
}

func (m *FixedPointManager) sub(a, b, out *FixedPoint) error {
	m.traceFixedAddSubOuter(a, b, out, true, true)
	if err := m.addSub(true, a, b, out); err != nil {
		return err
	}
	m.traceFixedAddSubOuter(a, b, out, true, false)
	return nil
}

func (m *FixedPointManager) Mul(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	m.traceFixedProduct(a, b, out, false, false)
	if err := m.mul(a, b, out); err != nil {
		return err
	}
	m.traceFixedProduct(a, b, out, false, true)
	return nil
}

// mul is the source mul without its STRACE.
func (m *FixedPointManager) mul(a, b, c *FixedPoint) error {
	if a.isZero() || b.isZero() {
		m.reset(c)
		return nil
	}
	wa, wb := a.words, b.words
	sign := a.sign != b.sign
	m.allocateIfNeeded(c)
	c.sign = sign
	total, frac := m.totalWords, m.fracWords
	r := m.buffer0[:2*total]
	m.mpn.Mul(wa, wb, r)
	// round result
	rr := r[frac:]
	if c.sign != m.roundUp && !fixedWordsZero(r[:frac]) {
		if !fixedWordsInc(rr[:total]) {
			return ErrFixedPointOverflow
		}
	}
	// check for overflows
	if !fixedWordsZero(rr[total : total+m.intWords]) {
		return ErrFixedPointOverflow
	}
	copy(c.words, rr[:total])
	return nil
}

func (m *FixedPointManager) Div(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	if b.isZero() || fixedWordsZero(b.words) {
		return ErrFixedPointDivZero
	}
	m.traceFixedProduct(a, b, out, true, false)
	if err := m.div(a, b, out); err != nil {
		return err
	}
	m.traceFixedProduct(a, b, out, true, true)
	return nil
}

// div is the source div without its STRACE; b is nonzero. An allocated
// all-zero divisor or dividend would make the source's sz read below its
// words (UB): g3 reports division by zero for the divisor and a zero
// quotient for the dividend.
func (m *FixedPointManager) div(a, b, c *FixedPoint) error {
	if b.isZero() || fixedWordsZero(b.words) {
		return ErrFixedPointDivZero
	}
	if a.isZero() || fixedWordsZero(a.words) {
		m.reset(c)
		return nil
	}
	total, frac := m.totalWords, m.fracWords
	sign := a.sign != b.sign
	wa, wb := a.words, b.words
	aShiftSize := fixedWordsSize(wa) + frac
	shifted := m.buffer0[:2*total]
	clear(shifted[:frac])
	copy(shifted[frac:frac+total], wa)
	bSize := fixedWordsSize(wb)
	m.allocateIfNeeded(c)
	c.sign = sign
	if bSize > aShiftSize {
		if c.sign != m.roundUp {
			m.setEpsilon(c)
		} else {
			m.reset(c)
		}
		return nil
	}
	qSize := aShiftSize - bSize + 1
	q := m.buffer1[:2*total]
	rem := m.buffer2[:bSize]
	if _, err := m.mpn.Div(shifted[:aShiftSize], wb[:bSize], q[:qSize], rem); err != nil {
		return err
	}
	for i := total; i < qSize; i++ {
		if q[i] != 0 {
			return ErrFixedPointOverflow
		}
	}
	// FORK FIX #123: z3 4.7.1 increments m_total_sz quotient words but then
	// copies only q_sz of them when q_sz < m_total_sz, so a carry out of the
	// q_sz words was lost in a stale buffer word (the quotient collapsed to
	// epsilon). The fork, like g3, clears those words and keeps the carry.
	if qSize < total {
		clear(q[qSize:total])
	}
	if c.sign != m.roundUp && !fixedWordsZero(rem) {
		// round the result
		if !fixedWordsInc(q[:total]) {
			return ErrFixedPointOverflow
		}
	}
	copy(c.words, q[:total])
	if fixedWordsZero(c.words) {
		if c.sign != m.roundUp {
			m.setEpsilon(c)
		} else {
			m.reset(c)
		}
	}
	return nil
}

// setEpsilon is the source set_epsilon on an allocated numeral.
func (m *FixedPointManager) setEpsilon(n *FixedPoint) {
	clear(n.words)
	n.words[0] = 1
}
