package numeral

// Source: src/util/hwf.cpp:387-393,409-427;
// src/util/f2n.h:164 instantiated as f2n<hwf_manager>.

import "github.com/glycerine/g3/pkg/arena"

// ToRationalString is the source's to_rational_string: a temporary
// unsynchronized mpq manager receives to_rational and formats the reduced
// value.
func (m *HardwareFloatManager) ToRationalString(x *HardwareFloat) (text string, err error) {
	if m == nil || x == nil {
		return "", ErrNumericOwner
	}
	if modularSourceDebug && !(m.IsNormal(x) || m.IsDenormal(x) || m.IsZero(x)) {
		return "", ErrHardwareFloatNonRegular
	}
	budget := arena.NewBudget(^uint64(0))
	qm, err := NewRationalManager(RationalConfig{Integers: IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: budget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}}}})
	if err != nil {
		return "", err
	}
	defer qm.Close()
	q := NewRational()
	defer func() {
		if e := qm.Del(&q); err == nil {
			err = e
		}
	}()
	if err = m.ToRational(x, qm, &q); err != nil {
		return "", err
	}
	return qm.String(&q)
}

func (m *HardwareFloatNumeralManager) ToRationalString(x *HardwareFloat) (string, error) {
	if m == nil || m.core == nil {
		return "", ErrNumericOwner
	}
	return m.core.ToRationalString(x)
}
