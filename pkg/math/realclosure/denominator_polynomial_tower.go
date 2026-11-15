package realclosure

// Source: src/math/realclosure/realclosure.cpp:3194-3301
// (clean_denominators_core(p_sz, p, norm_p, d)).

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// cleanTowerPolynomialDenominators returns owned norm_p and d with
// p = norm_p/d. The integer denominators are combined by their lcm; the
// non-rational denominators are multiplied in, skipping structurally equal
// duplicates, as the source does to control coefficient growth.
func (s *realValueStore) cleanTowerPolynomialDenominators(p []*realValue, usePrem bool) (normalized []*realValue, denominator *realValue, err error) {
	if len(p) == 0 {
		return nil, nil, arena.ErrInvalidSlot
	}
	nums := make([]*realValue, len(p))
	dens := make([]*realValue, len(p))
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(nums), s.releaseSturmCoefficients(dens))
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(normalized), s.releaseSturmScratch(denominator))
			normalized, denominator = nil, nil
		}
	}()
	allOne := true
	for i, coefficient := range p {
		if coefficient == nil {
			continue
		}
		nums[i], dens[i], err = s.cleanTowerValueDenominator(coefficient, usePrem)
		if err != nil {
			return nil, nil, err
		}
		one, e := s.isRationalOneValue(dens[i])
		if e != nil {
			return nil, nil, e
		}
		if !one {
			allOne = false
		}
	}
	if allOne {
		normalized, nums = nums, nil
		denominator, err = s.newRational(1, 1)
		return normalized, denominator, err
	}
	// Compute lcm of the integer elements in dens.
	lcmZ := numeral.NewRational()
	defer func() {
		if lcmZ.Num.Val != 0 {
			err = errors.Join(err, s.releaseSturmRational(&lcmZ))
		}
	}()
	foundZ := false
	for _, d := range dens {
		if d == nil || !d.isRational {
			continue
		}
		integer, e := s.numbers.IsInt(&d.rational)
		if e != nil || !integer {
			return nil, nil, errors.Join(e, arena.ErrInvalidSlot)
		}
		if !foundZ {
			foundZ = true
			if err = s.numbers.Copy(&lcmZ, &d.rational); err != nil {
				return nil, nil, err
			}
			continue
		}
		next := numeral.NewRational()
		if err = s.numbers.LCM(&lcmZ, &d.rational, &next); err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmRational(&next))
		}
		if err = s.releaseSturmRational(&lcmZ); err != nil {
			lcmZ = numeral.NewRational()
			return nil, nil, errors.Join(err, s.releaseSturmRational(&next))
		}
		lcmZ = next
	}
	var lcm *realValue
	if foundZ {
		lcm, err = s.newRationalCopy(&lcmZ)
	} else {
		lcm, err = s.newRational(1, 1)
	}
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(lcm)) }()
	// Compute the multipliers for nums, norm_p and d. Structurally equal
	// non-rational denominators are detected instead of using a GCD.
	if err = s.retain(lcm); err != nil {
		return nil, nil, err
	}
	denominator = lcm
	normalized = make([]*realValue, len(p))
	for i := range p {
		if nums[i] == nil {
			continue
		}
		var m *realValue
		isZ := dens[i].isRational
		if !isZ {
			if err = s.retain(lcm); err != nil {
				return nil, nil, err
			}
			m = lcm
		} else {
			quotient := numeral.NewRational()
			if err = s.numbers.Div(&lcmZ, &dens[i].rational, &quotient); err != nil {
				return nil, nil, errors.Join(err, s.releaseSturmRational(&quotient))
			}
			if m, err = s.ownNonzeroRational(&quotient); err != nil {
				return nil, nil, err
			}
		}
		foundLtEq := false
		for j := range p {
			if dens[j] == nil {
				continue
			}
			if i != j && !dens[j].isRational {
				equal, e := s.structurallyEqualValues(dens[i], dens[j])
				if e != nil {
					return nil, nil, errors.Join(e, s.releaseSturmScratch(m))
				}
				if equal {
					if j < i {
						foundLtEq = true
					}
				} else if m, err = s.replaceRankedProduct(m, dens[j], usePrem); err != nil {
					return nil, nil, errors.Join(err, s.releaseSturmScratch(m))
				}
			}
		}
		if !isZ && !foundLtEq {
			product, e := s.multiplyRankedRetainedValues(dens[i], denominator, usePrem)
			if e != nil {
				return nil, nil, errors.Join(e, s.releaseSturmScratch(m))
			}
			if e = s.releaseSturmScratch(denominator); e != nil {
				denominator = product
				return nil, nil, errors.Join(e, s.releaseSturmScratch(m))
			}
			denominator = product
		}
		if m, err = s.replaceRankedProduct(m, nums[i], usePrem); err != nil {
			return nil, nil, errors.Join(err, s.releaseSturmScratch(m))
		}
		normalized[i] = m
	}
	return normalized, denominator, nil
}

// cleanTowerPolynomialDenominatorsIfNeeded is clean_denominators(sz, p,
// norm_p, d): a polynomial whose denominators are already clean is returned
// unchanged with d = 1.
func (s *realValueStore) cleanTowerPolynomialDenominatorsIfNeeded(p []*realValue, usePrem bool) (normalized []*realValue, denominator *realValue, err error) {
	if len(p) == 0 {
		return nil, nil, arena.ErrInvalidSlot
	}
	clean, err := s.hasCleanTowerPolynomialDenominators(p)
	if err != nil {
		return nil, nil, err
	}
	if !clean {
		return s.cleanTowerPolynomialDenominators(p, usePrem)
	}
	if normalized, err = s.retainRationalPolynomial(p); err != nil {
		return nil, nil, err
	}
	if denominator, err = s.newRational(1, 1); err != nil {
		return nil, nil, errors.Join(err, s.releaseSturmCoefficients(normalized))
	}
	return normalized, denominator, nil
}

func (s *realValueStore) isRationalOneValue(value *realValue) (bool, error) {
	if value == nil || !value.isRational {
		return false, nil
	}
	return s.numbers.IsOne(&value.rational)
}
