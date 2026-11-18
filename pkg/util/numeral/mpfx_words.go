package numeral

// Source: src/util/bit_util.cpp:81-151,242-283,343-400 (nlz, ntz, is_zero,
// shr, has_one_at_first_k_bits, inc, lt, add) on mpfx's fixed-size words.

import "math/bits"

// fixedWordsZero is ::is_zero(sz, data).
func fixedWordsZero(ws []uint32) bool {
	for _, w := range ws {
		if w != 0 {
			return false
		}
	}
	return true
}

// fixedWordsInc is ::inc: false reports the wrap of every word.
func fixedWordsInc(ws []uint32) bool {
	for i := range ws {
		ws[i]++
		if ws[i] != 0 {
			return true
		}
	}
	return false
}

// fixedWordsLess is ::lt on two equal-size word arrays.
func fixedWordsLess(a, b []uint32) bool {
	for i := len(a); i > 0; {
		i--
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return false
}

// fixedWordsAdd is ::add: c <- a + b over len(c) words; false reports a
// carry out, after all words of c are written.
func fixedWordsAdd(a, b, c []uint32) bool {
	var k uint32
	for j := range c {
		r := a[j] + b[j]
		c1 := r < a[j]
		c[j] = r + k
		c2 := c[j] < r
		k = 0
		if c1 || c2 {
			k = 1
		}
	}
	return k == 0
}

// fixedWordsNLZ is ::nlz.
func fixedWordsNLZ(ws []uint32) uint32 {
	var r uint32
	for i := len(ws); i > 0; {
		i--
		if ws[i] == 0 {
			r += 32
		} else {
			return r + uint32(bits.LeadingZeros32(ws[i]))
		}
	}
	return r
}

// fixedWordsNTZ is ::ntz.
func fixedWordsNTZ(ws []uint32) uint32 {
	var r uint32
	for _, w := range ws {
		if w == 0 {
			r += 32
		} else {
			return r + uint32(bits.TrailingZeros32(w))
		}
	}
	return r
}

// fixedWordsShr is the same-size ::shr(sz, src, k, dst).
func fixedWordsShr(src []uint32, k uint32, dst []uint32) {
	sz := uint32(len(src))
	digitShift := k / 32
	if digitShift >= sz {
		clear(dst[:sz])
		return
	}
	bitShift := k % 32
	compShift := 32 - bitShift
	newSize := sz - digitShift
	i, j := uint32(0), digitShift
	if bitShift != 0 {
		for ; i < newSize-1; i, j = i+1, j+1 {
			dst[i] = src[j]>>bitShift | src[j+1]<<compShift
		}
		dst[i] = src[j] >> bitShift
	} else {
		for ; i < newSize; i, j = i+1, j+1 {
			dst[i] = src[j]
		}
	}
	for i := newSize; i < sz; i++ {
		dst[i] = 0
	}
}
