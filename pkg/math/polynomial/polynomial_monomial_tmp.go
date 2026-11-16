package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

const monomialHeaderBytes uint64 = 20
const monomialTempInitialCapacity uint32 = 128

func monomialObjectSize(n uint32) uint32         { return uint32(20) + n*8 }
func monomialRefIncrement(n uint32) uint32       { return n + 1 }
func monomialExponentProduct(a, b uint32) uint32 { return a * b }

// Scratch owns only scalar powers. The hash is the project's cached interning
// hash, not a container header or a reference-counted monomial object.
type monomialTemp struct {
	powers         []MonomialPower
	size, capacity uint32
	hash           uint32
}

func (m *MonomialManager) tempSize(t *monomialTemp) (uint32, error) {
	if e := m.tempConstruct(t); e != nil {
		return 0, e
	}
	return t.size, nil
}
func (m *MonomialManager) tempConstruct(t *monomialTemp) error {
	if t == nil {
		return arena.ErrBounds
	}
	if t.powers != nil {
		return nil
	}
	t.powers = make([]MonomialPower, int(monomialTempInitialCapacity))
	t.capacity = monomialTempInitialCapacity
	return nil
}
func (m *MonomialManager) tempGrow(t *monomialTemp, n uint32, preserve bool, line int32) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(line, "new_capacity > m_capacity", n > t.capacity); e != nil {
		return e
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	var next []MonomialPower
	if preserve {
		next = make([]MonomialPower, int(n))
		if uint64(t.size) > uint64(len(t.powers)) || t.size > n {
			return arena.ErrBounds
		}
		copy(next, t.powers[:t.size])
	} else {
		// increase_capacity destroys the old temporary before allocating its
		// replacement and the replacement constructor starts with size zero.
		t.powers = nil
		t.size = 0
		next = make([]MonomialPower, int(n))
	}
	t.powers = next
	t.capacity = n
	t.hash = 0
	return nil
}
func (m *MonomialManager) tempSetSize(t *monomialTemp, n uint32) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(582, "sz <= m_capacity", n <= t.capacity); e != nil {
		return e
	}
	t.size = n
	return nil
}
func (m *MonomialManager) tempPower(t *monomialTemp, i uint32) (MonomialPower, error) {
	if e := m.tempConstruct(t); e != nil {
		return MonomialPower{}, e
	}
	if i >= t.capacity {
		return MonomialPower{}, arena.ErrBounds
	}
	return t.powers[i], nil
}
func (m *MonomialManager) tempSetPower(t *monomialTemp, i uint32, p MonomialPower) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(587, "idx < m_capacity", i < t.capacity); e != nil {
		return e
	}
	if i >= t.capacity || uint64(i) >= uint64(len(t.powers)) {
		return arena.ErrBounds
	}
	t.powers[i] = p
	return nil
}
func (m *MonomialManager) tempPushBack(t *monomialTemp, p MonomialPower) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	if t.size >= t.capacity {
		if t.size > ^uint32(0)/2 {
			return arena.ErrBounds
		}
		if e := m.tempGrow(t, t.size*2, true, 528); e != nil {
			return e
		}
	}
	if uint64(t.size) >= uint64(len(t.powers)) {
		return arena.ErrBounds
	}
	t.powers[t.size] = p
	t.size++
	return nil
}
func (m *MonomialManager) tempReset(t *monomialTemp) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	t.size = 0
	return nil
}
func (m *MonomialManager) tempReserve(t *monomialTemp, n uint32) error {
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	if n <= t.capacity {
		return nil
	}
	if n > ^uint32(0)/2 {
		return arena.ErrBounds
	}
	return m.tempGrow(t, n*2, false, 521)
}
func (m *MonomialManager) tempPowers(t *monomialTemp) ([]MonomialPower, error) {
	if e := m.tempConstruct(t); e != nil {
		return nil, e
	}
	if uint64(t.size) > uint64(len(t.powers)) {
		return nil, arena.ErrBounds
	}
	return t.powers[:t.size], nil
}
func (m *MonomialManager) tempInit(t *monomialTemp, p []MonomialPower) error {
	if uint64(len(p)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	if e := m.tempConstruct(t); e != nil {
		return e
	}
	n := uint32(len(p))
	if n > t.capacity {
		if n > ^uint32(0)/2 {
			return arena.ErrBounds
		}
		if e := m.tempGrow(t, n*2, false, 521); e != nil {
			return e
		}
	}
	if e := m.sourceScalarAssert(549, "sz < m_capacity", n < t.capacity); e != nil {
		return e
	}
	t.size = n
	copy(t.powers[:n], p)
	return nil
}

func (m *MonomialManager) tempHash(t *monomialTemp) (uint32, error) {
	if e := m.tempConstruct(t); e != nil {
		return 0, e
	}
	if uint64(t.size) > uint64(len(t.powers)) {
		return 0, arena.ErrBounds
	}
	t.hash = monomialPowersHash(t.powers, t.size*8, 11)
	if polynomialValidateOwnership {
		if h := uhash.WordReader(t.size*8, 11, func(i uint32) uint32 {
			p := t.powers[i/2]
			if i%2 == 0 {
				return p.Var
			}
			return p.Degree
		}); h != t.hash {
			panic("polynomial: monomialPowersHash differs from hash.WordReader")
		}
	}
	return t.hash, nil
}

// monomialPowerWord is word w of the sequence Var0, Degree0, Var1, ...
func monomialPowerWord(p []MonomialPower, w uint32) uint32 {
	if w%2 == 0 {
		return p[w/2].Var
	}
	return p[w/2].Degree
}

// monomialPowersHash is uhash.WordReader(n, init, word) for word w =
// monomialPowerWord(p, w): the same source string_hash steps over the
// little-endian bytes of those words (signed-char tail included), without
// two closure calls per word. Checked builds compare it with WordReader.
func monomialPowersHash(p []MonomialPower, n, init uint32) uint32 {
	a, b, c := uint32(0x9e3779b9), uint32(0x9e3779b9), init
	i := uint32(0)
	for n-i >= 12 {
		w := i / 4
		a += monomialPowerWord(p, w)
		b += monomialPowerWord(p, w+1)
		c += monomialPowerWord(p, w+2)
		a, b, c = uhash.MixState(a, b, c)
		i += 12
	}
	c += n
	for j := uint32(0); j < n-i; j++ {
		k := i + j
		v := uint32(int32(int8(byte(monomialPowerWord(p, k/4) >> (8 * (k % 4))))))
		switch {
		case j < 4:
			a += v << (j * 8)
		case j < 8:
			b += v << ((j - 4) * 8)
		default:
			c += v << ((j - 8 + 1) * 8)
		}
	}
	return uhash.Mix(a, b, c)
}
func (m *MonomialManager) tempCleanup(t *monomialTemp) error { *t = monomialTemp{}; return nil }
