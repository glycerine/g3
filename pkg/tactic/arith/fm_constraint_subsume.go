package arith

// Source: src/tactic/arith/fm_tactic.cpp:626-718.
func FMConstraintSubsumes(a, b *FMConstraint) (bool, error) {
	return fmConstraintSubsumes(a, b, nil)
}

func fmConstraintSubsumes(a, b *FMConstraint, counter *uint32) (bool, error) {
	if a == nil || b == nil || a == b || len(a.Literals) > len(b.Literals) || len(a.Variables) != len(b.Variables) {
		return false, nil
	}
	q := a.numerals
	cmp, err := q.Compare(&a.Constant, &b.Constant)
	if err != nil || cmp > 0 {
		return false, err
	}
	if !a.Strict && b.Strict && cmp == 0 {
		return false, nil
	}
	if counter != nil {
		*counter += uint32(len(a.Literals) + len(b.Literals))
	}
	pos := make(map[uint32]int, len(a.Variables))
	for i, v := range a.Variables {
		pos[v] = i
	}
	for i, v := range b.Variables {
		j, ok := pos[v]
		if !ok {
			return false, nil
		}
		if equal, err := q.Equal(&a.Coefficients[j], &b.Coefficients[i]); err != nil || !equal {
			return false, err
		}
	}
	signs := make(map[uint32]int8, len(b.Literals))
	for _, l := range b.Literals {
		v := uint32(l)
		s := int8(1)
		if l < 0 {
			v = uint32(-l)
			s = -1
		}
		signs[v] = s
	}
	for _, l := range a.Literals {
		v := uint32(l)
		s := int8(1)
		if l < 0 {
			v = uint32(-l)
			s = -1
		}
		if signs[v] != s {
			return false, nil
		}
	}
	return true, nil
}
