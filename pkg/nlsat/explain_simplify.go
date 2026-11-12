package nlsat

// Source: src/nlsat/nlsat_explain.cpp:998-1298.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type explanationEqualityInfo struct {
	equality                  polynomial.Polynomial
	variable, degree          uint32
	leading                   polynomial.Polynomial
	leadingSign               int
	leadingConstant           bool
	addLeading, addInequality bool
}

func (n *explanationAssumptions) selectCoreEquality(core *scopedLiteralVector, max uint32) (polynomial.Polynomial, error) {
	var selected polynomial.Polynomial
	minimum := uint32(math.MaxUint32)
	for _, literal := range core.result {
		if literal.Sign() {
			continue
		}
		atom, err := n.search.store.literalAtom(literal)
		if err != nil {
			return polynomial.Polynomial{}, err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok || inequality.kind != AtomEQ || len(inequality.factors) != 1 || inequality.factors[0].Even {
			continue
		}
		degree, err := n.search.store.atoms.ineq.core.Degree(inequality.factors[0].Polynomial, max)
		if err != nil {
			return polynomial.Polynomial{}, err
		}
		if degree < minimum {
			selected, minimum = inequality.factors[0].Polynomial, degree
			if degree == 1 {
				break
			}
		}
	}
	return selected, nil
}

// holdLiteral is scoped_literal::operator=: it retains l before releasing
// the literal previously held, which the release may delete.
func (n *explanationAssumptions) holdLiteral(held *sat.Literal, l sat.Literal) error {
	if err := n.search.store.atoms.incRef(l.Variable()); err != nil {
		return err
	}
	old := *held
	*held = l
	return n.search.store.atoms.decRef(old.Variable())
}

// rewriteEqualityLiteral is simplify(literal l, eq_info&, var max,
// scoped_literal& new_lit) (nlsat_explain.cpp:1069-1182); held is new_lit.
func (n *explanationAssumptions) rewriteEqualityLiteral(literal sat.Literal, info *explanationEqualityInfo, max uint32, held *sat.Literal) (modified bool, err error) {
	result, modified, err := n.rewriteEqualityLiteralCore(literal, info, max, held)
	if err != nil {
		return modified, err
	}
	if result != *held {
		err = n.holdLiteral(held, result)
	}
	return modified, err
}

func (n *explanationAssumptions) rewriteEqualityLiteralCore(literal sat.Literal, info *explanationEqualityInfo, max uint32, held *sat.Literal) (result sat.Literal, modified bool, err error) {
	atom, err := n.search.store.literalAtom(literal)
	if err != nil {
		return result, false, err
	}
	inequality, ok := atom.(*InequalityAtom)
	if !ok {
		return literal, false, nil
	}
	if len(inequality.factors) == 1 {
		equal, equalErr := n.search.store.atoms.ineq.core.Equal(inequality.factors[0].Polynomial, info.equality)
		if equalErr != nil {
			return result, false, equalErr
		}
		if equal {
			return literal, false, nil
		}
	}
	core := n.search.store.atoms.ineq.core
	factors, err := polynomial.NewPolynomialRefVector(core)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, factors.Close()) }()
	remainder, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, remainder.Close()) }()
	even := make([]bool, 0, len(inequality.factors))
	atomSign := 1
	for _, factor := range inequality.factors {
		degree, degreeErr := core.Degree(factor.Polynomial, info.variable)
		if degreeErr != nil {
			return result, false, degreeErr
		}
		if degree < info.degree {
			if err = factors.Push(factor.Polynomial); err != nil {
				return result, false, err
			}
			even = append(even, factor.Even)
			continue
		}
		modified = true
		var d uint32
		if err = core.PseudoRemainder(factor.Polynomial, info.equality, info.variable, &d, remainder); err != nil {
			return result, false, err
		}
		value, valueErr := remainder.Get()
		if valueErr != nil {
			return result, false, valueErr
		}
		if d%2 == 1 && !factor.Even && info.leadingSign < 0 {
			atomSign = -atomSign
		}
		constant, constantErr := core.IsConstant(value)
		if constantErr != nil {
			return result, false, constantErr
		}
		if constant {
			sign, signErr := n.sign(value)
			if signErr != nil {
				return result, false, signErr
			}
			if sign == 0 {
				atomValue := inequality.kind == AtomEQ
				if !info.leadingConstant {
					info.addLeading = true
				}
				return explanationTruthLiteral(atomValue != literal.Sign()), true, nil
			}
			if !info.leadingConstant {
				info.addLeading = true
				if d%2 == 1 && !factor.Even && inequality.kind != AtomEQ {
					info.addInequality = true
				}
			}
			if sign < 0 && !factor.Even {
				atomSign = -atomSign
			}
			continue
		}
		if err = factors.Push(value); err != nil {
			return result, false, err
		}
		even = append(even, factor.Even)
		if !info.leadingConstant {
			info.addLeading = true
			if d%2 == 1 && !factor.Even && inequality.kind != AtomEQ {
				info.addInequality = true
			}
		}
	}
	if !modified {
		return literal, false, nil
	}
	kind := inequality.kind
	if atomSign < 0 {
		kind, err = kind.Flip()
		if err != nil {
			return result, false, err
		}
	}
	count, err := factors.Size()
	if err != nil {
		return result, false, err
	}
	values := make([]polynomial.Polynomial, count)
	for i := range values {
		values[i], err = factors.Get(uint32(i))
		if err != nil {
			return result, false, err
		}
	}
	result, err = n.search.store.atoms.mkInequalityLiteral(kind, values, even)
	if err != nil {
		return result, false, err
	}
	// new_lit = m_solver.mk_ineq_literal(...)
	if err = n.holdLiteral(held, result); err != nil {
		return result, false, err
	}
	if literal.Sign() {
		result = result.Negated()
		*held = result
	}
	if result == TrueLiteral || result == FalseLiteral {
		return result, true, nil
	}
	resultMax, err := n.search.store.maxVar([]sat.Literal{result})
	if err != nil {
		return result, false, err
	}
	if resultMax < max {
		value, valueErr := n.search.valueCore(result)
		if valueErr != nil {
			return result, false, valueErr
		}
		if value == lbool.False {
			if err = n.addLiteral(result); err != nil {
				return result, false, err
			}
		}
		return TrueLiteral, true, nil
	}
	result, err = n.normalizeLiteral(result, max)
	return result, true, err
}

