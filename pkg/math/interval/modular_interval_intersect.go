package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:89-136 (interval::intersect).

func (v *ModularInterval) Intersect(other, result *ModularInterval) (bool, error) {
	if v == nil || other == nil || result == nil || v.size != other.size {
		return false, ErrExtendedDomain
	}
	result.size = v.size
	return intersectModular(v, other, result), nil
}

func copyModularInterval(dst, src *ModularInterval) {
	dst.size = src.size
	dst.set(src.lower, src.upper, src.tight)
}
