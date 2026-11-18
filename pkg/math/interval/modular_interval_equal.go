package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:61-65 (interval::operator==).

func (v *ModularInterval) Equal(other *ModularInterval) (bool, error) {
	if v == nil || other == nil || v.size != other.size {
		return false, ErrExtendedDomain
	}
	return v.lower == other.lower && v.upper == other.upper && v.tight == other.tight, nil
}
