package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:138-156 (interval::negate).

func (v *ModularInterval) Negate(result *ModularInterval) (bool, error) {
	if v == nil || result == nil {
		return false, ErrExtendedDomain
	}
	result.size = v.size
	if !v.tight {
		result.set(0, modularBound(v.size), true)
		return true, nil
	}
	if v.IsFull() {
		return false, nil
	}
	if v.lower == 0 {
		result.set(v.upper+1, modularBound(v.size), false)
	} else if v.upper == modularBound(v.size) {
		result.set(0, v.lower-1, false)
	} else {
		result.set(v.upper+1, v.lower-1, false)
	}
	return true, nil
}
