package realclosure

// Source: src/math/realclosure/realclosure.cpp:4313-4344 and manager interval
// query wrappers. This is the retained rational and single-root algebraic
// dispatch; transcendental and infinitesimal extension kinds are not live.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *realValueStore) valueInterval(value *realValue, bounds *numeral.BinaryRationalManager) (*BinaryInterval, error) {
	if s == nil || bounds == nil || value == nil || value.store != s || value.deleted || value.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.cachedRationalApproxInterval(value, bounds)
	}
	if value.functionInterval == nil || value.functionInterval.numbers != bounds {
		return nil, arena.ErrInvalidSlot
	}
	return value.functionInterval, nil
}

func (s *realValueStore) refineValueInterval(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (bool, error) {
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return false, err
	}
	magnitude, err := s.sturmIntervalMagnitude(interval)
	if err != nil {
		return false, err
	}
	if magnitude == math.MinInt32 || magnitude < 0 && uint32(-magnitude) > precision {
		return true, nil
	}
	if value.isRational {
		return true, s.refineSturmRationalInterval(value, interval, precision)
	}
	return s.refineSturmAlgebraicInterval(value, precision)
}

func (s *realValueStore) cloneValueInterval(value *realValue, bounds *numeral.BinaryRationalManager, precision uint32) (result *BinaryInterval, err error) {
	if value == nil {
		result, err = NewBinaryInterval(bounds)
		if err != nil {
			return nil, err
		}
		zero := numeral.NewBinaryRational()
		defer func() { err = errors.Join(err, bounds.Del(&zero)) }()
		if err = bounds.SetInt32(&zero, 0); err != nil {
			return nil, err
		}
		err = result.SetFinite(&zero, &zero, false, false)
		return result, err
	}
	if _, err = s.refineValueInterval(value, bounds, precision); err != nil {
		return nil, err
	}
	source, err := s.valueInterval(value, bounds)
	if err != nil {
		return nil, err
	}
	result, err = NewBinaryInterval(bounds)
	if err != nil {
		return nil, err
	}
	if err = result.SetFrom(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
