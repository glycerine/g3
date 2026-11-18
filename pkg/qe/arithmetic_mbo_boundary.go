package qe

// Temporary boundary to math/simplex's model_based_opt, whose API still takes
// *big.Rat. qe's own rationals (linearize's ts, c and mul) are rationals of
// the manager's NumeralService().Manager; they are converted only where they
// cross into model_based_opt. The math/simplex rational patch removes this
// file when model_based_opt takes rationals.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/math/simplex"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func mboRat(q *unumeral.RationalManager, r *unumeral.Rational) (*big.Rat, error) {
	out := new(big.Rat)
	if err := q.BigRatOf(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func mboAddVariable(mbo *simplex.ModelBasedOpt, q *unumeral.RationalManager, value *unumeral.Rational, integer bool) (uint32, error) {
	v, err := mboRat(q, value)
	if err != nil {
		return 0, err
	}
	return mbo.AddVariable(v, integer), nil
}

func mboLinearTerm(q *unumeral.RationalManager, id uint32, coefficient *unumeral.Rational) (simplex.LinearTerm, error) {
	v, err := mboRat(q, coefficient)
	if err != nil {
		return simplex.LinearTerm{}, err
	}
	return simplex.NewLinearTerm(id, v), nil
}

func mboAddConstraint(mbo *simplex.ModelBasedOpt, coeffs []simplex.LinearTerm, q *unumeral.RationalManager, constant *unumeral.Rational, relation simplex.Relation) error {
	c, err := mboRat(q, constant)
	if err != nil {
		return err
	}
	return mbo.AddConstraint(coeffs, c, relation)
}

func mboAddDivides(mbo *simplex.ModelBasedOpt, coeffs []simplex.LinearTerm, q *unumeral.RationalManager, constant, modulus *unumeral.Rational) error {
	c, err := mboRat(q, constant)
	if err != nil {
		return err
	}
	m, err := mboRat(q, modulus)
	if err != nil {
		return err
	}
	return mbo.AddDivides(coeffs, c, m)
}

func mboSetObjective(mbo *simplex.ModelBasedOpt, coeffs []simplex.LinearTerm, q *unumeral.RationalManager, constant *unumeral.Rational) error {
	c, err := mboRat(q, constant)
	if err != nil {
		return err
	}
	return mbo.SetObjective(coeffs, c)
}
