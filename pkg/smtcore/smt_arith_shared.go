package smtcore

// Arithmetic helpers shared by the arithmetic, LRA, difference-logic,
// UTVPI, sequence and string theories. They were defined next to the
// replay arithmetic theory, which SMTTheoryArith replaced.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// SMTArithmeticObjectiveValue is the concrete result of the arithmetic
// maximizer: an unbounded sentinel, or an exact tableau point. For inf_ext,
// Finite also retains the point's infinity coefficient; it must not be reduced
// to Unbounded. Finite's rationals belong to the AST manager's rational
// manager and to the value's holder, which frees them (Del).
type SMTArithmeticObjectiveValue struct {
	Unbounded bool
	Finite    simplex.Infinitesimal
}

// Del frees the value's rationals.
func (v *SMTArithmeticObjectiveValue) Del(q *numeral.RationalManager) error {
	return v.Finite.Del(q)
}

const (
	smtArithOpIDiv int32 = 11
	smtArithOpRem  int32 = 12
	smtArithOpMod  int32 = 13
)

// parameters returns owning copies of the source rule symbol followed by
// literal coefficients and then equality coefficients. Callers release them
// after their justification has taken its own copies.
func (a *smtArithmeticAntecedents) parameters(m *ast.ASTManager, rule string) (result []ast.ASTParameter, err error) {
	if a.numParameters() == 0 {
		return nil, nil
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, smtArithmeticReleaseParameters(m, result))
			result = nil
		}
	}()
	symbol, err := m.Symbols.Intern(rule)
	if err != nil {
		return nil, err
	}
	p, err := m.NewParameterSymbol(symbol)
	if err != nil {
		return nil, err
	}
	result = append(result, p)
	for _, coefficients := range [][]numeral.Rational{a.literalCoefficients, a.equalityCoefficients} {
		for i := range coefficients {
			p, err = m.NewParameterFromRational(&coefficients[i])
			if err != nil {
				return result, err
			}
			result = append(result, p)
		}
	}
	return result, nil
}

func smtArithmeticReleaseParameters(m *ast.ASTManager, parameters []ast.ASTParameter) (err error) {
	for i := range parameters {
		err = errors.Join(err, m.DelParameter(&parameters[i]))
	}
	return
}

// Source context::is_shared common cases. consultTheory is true only for a
// single-theory class whose owner-specific sharing rule remains to be checked.
func (s *SMTInternalizerState) sharedTheoryNode(node *SMTEnode) (shared, consultTheory bool, result error) {
	if err := s.open(); err != nil {
		return false, false, err
	}
	if node == nil || node.manager != s.manager || node.root == nil {
		return false, false, ast.ErrASTDomain
	}
	root := node.root
	family := root.family
	if family == s.manager.BasicFamilyID() {
		if root.declKind == ast.ASTOpIte {
			return true, false, nil
		}
	}
	count := root.NumTheoryVars()
	if count == 0 {
		return false, false, nil
	}
	if count > 1 {
		return true, false, nil
	}
	if q := s.quantifierManager; q != nil && q.IsShared(root) {
		return true, false, nil
	}
	theory := root.theoryVariables.theory
	for _, parent := range root.sourceParents {
		family := parent.family
		if family != theory && family != s.manager.BasicFamilyID() {
			return true, false, nil
		}
	}
	return false, true, nil
}

func smtSharedArithmeticNode(owner *SMTTheoryBase, node *SMTEnode, division bool) (bool, error) {
	s, err := owner.State()
	if err != nil {
		return false, err
	}
	shared, consult, err := s.sharedTheoryNode(node)
	if err != nil || !consult || !division {
		return shared, err
	}
	for _, parent := range node.root.sourceParents {
		if parent.family == owner.FamilyID() {
			if kind := parent.declKind; kind == smtArithOpDiv || kind == smtArithOpIDiv || kind == smtArithOpRem || kind == smtArithOpMod {
				return true, nil
			}
		}
	}
	return false, nil
}

type smtArithmeticEqualityCandidate struct {
	left, right SMTTheoryVar
}

// SMTInfEpsString is inf_eps_rational<inf_rational>::to_string for a value
// of q.
func SMTInfEpsString(q *numeral.RationalManager, value *simplex.Infinitesimal) (string, error) {
	return smtArithmeticBoundName(q, value)
}

// Source inf_eps_rational<inf_rational>::to_string and inf_rational::to_string.
func smtArithmeticBoundName(q *numeral.RationalManager, value *simplex.Infinitesimal) (string, error) {
	finite, err := q.String(&value.Main)
	if err != nil {
		return "", err
	}
	epsilon, err := q.Sign(&value.Epsilon)
	if err != nil {
		return "", err
	}
	if epsilon != 0 {
		sign := " +e*"
		if epsilon < 0 {
			sign = " -e*"
		}
		// abs(m_infinitesimal) is a temporary.
		a := smtQZero()
		err = q.Copy(&a, &value.Epsilon)
		if err == nil {
			err = q.Abs(&a)
		}
		var text string
		if err == nil {
			text, err = q.String(&a)
		}
		if err = smtErrJoin(err, smtQDel(q, &a)); err != nil {
			return "", err
		}
		finite = "(" + finite + sign + text + ")"
	}
	infinite, err := q.Sign(&value.Infinity)
	if err != nil || infinite == 0 {
		return finite, err
	}
	var infinity string
	if one, err := q.IsOne(&value.Infinity); err != nil {
		return "", err
	} else if one {
		infinity = "oo"
	} else if minusOne, err := q.IsMinusOne(&value.Infinity); err != nil {
		return "", err
	} else if minusOne {
		infinity = "-oo"
	} else {
		text, err := q.String(&value.Infinity)
		if err != nil {
			return "", err
		}
		infinity = text + "*oo"
	}
	main, err := q.Sign(&value.Main)
	if err != nil {
		return "", err
	}
	if main == 0 && epsilon == 0 {
		return infinity, nil
	}
	return "(" + infinity + " + " + finite + ")", nil
}
