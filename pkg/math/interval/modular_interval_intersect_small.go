package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:89-136 (interval::intersect).

func intersectModular(a, b, r *ModularInterval) bool {
	equal, _ := a.Equal(b)
	if a.IsFull() || equal {
		copyModularInterval(r, b)
		return true
	}
	if b.IsFull() {
		copyModularInterval(r, a)
		return true
	}
	if a.IsWrapped() {
		if b.IsWrapped() {
			if a.upper >= b.lower {
				copyModularInterval(r, b)
			} else if b.upper >= a.lower {
				copyModularInterval(r, a)
			} else {
				r.set(maxU64(a.lower, b.lower), minU64(a.upper, b.upper), false)
			}
			return true
		}
		return intersectModular(b, a, r)
	}
	if b.IsWrapped() {
		if a.upper < b.lower && a.lower > b.upper {
			return false
		}
		if a.upper >= b.lower && a.lower <= b.upper {
			copyModularInterval(r, b)
		} else if a.upper >= b.lower {
			r.set(b.lower, a.upper, false)
		} else {
			r.set(a.lower, minU64(a.upper, b.upper), false)
		}
		return true
	}
	if a.lower > b.upper || a.upper < b.lower {
		return false
	}
	r.set(maxU64(a.lower, b.lower), minU64(a.upper, b.upper), a.tight && b.tight)
	return true
}
func minU64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
