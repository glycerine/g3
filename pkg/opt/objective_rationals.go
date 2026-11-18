package opt

// Source: src/opt/opt_context.h objective (vector<rational> m_weights,
// adjust_value m_adjust_value): the decomposed and decoded objectives own
// their rationals, which die with them.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// cloneRationals returns owned copies of source.
func cloneRationals(q *numeral.RationalManager, source []numeral.Rational) ([]numeral.Rational, error) {
	if source == nil {
		return nil, nil
	}
	result := make([]numeral.Rational, len(source))
	for i := range result {
		result[i] = numeral.NewRational()
	}
	for i := range source {
		if err := q.Copy(&result[i], &source[i]); err != nil {
			return nil, errors.Join(err, releaseRationals(q, result))
		}
	}
	return result, nil
}

// cloneRational returns an owned copy of source.
func cloneRational(q *numeral.RationalManager, source *numeral.Rational) (numeral.Rational, error) {
	result := numeral.NewRational()
	return result, q.Copy(&result, source)
}

func releaseRationals(q *numeral.RationalManager, rs []numeral.Rational) (err error) {
	for i := len(rs) - 1; i >= 0; i-- {
		err = errors.Join(err, q.Del(&rs[i]))
	}
	return err
}

// ReleaseRationals releases the objective's weights and offset.
func (o *DecomposedObjective) ReleaseRationals(q *numeral.RationalManager) error {
	err := errors.Join(releaseRationals(q, o.Weights), q.Del(&o.Offset))
	o.Weights, o.Offset = nil, numeral.NewRational()
	return err
}

// ReleaseRationals releases the objective's weights and offset.
func (o *DecodedFormulaObjective) ReleaseRationals(q *numeral.RationalManager) error {
	err := errors.Join(releaseRationals(q, o.Weights), q.Del(&o.Offset))
	o.Weights, o.Offset = nil, numeral.NewRational()
	return err
}

// ReleaseRationals releases every objective's rationals.
func (s *DecodedFormulaSet) ReleaseRationals(q *numeral.RationalManager) (err error) {
	for i := range s.Objectives {
		err = errors.Join(err, s.Objectives[i].ReleaseRationals(q))
	}
	return err
}

func (p *pbSum) release(q *numeral.RationalManager) error {
	err := errors.Join(releaseRationals(q, p.weights), q.Del(&p.offset))
	p.weights, p.offset = nil, numeral.NewRational()
	return err
}

// ReleaseDecomposed releases the rationals of every objective.
func ReleaseDecomposed(q *numeral.RationalManager, objectives []DecomposedObjective) (err error) {
	for i := range objectives {
		err = errors.Join(err, objectives[i].ReleaseRationals(q))
	}
	return err
}
