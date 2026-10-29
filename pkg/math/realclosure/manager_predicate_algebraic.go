package realclosure

// Source: src/math/realclosure/realclosure.cpp:2499-2503, 6282-6284.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) queryValueIsAlgebraic(value *realValue) (bool, error) {
	if value == nil {
		return false, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 {
		return false, arena.ErrInvalidSlot
	}
	return !value.isRational && value.root != nil && !value.root.deleted, nil
}

func (s *realValueStore) queryValueIsInfinitesimal(value *realValue) (bool, error) {
	if _, err := s.queryValueIsRational(value); err != nil {
		return false, err
	}
	return false, nil
}

func (s *realValueStore) queryValueIsTranscendental(value *realValue) (bool, error) {
	if _, err := s.queryValueIsRational(value); err != nil {
		return false, err
	}
	return false, nil
}
