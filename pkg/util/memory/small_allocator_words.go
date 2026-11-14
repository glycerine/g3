package memory

// Native word payloads beside small-allocator objects.
//
// z3's mpz cells keep their digits in a plain malloc'd array. A caller that
// stores such an array in an allocator object can attach a Go []uint32 to the
// object instead of routing every word through the byte runtime. The object's
// allocation, accounting, failure points and release are unchanged: the
// payload is extra Go memory indexed by the object's location slot, outside
// the emulated byte runtime and its initialized-byte bitmap. A payload
// belongs to the object generation that attached it: Words refuses a later
// occupant of the same location slot with ErrRuntimeUninitialized, the error
// the byte runtime gave for a digit that was never written, so a backing that
// names an object without a digit array stops instead of reading the
// previous occupant's words (math/polynomial's convert-bounds read-stop tests
// rely on this). A payload is reset when its slot is attached again.
// Payloads survive Close, as a caller-owned direct object's storage survives
// the source allocator's destructor; only WordsUnchecked can reach them then.

import "github.com/glycerine/g3/pkg/arena"

// AttachWords gives the live object r a zeroed native payload of n words,
// reusing the slot's previous payload storage when it is large enough.
// It neither reserves budget nor touches the byte runtime.
func (a *SmallAllocator) AttachWords(r ObjectRef, n uint64) error {
	if e := a.open(); e != nil {
		return e
	}
	if _, e := a.object(r); e != nil {
		return e
	}
	if n > uint64(^uint(0)>>3) {
		return arena.ErrBounds
	}
	for uint64(len(a.words)) < uint64(r.slot) {
		a.words = append(a.words, nil)
		a.wordsGeneration = append(a.wordsGeneration, 0)
	}
	w := a.words[r.slot-1]
	if uint64(cap(w)) < n {
		w = make([]uint32, n)
	} else {
		w = w[:n]
		clear(w)
	}
	a.words[r.slot-1] = w
	a.wordsGeneration[r.slot-1] = r.generation
	return nil
}

// Words returns the native payload of the live object r after validating its
// owner and generation. An object that never had AttachWords for its
// generation has no digit array: that is ErrRuntimeUninitialized. The slice
// aliases the payload; it is valid until the object is released.
func (a *SmallAllocator) Words(r ObjectRef) ([]uint32, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	if _, e := a.object(r); e != nil {
		return nil, e
	}
	if uint64(r.slot) > uint64(len(a.words)) || a.wordsGeneration[r.slot-1] != r.generation {
		return nil, ErrRuntimeUninitialized
	}
	return a.words[r.slot-1], nil
}

// WordsUnchecked returns the payload stored for r's slot without validating
// the allocator, the object's owner, its generation, or that the payload was
// attached for r at all. Use on a stale, reused or payload-less object is a
// caller bug (see numericValidateCells); it reads the slot's last payload.
func (a *SmallAllocator) WordsUnchecked(r ObjectRef) []uint32 {
	return a.words[r.slot-1]
}
