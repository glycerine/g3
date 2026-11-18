package api

// Source: src/api/api_rcf.cpp:293-306.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *SMTContext) RCFNumeratorDenominator(value *RCFNumber) (numerator, denominator *RCFNumber, resultErr error) {
	if _, err := c.checkRCF(value); err != nil {
		return nil, nil, err
	}
	if value.real != nil {
		cleanNumerator, cleanDenominator, err := c.realClosure.CleanDenominators(value.real)
		if err != nil {
			return nil, nil, err
		}
		numerator, err = c.wrapRealRCF(value.service, cleanNumerator, nil)
		if err != nil {
			return nil, nil, errors.Join(err, cleanDenominator.Close())
		}
		denominator, err = c.wrapRealRCF(value.service, cleanDenominator, nil)
		if err != nil {
			return nil, nil, errors.Join(err, numerator.Close())
		}
		return numerator, denominator, nil
	}
	text, err := c.RCFString(value)
	if err != nil {
		return nil, nil, err
	}
	m, err := c.Manager()
	if err != nil {
		return nil, nil, err
	}
	// numerator(q) and denominator(q) of the rational value.
	var numeratorText, denominatorText string
	err = withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		if q.SetString(&rs[0], text) != nil {
			return ErrRCFNumberOwner
		}
		e := errors.Join(q.DenominatorRational(&rs[0], &rs[1]), q.NumeratorRational(&rs[0], &rs[0]))
		if e == nil {
			numeratorText, e = q.String(&rs[0])
		}
		if e == nil {
			denominatorText, e = q.String(&rs[1])
		}
		return e
	})
	if err != nil {
		return nil, nil, err
	}
	numerator, err = c.RCFRational(numeratorText)
	if err != nil {
		return nil, nil, err
	}
	denominator, err = c.RCFRational(denominatorText)
	if err != nil {
		_ = numerator.Close()
		return nil, nil, err
	}
	return numerator, denominator, nil
}