func (n *explanationAssumptions) simplifyCoreWithEquality(core *scopedLiteralVector, equality polynomial.Polynomial, max uint32) (modified bool, err error) {
	polynomials := n.search.store.atoms.ineq.core
	variable, err := polynomials.MaxVariable(equality)
	if err != nil {
		return false, err
	}
	degree, err := polynomials.Degree(equality, variable)
	if err != nil {
		return false, err
	}
	leadingRef, err := polynomial.NewPolynomialRef(polynomials)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, leadingRef.Close()) }()
	leading, err := polynomials.Coeff(equality, variable, degree)
	if err != nil {
		return false, err
	}
	if err = leadingRef.Assign(leading); err != nil {
		return false, err
	}
	leading, err = leadingRef.Get()
	if err != nil {
		return false, err
	}
	leadingSign, err := n.sign(leading)
	if err != nil {
		return false, err
	}
	leadingConstant, err := polynomials.IsConstant(leading)
	if err != nil {
		return false, err
	}
	info := explanationEqualityInfo{equality: equality, variable: variable, degree: degree, leading: leading, leadingSign: leadingSign, leadingConstant: leadingConstant}
	// scoped_literal new_lit(m_solver), released when simplify returns.
	held := sat.DefaultLiteral()
	defer func() { err = errors.Join(err, n.holdLiteral(&held, sat.DefaultLiteral())) }()
	j := uint32(0)
	for i := 0; i < len(core.result); i++ {
		literal := core.result[i]
		// new_lit = null_literal
		if err = n.holdLiteral(&held, sat.DefaultLiteral()); err != nil {
			return false, err
		}
		if _, err = n.rewriteEqualityLiteral(literal, &info, max, &held); err != nil {
			return false, err
		}
		newLiteral := held
		if newLiteral == literal {
			if err = core.set(j, literal); err != nil {
				return false, err
			}
			j++
			continue
		}
		modified = true
		if newLiteral == TrueLiteral {
			continue
		}
		if newLiteral == FalseLiteral {
			j = 0
			break
		}
		if err = core.set(j, newLiteral); err != nil {
			return false, err
		}
		j++
	}
	if err = core.shrink(j); err != nil {
		return false, err
	}
	if info.addLeading {
		if info.addInequality {
			kind := AtomGT
			if info.leadingSign < 0 {
				kind = AtomLT
			}
			err = n.addSimpleAssumption(kind, info.leading, false)
		} else {
			err = n.addSimpleAssumption(AtomEQ, info.leading, true)
		}
	}
	return modified, err
}

