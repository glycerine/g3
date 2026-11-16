package numeral

// Source: src/util/mpz.h::mul, src/util/mpz.cpp::big_mul,
// src/util/mpn.cpp::mul

import (
	"math"
	"math/bits"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Mul translates the source small-int path and big multiplication. Exact
// destination/input aliasing is supported through the manager's result scratch.
func (m *IntegerManager) Mul(a, b, c *Int) error {
	return m.mulWithAllocationContext(a, b, c, numeralContextRef(integerAllocationInvocationContext{}))
}
func (m *IntegerManager) mulWithAllocationContext(a, b, c *Int, context *integerAllocationInvocationContext) (err error) {
	return m.mulPrechecked(a, b, c, context, 0)
}

// mulPrechecked is mulWithAllocationContext for a caller that has already
// run check on the operands named by pre (bit i: operand i of a, b, c); see
// skipPrechecked.
func (m *IntegerManager) mulPrechecked(a, b, c *Int, context *integerAllocationInvocationContext, pre uint8) (err error) {
	if smallIntegerOperands(a, b, c) {
		if v := int64(a.Val) * int64(b.Val); v >= math.MinInt32 && v <= math.MaxInt32 {
			if e := m.checkReady(); e != nil {
				return e
			}
			c.Val = int32(v)
			return nil
		}
	}
	if e := m.checkOperands(a, b, c, pre); e != nil {
		return e
	}
	m.traceMPZBinary(a, b, c, "*", true)
	// The after-record is only emitted by trace builds; others skip the
	// deferred closure (traceMPZ* is a no-op there).
	if diagnostic.TraceCompiled() {
		defer func() {
			if err == nil {
				m.traceMPZBinary(a, b, c, "*", false)
			}
		}()
	}
	if a.IsSmall() && b.IsSmall() {
		return m.setInt64WithAllocationContext(c, int64(a.Val)*int64(b.Val), context)
	}
	sa, ca, e := m.signCell(a, 0)
	if e != nil {
		return e
	}
	sb, cb, e := m.signCell(b, 1)
	if e != nil {
		return e
	}
	size := uint64(m.Cell(ca).Size) + uint64(m.Cell(cb).Size)
	if size > math.MaxUint32 {
		return arena.ErrBounds
	}
	if e := m.ensureTmpWithAllocationContext(0, uint32(size), context); e != nil {
		return e
	}
	if e := m.prepareTmpSpan(0, uint32(size)); e != nil {
		return e
	}
	if SourceExtendedGCDDebug { // the mpn views are built only for the debug trace
		m.Natural.traceMPNBinaryView(
			limbView{allocator: m.Allocator, backing: m.Cell(ca).Backing, size: m.Cell(ca).Size},
			limbView{allocator: m.Allocator, backing: m.Cell(cb).Backing, size: m.Cell(cb).Size}, "*")
	}
	m.multiplyCells(ca, cb, m.Tmp[0])
	if SourceExtendedGCDDebug { // the mpn views are built only for the debug trace
		m.Natural.traceMPNUnaryView(limbView{allocator: m.Allocator, backing: m.Cell(m.Tmp[0]).Backing, size: uint32(size)}, 427, true)
	}
	sign := int32(1)
	if sa != sb {
		sign = -1
	}
	return m.publishTmpAtWithAllocationContext(0, c, sign, uint32(size), context)
}

// multiplyCells is indexed Knuth Algorithm M. Destination must not alias an
// input; public Mul stages it in source-equivalent owner scratch.
func (m *IntegerManager) multiplyCells(a, b, c NumericCellRef) {
	na, nb := m.Cell(a).Size, m.Cell(b).Size
	if da, la, ok := m.digitsDirect(a); ok {
		if db, lb, ok := m.digitsDirect(b); ok {
			if dc, lc, ok := m.digitsDirect(c); ok {
				multiplyDigits(da, la, na, db, lb, nb, dc, lc)
				return
			}
		}
	}
	for i := uint32(0); i < na; i++ {
		m.writeDigit(c, i, 0)
	}
	for j := uint32(0); j < nb; j++ {
		v := m.Digit(b, j)
		if v == 0 {
			m.writeDigit(c, j+na, 0)
			continue
		}
		var k uint32
		for i := uint32(0); i < na; i++ {
			t := uint64(m.Digit(a, i))*uint64(v) + uint64(m.Digit(c, i+j)) + uint64(k)
			m.writeDigit(c, i+j, uint32(t))
			k = uint32(t >> 32)
		}
		m.writeDigit(c, j+na, k)
	}
}

// multiplyDigits is multiplyCells over digit payloads read with Digit's
// limits (la, lb, lc): the same limb reads, writes and bounds panics.
func multiplyDigits(da []uint32, la, na uint32, db []uint32, lb, nb uint32, dc []uint32, lc uint32) {
	read := func(d []uint32, limit, i uint32) uint32 {
		if i >= limit {
			return 0
		}
		if uint64(i) >= uint64(len(d)) {
			panic(arena.ErrBounds)
		}
		return d[i]
	}
	if uint64(na)+uint64(nb) > uint64(len(dc)) || na > la || na > uint32(len(da)) || nb > lb || nb > uint32(len(db)) || uint64(na)+uint64(nb) > uint64(lc) {
		// Out-of-range shapes keep the exact per-limb semantics.
		for i := uint32(0); i < na; i++ {
			if uint64(i) >= uint64(len(dc)) {
				panic(arena.ErrBounds)
			}
			dc[i] = 0
		}
		for j := uint32(0); j < nb; j++ {
			v := read(db, lb, j)
			if v == 0 {
				if uint64(j+na) >= uint64(len(dc)) {
					panic(arena.ErrBounds)
				}
				dc[j+na] = 0
				continue
			}
			var k uint32
			for i := uint32(0); i < na; i++ {
				t := uint64(read(da, la, i))*uint64(v) + uint64(read(dc, lc, i+j)) + uint64(k)
				if uint64(i+j) >= uint64(len(dc)) {
					panic(arena.ErrBounds)
				}
				dc[i+j] = uint32(t)
				k = uint32(t >> 32)
			}
			if uint64(j+na) >= uint64(len(dc)) {
				panic(arena.ErrBounds)
			}
			dc[j+na] = k
		}
		return
	}
	a, b, c := da[:na], db[:nb], dc[:na+nb]
	if na >= mulWideMinDigits && nb >= mulWideMinDigits {
		multiplyDigitsWide(a, b, c)
		return
	}
	for i := range a {
		c[i] = 0
	}
	for j, v := range b {
		if v == 0 {
			c[j+len(a)] = 0
			continue
		}
		var k uint32
		cj := c[j : j+len(a)]
		for i, u := range a {
			t := uint64(u)*uint64(v) + uint64(cj[i]) + uint64(k)
			cj[i] = uint32(t)
			k = uint32(t >> 32)
		}
		c[j+len(a)] = k
	}
}

// mulWideMinDigits is the smallest operand length (in 32-bit digits) for
// which multiplyDigitsWide's packing pays for itself.
const mulWideMinDigits = 4

// multiplyDigitsWide stores a*b into c (len(c) == len(a)+len(b), c aliasing
// neither input). It is the schoolbook product of multiplyDigits computed
// on 64-bit limbs, two source digits at a time: the product, and so every
// digit written to c, is the same integer.
func multiplyDigitsWide(a, b, c []uint32) {
	la, lb := (len(a)+1)/2, (len(b)+1)/2
	// Small operands use small stack buffers: Go zeroes a whole buffer
	// array at its declaration, so the 32/32/64-limb buffers cost more to
	// clear than a few-limb product does to compute.
	if la <= 4 && lb <= 4 {
		var bufA, bufB [4]uint64
		var bufC [8]uint64
		multiplyWideLimbs(bufA[:la], bufB[:lb], bufC[:la+lb], a, b, c)
		return
	}
	if la <= 32 && lb <= 32 {
		var bufA, bufB [32]uint64
		var bufC [64]uint64
		multiplyWideLimbs(bufA[:la], bufB[:lb], bufC[:la+lb], a, b, c)
		return
	}
	multiplyWideLimbs(make([]uint64, la), make([]uint64, lb), make([]uint64, la+lb), a, b, c)
}

// multiplyWideLimbs is multiplyDigitsWide's product with x, y and z the
// operand and (zeroed) result limb buffers.
func multiplyWideLimbs(x, y, z []uint64, a, b, c []uint32) {
	la := len(x)
	packDigitsWide(x, a)
	packDigitsWide(y, b)
	for j, v := range y {
		if v == 0 {
			continue // z[j+la] is still zero
		}
		var k uint64
		zj := z[j : j+la]
		for i, u := range x {
			hi, lo := bits.Mul64(u, v)
			var carry uint64
			lo, carry = bits.Add64(lo, zj[i], 0)
			hi += carry
			lo, carry = bits.Add64(lo, k, 0)
			hi += carry
			zj[i] = lo
			k = hi
		}
		z[j+la] = k
	}
	for i := range c {
		c[i] = uint32(z[i>>1] >> (32 * uint(i&1)))
	}
}
func packDigitsWide(dst []uint64, src []uint32) {
	for i := range dst {
		v := uint64(src[2*i])
		if 2*i+1 < len(src) {
			v |= uint64(src[2*i+1]) << 32
		}
		dst[i] = v
	}
}
