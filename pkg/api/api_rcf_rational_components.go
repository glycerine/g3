package api

// Source: src/api/api_rcf.cpp:293-306.

import "math/big"

func (c *SMTContext) RCFNumeratorDenominator(value *RCFNumber) (numerator, denominator *RCFNumber, resultErr error) {
	text, err := c.RCFString(value)
	if err != nil {
		return nil, nil, err
	}
	rational, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, nil, ErrRCFNumberOwner
	}
	numerator, err = c.RCFRational(rational.Num().String())
	if err != nil {
		return nil, nil, err
	}
	denominator, err = c.RCFRational(rational.Denom().String())
	if err != nil {
		_ = numerator.Close()
		return nil, nil, err
	}
	return numerator, denominator, nil
}
