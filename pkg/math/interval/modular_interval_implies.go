package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:67-87 (interval::implies).

func (v *ModularInterval) Implies(other *ModularInterval) (bool, error) {
	if v == nil || other == nil || v.size != other.size {
		return false, ErrExtendedDomain
	}
	if other.IsFull() {
		return true, nil
	}
	if v.IsFull() {
		return false, nil
	}
	return impliesModular(v, other), nil
}

func impliesModular(a, b *ModularInterval) bool {
	if a.IsWrapped() {
		return b.IsWrapped() && a.upper <= b.upper && a.lower >= b.lower
	}
	if b.IsWrapped() {
		return a.upper <= b.upper || a.lower >= b.lower
	}
	return a.lower >= b.lower && a.upper <= b.upper
}
