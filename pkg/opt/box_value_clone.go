package opt

// Source: src/opt/opt_context.cpp retained box bounds and assignments.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// cloneObjectiveValue deep-copies source: its soft rationals are fresh
// rationals of q, owned by the copy.
func cloneObjectiveValue(q *numeral.RationalManager, source *ObjectiveValue) (ObjectiveValue, error) {
	result := *source
	result.Arithmetic.Finite = source.Arithmetic.Finite.Clone()
	result.ArithmeticModel.Finite = source.ArithmeticModel.Finite.Clone()
	result.SoftAssignment = append([]bool(nil), source.SoftAssignment...)
	result.initRationals()
	dst, src := result.rationals(), source.rationals()
	var err error
	for i := range dst {
		if err = q.Copy(dst[i], src[i]); err != nil {
			return result, errors.Join(err, result.ReleaseRationals(q))
		}
	}
	return result, nil
}

// assignObjectiveValue releases dst's rationals and replaces it with a
// deep copy of source.
func assignObjectiveValue(q *numeral.RationalManager, dst, source *ObjectiveValue) error {
	if err := dst.ReleaseRationals(q); err != nil {
		return err
	}
	value, err := cloneObjectiveValue(q, source)
	*dst = value
	return err
}

// CloneObjectiveValues deep-copies values; the copies' rationals are
// owned by the caller (ReleaseObjectiveValues).
func CloneObjectiveValues(q *numeral.RationalManager, source []ObjectiveValue) ([]ObjectiveValue, error) {
	if source == nil {
		return nil, nil
	}
	result := make([]ObjectiveValue, len(source))
	for i := range source {
		value, err := cloneObjectiveValue(q, &source[i])
		if err != nil {
			return nil, errors.Join(err, ReleaseObjectiveValues(q, result[:i]))
		}
		value.DisplayLower = source[i].DisplayLower.Clone()
		value.DisplayUpper = source[i].DisplayUpper.Clone()
		result[i] = value
	}
	return result, nil
}

// ReleaseObjectiveValues releases the values' rationals.
func ReleaseObjectiveValues(q *numeral.RationalManager, values []ObjectiveValue) (err error) {
	for i := range values {
		err = errors.Join(err, values[i].ReleaseRationals(q))
	}
	return err
}
