package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:43-60 (the interval
// constructor's SASSERTs and is_full/is_wrapped).

func (v *ModularInterval) Invariant() bool {
	if v == nil || v.size == 0 || v.size > 64 {
		return false
	}
	bound := modularBound(v.size)
	return v.lower <= bound && v.upper <= bound &&
		(v.lower <= v.upper || v.lower != v.upper+1)
}

func (v *ModularInterval) IsFull() bool {
	if v == nil {
		return false
	}
	return v.lower == 0 && v.upper == modularBound(v.size)
}
func (v *ModularInterval) IsWrapped() bool {
	if v == nil {
		return false
	}
	return v.lower > v.upper
}
func (v *ModularInterval) IsSingleton() bool {
	if v == nil {
		return false
	}
	return v.lower == v.upper
}
