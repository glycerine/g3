package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:138-156 (interval::negate).

func (v *ModularInterval) Negate(result *ModularInterval) (bool, error) {
	if v == nil || result == nil {
		return false, ErrExtendedDomain
	}
	result.size = v.size
	if !v.tight {
		// The source writes interval(0, uMaxInt(sz), true), which binds true
		// to the sz parameter: size 1, tight false. g3 keeps the evident
		// intent (the full interval of v's size, tight). Unreachable: the
		// only producer, is_bound, makes tight intervals, so no input can
		// show the difference (AUDIT.md math_misc_02-F5).
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
