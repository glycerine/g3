package realclosure

// Source: src/math/realclosure/realclosure.cpp:4313-4344.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) refineAnyRetainedInterval(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (bool, error) {
	if value == nil {
		return true, nil
	}
	if value.store != s || value.deleted || value.refs == 0 || bounds == nil {
		return false, arena.ErrInvalidSlot
	}
	if value.isRational {
		interval, err := s.cachedRationalApproxInterval(value, bounds)
		if err != nil {
			return false, err
		}
		return true, s.refineSturmRationalInterval(value, interval, precision)
	}
	if value.root != nil && value.root.kind == realExtensionTranscendental {
		return s.refineTranscendentalPolynomialValue(value, precision)
	}
	if value.root != nil && value.root.kind == realExtensionInfinitesimal {
		return false, nil
	}
	if value.root.owner.intervals != bounds {
		return false, arena.ErrInvalidSlot
	}
	return s.refineAlgebraicPolynomialValue(value, precision)
}