func (n *explanationAssumptions) simplifyCoreEqualities(core *scopedLiteralVector, max uint32) error {
	for len(core.result) != 0 {
		equality, err := n.selectCoreEquality(core, max)
		if err != nil {
			return err
		}
		if equality == (polynomial.Polynomial{}) {
			break
		}
		modified, err := n.simplifyCoreWithEquality(core, equality, max)
		if err != nil {
			return err
		}
		if !modified {
			// The source breaks here; the lower-stage equations below
			// still apply.
			break
		}
	}
	for len(core.result) != 0 {
		equality, err := n.selectLowerStageEquality(core, max)
		if err != nil || equality == nil {
			return err
		}
		modified, err := n.simplifyCoreWithEquality(core, equality.factors[0].Polynomial, max)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_explain.cpp", 1333, "simplify(C, eq_p, max)", modified); err != nil {
			return err
		}
		if !modified {
			return ErrSearchPending
		}
		if err = n.addLiteral(sat.MakeLiteral(equality.boolVar, true)); err != nil {
			return err
		}
	}
	return nil
}

// selectLowerStageEquality is select_lower_stage_eq (nlsat_explain.cpp:1271-
// 1308): for each non-root literal of C, each factor p and each variable y of
// p in m_pm.vars first-encounter order with y < max, m_x2eq[y] is the
// candidate.
func (n *explanationAssumptions) selectLowerStageEquality(core *scopedLiteralVector, max uint32) (*InequalityAtom, error) {
	polynomials := n.search.store.atoms.ineq.core
	var xs []uint32
	for _, literal := range core.result {
		atom, err := n.search.store.literalAtom(literal)
		if err != nil {
			return nil, err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok {
			continue
		}
		for _, factor := range inequality.factors {
			xs, err = explanationPolynomialVars(polynomials, factor.Polynomial, xs[:0])
			if err != nil {
				return nil, err
			}
			for _, variable := range xs {
				if variable >= max || uint64(variable) >= uint64(len(n.search.store.vars.equalities)) {
					continue
				}
				candidate, ok := n.search.store.vars.equalities[variable].(*InequalityAtom)
				if !ok || candidate == nil {
					continue
				}
				if err = atomAssert("nlsat_explain.cpp", 1294, "to_ineq_atom(eq)->size() == 1", len(candidate.factors) == 1); err != nil {
					return nil, err
				}
				if len(candidate.factors) != 1 || candidate.factors[0].Even {
					continue
				}
				equality := candidate.factors[0].Polynomial
				degree, err := polynomials.Degree(equality, variable)
				if err != nil {
					return nil, err
				}
				constant, err := polynomials.NonzeroConstantCoefficient(equality, variable, degree)
				if err != nil {
					return nil, err
				}
				if !constant {
					continue
				}
				factorDegree, err := polynomials.Degree(factor.Polynomial, variable)
				if err != nil {
					return nil, err
				}
				if factorDegree >= degree {
					return candidate, nil
				}
			}
		}
	}
	return nil, nil
}

// explanationPolynomialVars is polynomial::manager::vars: the distinct
// variables of p in first-encounter order over its monomials.
func explanationPolynomialVars(core *polynomial.MultivariatePolynomialCore, p polynomial.Polynomial, out []uint32) ([]uint32, error) {
	monomials, err := core.MonomialManager()
	if err != nil {
		return nil, err
	}
	info, err := core.Info(p)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < info.Size; i++ {
		monomial, err := core.TermMonomial(p, i)
		if err != nil {
			return nil, err
		}
		monomialInfo, err := monomials.Info(monomial)
		if err != nil {
			return nil, err
		}
		for j := uint32(0); j < monomialInfo.Size; j++ {
			power, err := monomials.ReadPower(monomial, j)
			if err != nil {
				return nil, err
			}
			seen := false
			for _, x := range out {
				if x == power.Var {
					seen = true
					break
				}
			}
			if !seen {
				out = append(out, power.Var)
			}
		}
	}
	return out, nil
}
