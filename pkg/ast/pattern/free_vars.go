package pattern

import "math/bits"

// Most inferred patterns mention only the innermost few binders. Keep their
// membership inline and allocate sparse storage only for larger indices.
type patternVarSet struct {
	low  uint64
	high map[uint32]struct{}
}

func (s *patternVarSet) insert(v uint32) {
	if v < 64 {
		s.low |= uint64(1) << v
		return
	}
	if s.high == nil {
		s.high = make(map[uint32]struct{})
	}
	s.high[v] = struct{}{}
}

func (s *patternVarSet) include(other patternVarSet) {
	s.low |= other.low
	for v := range other.high {
		s.insert(v)
	}
}

func (s patternVarSet) count() int {
	return bits.OnesCount64(s.low) + len(s.high)
}

func (s patternVarSet) subset(other patternVarSet) bool {
	if s.low&^other.low != 0 || len(s.high) > len(other.high) {
		return false
	}
	for v := range s.high {
		if _, ok := other.high[v]; !ok {
			return false
		}
	}
	return true
}

func sameFreeVars(a, b patternVarSet) bool {
	return a.low == b.low && len(a.high) == len(b.high) && a.subset(b)
}
